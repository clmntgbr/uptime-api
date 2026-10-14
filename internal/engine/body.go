package engine

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	"go-api/internal/contract"

	"github.com/klauspost/compress/zstd"
)

const (
	// BodyPreviewBytes is the plaintext preview kept in Kafka / Postgres.
	BodyPreviewBytes = 4 * 1024
	// MaxResponseKeys caps JSON paths collected for later search.
	MaxResponseKeys = 256
	// MaxResponseKeyDepth limits how deep we walk a JSON document.
	MaxResponseKeyDepth = 8
)

func (r *Runner) storeBody(ctx context.Context, state *run, call contract.JobCall, resp *Response, cr *contract.CallRun) {
	masked := state.masker.mask(string(resp.Body))
	cr.Response.Keys = collectJSONPaths(masked)

	limit := BodyPreviewBytes
	if state.budget < limit {
		limit = state.budget
	}
	if limit < 0 {
		limit = 0
	}

	if len(masked) <= limit {
		cr.Response.Body = masked
		state.budget -= len(masked)
		return
	}

	preview := truncateUTF8(masked, limit)
	cr.Response.Body = preview
	cr.Response.Truncated = true
	state.budget -= len(preview)

	sum := sha256.Sum256([]byte(masked))
	cr.BodyHash = hex.EncodeToString(sum[:])
	cr.BodyEncoding = contract.BodyEncodingZstd

	if r.bodies == nil {
		r.logger.Warn("body store missing; keeping preview only", "run_id", state.job.RunID, "call", call.Name)
		return
	}

	compressed, err := zstdCompress([]byte(masked))
	if err != nil {
		r.logger.Warn("body zstd failed", "run_id", state.job.RunID, "call", call.Name, "error", err)
		return
	}
	key := fmt.Sprintf("%s/%s/%s/%d.zst", state.job.UserID, state.job.WorkflowID, state.job.RunID, call.Order)
	ref, err := r.bodies.Put(ctx, key, compressed, "application/zstd")
	if err != nil {
		r.logger.Warn("body upload failed", "run_id", state.job.RunID, "call", call.Name, "error", err)
		return
	}
	cr.BodyRef = ref
}

func zstdCompress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	enc, err := zstd.NewWriter(&buf, zstd.WithEncoderLevel(zstd.SpeedDefault))
	if err != nil {
		return nil, err
	}
	if _, err := enc.Write(data); err != nil {
		_ = enc.Close()
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// collectJSONPaths walks a JSON document and returns dotted paths for search
// (example: data, data.items, data.items[].id). Non-JSON bodies yield nil.
func collectJSONPaths(body string) []string {
	var root any
	if err := json.Unmarshal([]byte(body), &root); err != nil {
		return nil
	}
	seen := map[string]struct{}{}
	var out []string
	walkJSONPaths(root, "", 0, seen, &out)
	sort.Strings(out)
	return out
}

func walkJSONPaths(node any, prefix string, depth int, seen map[string]struct{}, out *[]string) {
	if len(*out) >= MaxResponseKeys || depth > MaxResponseKeyDepth {
		return
	}
	switch v := node.(type) {
	case map[string]any:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if len(*out) >= MaxResponseKeys {
				return
			}
			path := k
			if prefix != "" {
				path = prefix + "." + k
			}
			if _, ok := seen[path]; !ok {
				seen[path] = struct{}{}
				*out = append(*out, path)
			}
			walkJSONPaths(v[k], path, depth+1, seen, out)
		}
	case []any:
		path := "[]"
		if prefix != "" {
			path = prefix + "[]"
		}
		if _, ok := seen[path]; !ok {
			seen[path] = struct{}{}
			*out = append(*out, path)
		}
		if len(v) == 0 || len(*out) >= MaxResponseKeys {
			return
		}
		walkJSONPaths(v[0], path, depth+1, seen, out)
	}
}

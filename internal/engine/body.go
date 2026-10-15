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
	// MaxResponseKeyDepth limits how deep we walk a JSON document.
	MaxResponseKeyDepth = 8
)

func (r *Runner) storeBody(ctx context.Context, state *run, call contract.JobCall, resp *Response, cr *contract.CallRun) {
	masked := state.masker.mask(string(resp.Body))
	// Keys from the full masked body (never the preview). Skipped when the body
	// is too large to walk safely; the call still succeeds and claim-checks to S3.
	if len(masked) <= MaxBodyKeysBytes {
		cr.Response.Keys = collectJSONPaths(masked)
	}

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

// collectJSONPaths walks a JSON document and returns dotted paths for search.
// Schema paths (`items[].id`) come first, then indexed paths (`items[0].id`, …)
// for every array element. There is no key-count cap: keys are taken from the
// full body even when the stored preview is truncated. Non-JSON bodies yield nil.
func collectJSONPaths(body string) []string {
	var root any
	if err := json.Unmarshal([]byte(body), &root); err != nil {
		return nil
	}
	seen := map[string]struct{}{}
	var out []string
	walkJSONPaths(root, "", 0, false, seen, &out)
	walkJSONPaths(root, "", 0, true, seen, &out)
	sort.Strings(out)
	return out
}

func addJSONPath(path string, seen map[string]struct{}, out *[]string) {
	if _, ok := seen[path]; ok {
		return
	}
	seen[path] = struct{}{}
	*out = append(*out, path)
}

// walkJSONPaths walks node. When indexed is false, only schema forms (items[])
// are emitted. When true, only concrete indices (items[0], items[1], …) are
// emitted.
func walkJSONPaths(node any, prefix string, depth int, indexed bool, seen map[string]struct{}, out *[]string) {
	if depth > MaxResponseKeyDepth {
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
			path := k
			if prefix != "" {
				path = prefix + "." + k
			}
			addJSONPath(path, seen, out)
			walkJSONPaths(v[k], path, depth+1, indexed, seen, out)
		}
	case []any:
		if !indexed {
			generic := "[]"
			if prefix != "" {
				generic = prefix + "[]"
			}
			addJSONPath(generic, seen, out)
			// Union of keys across elements under the schema path.
			for _, elem := range v {
				walkJSONPaths(elem, generic, depth+1, false, seen, out)
			}
			return
		}
		for i, elem := range v {
			path := fmt.Sprintf("[%d]", i)
			if prefix != "" {
				path = fmt.Sprintf("%s[%d]", prefix, i)
			}
			addJSONPath(path, seen, out)
			walkJSONPaths(elem, path, depth+1, true, seen, out)
		}
	}
}

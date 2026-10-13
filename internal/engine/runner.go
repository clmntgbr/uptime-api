package engine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"go-api/internal/contract"
	"go-api/internal/domain/port"
	domainrule "go-api/internal/domain/rule"
	"go-api/internal/domain/variable"

	"github.com/google/uuid"
)

const (
	// InlineBodyBytes is the part of a response body kept in the result (and the database).
	InlineBodyBytes = 256 * 1024
	// InlineBudgetBytes caps the inline bodies of a whole run, so that the
	// result message stays under the 4 MB Kafka limit with 20 calls.
	InlineBudgetBytes = 2 * 1024 * 1024
	// RequestBodyBytes is the part of a request body kept in the result.
	RequestBodyBytes = 16 * 1024
)

type Runner struct {
	executor Executor
	bodies   port.BodyStore
	clock    port.Clock
	logger   *slog.Logger
}

func NewRunner(executor Executor, bodies port.BodyStore, clock port.Clock, logger *slog.Logger) *Runner {
	return &Runner{executor: executor, bodies: bodies, clock: clock, logger: logger}
}

var _ port.WorkflowRunner = (*Runner)(nil)

// run is the state of one execution.
type run struct {
	job      contract.Job
	values   map[string]string
	masker   *masker
	session  Session
	budget   int
	producer map[string]string
}

// Run executes the calls in order and never returns an error: failures are part of the result.
func (r *Runner) Run(ctx context.Context, job contract.Job, secrets map[string]string) contract.Result {
	started := r.clock.Now()
	timeout := time.Duration(job.TimeoutMS) * time.Millisecond
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if !job.Deadline.IsZero() {
		var cancelDeadline context.CancelFunc
		ctx, cancelDeadline = context.WithDeadline(ctx, job.Deadline)
		defer cancelDeadline()
	}

	state := &run{
		job:      job,
		values:   map[string]string{variable.BuiltinRunID: job.RunID},
		masker:   newMasker(secrets),
		session:  r.executor.NewSession(),
		budget:   InlineBudgetBytes,
		producer: map[string]string{},
	}
	runVariables := map[string]string{}
	for _, v := range job.Variables {
		state.values[v.Name] = v.Value
		runVariables[v.Name] = v.Value
	}
	for name, value := range secrets {
		state.values[name] = value
		runVariables[name] = variable.MaskedValue
	}
	for _, c := range job.Calls {
		for _, e := range c.Extractions {
			state.producer[e.Name] = c.Name
		}
	}

	calls := append([]contract.JobCall(nil), job.Calls...)
	sort.SliceStable(calls, func(i, j int) bool { return calls[i].Order < calls[j].Order })

	result := contract.Result{
		V:          contract.ResultVersion,
		ID:         uuid.NewString(),
		Type:       contract.ResultType,
		UserID:     job.UserID,
		WorkflowID: job.WorkflowID,
		RunID:      job.RunID,
		Trigger:    job.Trigger,
		StartedAt:  started,
		Variables:  runVariables,
		Calls:      make([]contract.CallRun, 0, len(calls)),
	}

	for _, call := range calls {
		var cr contract.CallRun
		if ctx.Err() != nil {
			cr = skippedCall(call, r.clock.Now(), "workflow timeout reached")
		} else {
			cr = r.runCall(ctx, state, call)
		}
		switch cr.Status {
		case contract.CallStatusSuccess:
			result.Counts.Success++
		case contract.CallStatusFailed:
			result.Counts.Failed++
		case contract.CallStatusError:
			result.Counts.Error++
		default:
			result.Counts.Skipped++
		}
		result.Calls = append(result.Calls, cr)
	}

	result.FinishedAt = r.clock.Now()
	result.DurationMS = result.FinishedAt.Sub(started).Milliseconds()
	result.Status = contract.RunStatusSuccess
	if result.Counts.Success != len(calls) {
		result.Status = contract.RunStatusFailed
	}
	result.EmittedAt = r.clock.Now()
	return result
}

func skippedCall(call contract.JobCall, at time.Time, reason string) contract.CallRun {
	return contract.CallRun{
		CallID:     call.ID,
		Name:       call.Name,
		Order:      call.Order,
		Status:     contract.CallStatusSkipped,
		Reason:     reason,
		Rules:      []contract.RuleRun{},
		StartedAt:  at,
		FinishedAt: at,
	}
}

func (r *Runner) runCall(ctx context.Context, state *run, call contract.JobCall) contract.CallRun {
	started := r.clock.Now()
	state.values[variable.BuiltinTimestamp] = strconv.FormatInt(started.Unix(), 10)
	state.values[variable.BuiltinUUID] = uuid.NewString()

	rendered, used, missing := state.render(call)
	if len(missing) > 0 {
		return skippedCall(call, started, state.missingReason(missing))
	}

	cr := contract.CallRun{
		CallID:        call.ID,
		Name:          call.Name,
		Order:         call.Order,
		UsedVariables: used,
		StartedAt:     started,
		Rules:         []contract.RuleRun{},
	}
	cr.Request = state.requestInfo(rendered)

	if err := validateTarget(rendered.URL); err != nil {
		cr.Status = contract.CallStatusError
		cr.Error = &contract.CallError{Type: contract.ErrorTypeInvalid, Message: err.Error()}
		cr.FinishedAt = r.clock.Now()
		return cr
	}

	resp, err := state.session.Do(ctx, rendered.request)
	cr.FinishedAt = r.clock.Now()
	if err != nil {
		var execErr *ExecError
		if !errors.As(err, &execErr) {
			execErr = &ExecError{Type: contract.ErrorTypeOther, Message: err.Error()}
		}
		cr.Status = contract.CallStatusError
		cr.Error = &contract.CallError{Type: execErr.Type, Message: state.masker.mask(execErr.Message)}
		if execErr.Partial != nil {
			cr.TimingsMS = execErr.Partial.Timings
			cr.Network = contract.NetworkInfo{IP: execErr.Partial.IP, IPVersion: execErr.Partial.IPVersion, Redirects: state.maskAll(execErr.Partial.Redirects)}
			cr.TLS = execErr.Partial.TLS
		}
		return cr
	}

	cr.HTTPStatus = resp.Status
	cr.TimingsMS = resp.Timings
	cr.Network = contract.NetworkInfo{IP: resp.IP, IPVersion: resp.IPVersion, Redirects: state.maskAll(resp.Redirects)}
	cr.TLS = resp.TLS

	b := &body{raw: resp.Body}
	cr.ExtractedVariables = state.extractAll(call, resp, b)

	rules := make([]domainrule.Rule, 0, len(call.Rules))
	for _, jr := range call.Rules {
		rules = append(rules, domainrule.Rule{
			Subject:  domainrule.Subject(jr.Subject),
			Operator: domainrule.Operator(jr.Operator),
			Target:   jr.Target,
			Value:    rendered.ruleValues[jr.ID],
		})
	}
	if len(rules) == 0 {
		rules = domainrule.Defaults()
	}
	obs := observation{
		status:  resp.Status,
		totalMS: resp.Timings.Total,
		size:    int64(len(resp.Body)),
		headers: resp.Headers,
		body:    b,
		tls:     resp.TLS,
		now:     r.clock.Now(),
	}
	cr.Status = contract.CallStatusSuccess
	for _, rule := range rules {
		rr := evaluate(rule, obs)
		rr.Text = state.masker.mask(rr.Text)
		rr.Observed = state.masker.mask(rr.Observed)
		rr.Expected = state.masker.mask(rr.Expected)
		if rr.Subject == string(domainrule.SubjectHeader) && isSensitiveHeader(rule.Target) {
			rr.Observed = variable.MaskedValue
		}
		if !rr.Passed {
			cr.Status = contract.CallStatusFailed
		}
		cr.Rules = append(cr.Rules, rr)
	}

	cr.Response = state.responseInfo(resp)
	r.storeBody(ctx, state, call, resp, &cr)
	return cr
}

// storeBody keeps the beginning of the body inline and the whole (masked) body
// in the object store when it does not fit.
func (r *Runner) storeBody(ctx context.Context, state *run, call contract.JobCall, resp *Response, cr *contract.CallRun) {
	masked := state.masker.mask(string(resp.Body))
	limit := InlineBodyBytes
	if state.budget < limit {
		limit = state.budget
	}
	if len(masked) <= limit {
		cr.Response.Body = masked
		state.budget -= len(masked)
		return
	}

	cr.Response.Body = truncateUTF8(masked, limit)
	cr.Response.Truncated = true
	state.budget -= len(cr.Response.Body)
	if r.bodies == nil {
		return
	}
	key := fmt.Sprintf("%s/%s/%s/%d.body", state.job.UserID, state.job.WorkflowID, state.job.RunID, call.Order)
	ref, err := r.bodies.Put(ctx, key, []byte(masked), resp.Headers.Get("Content-Type"))
	if err != nil {
		r.logger.Warn("body upload failed", "run_id", state.job.RunID, "call", call.Name, "error", err)
		return
	}
	cr.BodyRef = ref
}

type renderedCall struct {
	request    Request
	URL        string
	query      map[string]string
	ruleValues map[string]string
}

// render resolves the placeholders of the call; missing lists the unresolved names.
func (s *run) render(call contract.JobCall) (renderedCall, []string, []string) {
	usedSet := map[string]struct{}{}
	missingSet := map[string]struct{}{}
	lookup := func(name string) (string, bool) {
		v, ok := s.values[name]
		if ok {
			usedSet[name] = struct{}{}
		}
		return v, ok
	}
	render := func(t string) string {
		out, missing := variable.Render(t, lookup)
		for _, m := range missing {
			missingSet[m] = struct{}{}
		}
		return out
	}

	rawURL := render(call.URL)
	headers := make(map[string]string, len(call.Headers))
	for k, v := range call.Headers {
		headers[k] = render(v)
	}
	query := make(map[string]string, len(call.Query))
	for k, v := range call.Query {
		query[k] = render(v)
	}
	bodyText := render(call.Body)
	ruleValues := make(map[string]string, len(call.Rules))
	for _, rule := range call.Rules {
		ruleValues[rule.ID] = render(rule.Value)
	}

	finalURL := rawURL
	if len(query) > 0 {
		if u, err := url.Parse(rawURL); err == nil {
			q := u.Query()
			for k, v := range query {
				q.Set(k, v)
			}
			u.RawQuery = q.Encode()
			finalURL = u.String()
		}
	}

	rc := renderedCall{
		URL:        finalURL,
		query:      query,
		ruleValues: ruleValues,
		request: Request{
			Method:    call.Method,
			URL:       finalURL,
			Headers:   headers,
			Body:      bodyText,
			Timeout:   time.Duration(call.TimeoutMS) * time.Millisecond,
			VerifyTLS: call.VerifyTLS,
			IPVersion: call.IPVersion,
			ProxyURL:  call.ProxyURL,
		},
	}
	return rc, sortedSet(usedSet, true), sortedSet(missingSet, false)
}

func (s *run) missingReason(missing []string) string {
	parts := make([]string, 0, len(missing))
	for _, name := range missing {
		if producer, ok := s.producer[name]; ok {
			parts = append(parts, fmt.Sprintf("{{%s}} (not extracted by %q)", name, producer))
		} else {
			parts = append(parts, fmt.Sprintf("{{%s}}", name))
		}
	}
	return "missing variable " + strings.Join(parts, ", ")
}

func (s *run) extractAll(call contract.JobCall, resp *Response, b *body) map[string]string {
	if len(call.Extractions) == 0 {
		return nil
	}
	out := make(map[string]string, len(call.Extractions))
	for _, e := range call.Extractions {
		value, err := extract(e, resp, b)
		if err != nil {
			continue
		}
		s.values[e.Name] = value
		if isSecretLikeName(e.Name) {
			s.masker.add(value)
		}
		out[e.Name] = s.masker.mask(value)
	}
	return out
}

func (s *run) requestInfo(rc renderedCall) contract.RequestInfo {
	info := contract.RequestInfo{
		Method:  rc.request.Method,
		URL:     s.masker.mask(rc.URL),
		Headers: s.masker.maskHeaders(rc.request.Headers),
		Size:    len(rc.request.Body),
	}
	if len(rc.query) > 0 {
		info.Query = make(map[string]string, len(rc.query))
		for k, v := range rc.query {
			if isSecretLikeName(k) {
				info.Query[k] = variable.MaskedValue
			} else {
				info.Query[k] = s.masker.mask(v)
			}
		}
	}
	maskedBody := s.masker.mask(rc.request.Body)
	if len(maskedBody) > RequestBodyBytes {
		maskedBody = truncateUTF8(maskedBody, RequestBodyBytes)
		info.BodyTruncated = true
	}
	info.Body = maskedBody
	return info
}

func (s *run) responseInfo(resp *Response) contract.ResponseInfo {
	return contract.ResponseInfo{
		Headers:     s.masker.maskHTTPHeaders(resp.Headers),
		Size:        int64(len(resp.Body)),
		Protocol:    resp.Protocol,
		Compression: resp.Compression,
	}
}

func (s *run) maskAll(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = s.masker.mask(v)
	}
	return out
}

func validateTarget(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid url")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("only http and https urls are allowed")
	}
	if u.Hostname() == "" {
		return fmt.Errorf("url has no host")
	}
	return nil
}

func truncateUTF8(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}

func sortedSet(set map[string]struct{}, skipBuiltins bool) []string {
	if len(set) == 0 {
		return nil
	}
	out := make([]string, 0, len(set))
	for k := range set {
		if skipBuiltins && variable.IsBuiltin(k) {
			continue
		}
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

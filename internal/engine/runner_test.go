package engine

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"go-api/internal/contract"
	"go-api/internal/domain/variable"

	"github.com/google/uuid"
)

type fakeExecutor struct {
	handler  func(req Request) (*Response, error)
	requests []Request
}

func (f *fakeExecutor) NewSession() Session { return f }

func (f *fakeExecutor) Do(_ context.Context, req Request) (*Response, error) {
	f.requests = append(f.requests, req)
	return f.handler(req)
}

type fakeStore struct{ puts map[string][]byte }

func (s *fakeStore) Put(_ context.Context, key string, data []byte, _ string) (string, error) {
	if s.puts == nil {
		s.puts = map[string][]byte{}
	}
	s.puts[key] = data
	return "s3://bodies/" + key, nil
}

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

func jsonResponse(status int, body string, headers ...string) *Response {
	h := http.Header{"Content-Type": {"application/json"}}
	for i := 0; i+1 < len(headers); i += 2 {
		h.Set(headers[i], headers[i+1])
	}
	return &Response{Status: status, Protocol: "HTTP/2.0", Headers: h, Body: []byte(body), Timings: contract.Timings{Total: 120}}
}

func newJob(calls ...contract.JobCall) contract.Job {
	for i := range calls {
		calls[i].Order = i + 1
		if calls[i].ID == "" {
			calls[i].ID = uuid.NewString()
		}
		if calls[i].Method == "" {
			calls[i].Method = "GET"
		}
		calls[i].TimeoutMS = 1000
	}
	return contract.Job{
		V: contract.JobVersion, ID: uuid.NewString(), Type: contract.JobType,
		UserID: uuid.NewString(), WorkflowID: uuid.NewString(), RunID: uuid.NewString(),
		Trigger: contract.TriggerSchedule, TimeoutMS: 300000, Deadline: time.Now().Add(time.Minute),
		Variables: []contract.JobVariable{{Name: "base_url", Type: "static", Value: "https://api.example.com"}},
		Calls:     calls,
	}
}

func newRunner(exec Executor, store *fakeStore) *Runner {
	var bodies interface {
		Put(context.Context, string, []byte, string) (string, error)
	}
	if store != nil {
		bodies = store
	}
	return NewRunner(exec, bodies, fixedClock{time.Now()}, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestRun_ChainsVariablesAndMasksSecrets(t *testing.T) {
	exec := &fakeExecutor{handler: func(req Request) (*Response, error) {
		switch {
		case strings.HasSuffix(req.URL, "/login"):
			return jsonResponse(200, `{"access_token":"tok-123456","user":{"id":42}}`), nil
		default:
			return jsonResponse(200, `{"items":[{"id":"o-1"}],"echo":"hunter22"}`), nil
		}
	}}
	job := newJob(
		contract.JobCall{
			Name: "login", Method: "POST", URL: "{{base_url}}/login", Body: `{"password":"{{password}}"}`,
			Extractions: []contract.JobExtraction{
				{Name: "access_token", Source: "json", Expression: "$.access_token"},
				{Name: "user_id", Source: "json", Expression: "$.user.id"},
			},
			Rules: []contract.JobRule{{ID: "r1", Subject: "status", Operator: "is", Value: "200"}},
		},
		contract.JobCall{
			Name: "orders", URL: "{{base_url}}/users/{{user_id}}/orders",
			Headers: map[string]string{"Authorization": "Bearer {{access_token}}", "X-Trace": "{{password}}"},
			Rules: []contract.JobRule{
				{ID: "r2", Subject: "json", Operator: "is", Target: "$.items[0].id", Value: "o-1"},
				{ID: "r3", Subject: "response_time", Operator: "exceeds", Value: "800"},
			},
		},
	)

	result := newRunner(exec, nil).Run(context.Background(), job, map[string]string{"password": "hunter22"})

	if result.Status != contract.RunStatusSuccess || result.Counts.Success != 2 {
		t.Fatalf("unexpected result %+v", result)
	}
	if exec.requests[0].Body != `{"password":"hunter22"}` {
		t.Fatalf("secret must be sent in clear to the target, got %q", exec.requests[0].Body)
	}
	if exec.requests[1].URL != "https://api.example.com/users/42/orders" || exec.requests[1].Headers["Authorization"] != "Bearer tok-123456" {
		t.Fatalf("variables not resolved: %+v", exec.requests[1])
	}

	login, orders := result.Calls[0], result.Calls[1]
	if strings.Contains(login.Request.Body, "hunter22") || login.ExtractedVariables["access_token"] != variable.MaskedValue {
		t.Fatalf("secrets leaked in login result: %+v", login)
	}
	if login.ExtractedVariables["user_id"] != "42" {
		t.Fatalf("non secret variable must stay readable: %v", login.ExtractedVariables)
	}
	if orders.Request.Headers["Authorization"] != variable.MaskedValue || orders.Request.Headers["X-Trace"] != variable.MaskedValue {
		t.Fatalf("headers not masked: %v", orders.Request.Headers)
	}
	if strings.Contains(orders.Response.Body, "hunter22") {
		t.Fatal("secret echoed in the response body must be masked")
	}
	if result.Variables["password"] != variable.MaskedValue {
		t.Fatalf("run variables must mask secrets: %v", result.Variables)
	}
	if strings.Join(orders.UsedVariables, ",") != "access_token,base_url,password,user_id" {
		t.Fatalf("unexpected used variables %v", orders.UsedVariables)
	}
}

func TestRun_FailedExtractionSkipsDependents(t *testing.T) {
	exec := &fakeExecutor{handler: func(req Request) (*Response, error) {
		return jsonResponse(200, `<html>maintenance</html>`), nil
	}}
	job := newJob(
		contract.JobCall{Name: "list", URL: "{{base_url}}/orders", Extractions: []contract.JobExtraction{{Name: "first_id", Source: "json", Expression: "$.items[0].id"}}},
		contract.JobCall{Name: "detail", URL: "{{base_url}}/orders/{{first_id}}"},
		contract.JobCall{Name: "health", URL: "{{base_url}}/health"},
	)
	result := newRunner(exec, nil).Run(context.Background(), job, nil)

	if result.Calls[1].Status != contract.CallStatusSkipped || !strings.Contains(result.Calls[1].Reason, `{{first_id}} (not extracted by "list")`) {
		t.Fatalf("dependent call must be skipped with the reason: %+v", result.Calls[1])
	}
	if result.Calls[2].Status != contract.CallStatusSuccess || len(exec.requests) != 2 {
		t.Fatal("independent calls must still run")
	}
	if result.Status != contract.RunStatusFailed || result.Counts.Skipped != 1 {
		t.Fatalf("run with a skipped call is failed: %+v", result.Counts)
	}
}

func TestRun_DefaultRuleAndNetworkError(t *testing.T) {
	exec := &fakeExecutor{handler: func(req Request) (*Response, error) {
		if strings.Contains(req.URL, "down") {
			return nil, &ExecError{Type: contract.ErrorTypeDNS, Message: "no such host"}
		}
		return jsonResponse(503, `{}`), nil
	}}
	job := newJob(
		contract.JobCall{Name: "unavailable", URL: "https://api.example.com/"},
		contract.JobCall{Name: "down", URL: "https://down.example.com/"},
	)
	result := newRunner(exec, nil).Run(context.Background(), job, nil)

	first := result.Calls[0]
	if first.Status != contract.CallStatusFailed || len(first.Rules) != 1 || first.Rules[0].Text != "status is 2xx" || first.Rules[0].Observed != "503" {
		t.Fatalf("default rule must fail on 503: %+v", first)
	}
	second := result.Calls[1]
	if second.Status != contract.CallStatusError || second.Error.Type != contract.ErrorTypeDNS {
		t.Fatalf("network errors make the call error: %+v", second)
	}
}

func TestRun_AllRulesEvaluated(t *testing.T) {
	exec := &fakeExecutor{handler: func(req Request) (*Response, error) {
		r := jsonResponse(500, `{"status":"down","count":7}`, "X-Version", "2.1")
		r.Timings.Total = 1240
		return r, nil
	}}
	job := newJob(contract.JobCall{Name: "check", URL: "https://api.example.com/", Rules: []contract.JobRule{
		{ID: "1", Subject: "status", Operator: "is", Value: "200"},
		{ID: "2", Subject: "response_time", Operator: "exceeds", Value: "800"},
		{ID: "3", Subject: "json", Operator: "is_not", Target: "$.status", Value: "down"},
		{ID: "4", Subject: "json", Operator: "exceeds", Target: "$.count", Value: "10"},
		{ID: "5", Subject: "header", Operator: "contains", Target: "x-version", Value: "2."},
		{ID: "6", Subject: "body", Operator: "not_contains", Value: "error"},
		{ID: "7", Subject: "certificate", Operator: "expires_within", Value: "14"},
	}})
	result := newRunner(exec, nil).Run(context.Background(), job, nil)
	rules := result.Calls[0].Rules

	want := []bool{false, false, false, true, true, true, false}
	if len(rules) != len(want) {
		t.Fatalf("every rule must be evaluated, got %d", len(rules))
	}
	for i, passed := range want {
		if rules[i].Passed != passed {
			t.Errorf("rule %q: passed=%v, want %v (observed %q)", rules[i].Text, rules[i].Passed, passed, rules[i].Observed)
		}
	}
	if rules[1].Observed != "1240 ms" || rules[1].Text != "response time exceeds 800 ms" {
		t.Errorf("unexpected response time result %+v", rules[1])
	}
}

func TestRun_CertificateRule(t *testing.T) {
	now := time.Now()
	exec := &fakeExecutor{handler: func(req Request) (*Response, error) {
		r := jsonResponse(200, `{}`)
		r.TLS = &contract.TLSInfo{ExpiresAt: now.Add(90 * 24 * time.Hour), ChainExpiresAt: now.Add(10 * 24 * time.Hour), ChainValid: true, NameMatch: true}
		return r, nil
	}}
	job := newJob(contract.JobCall{Name: "tls", URL: "https://api.example.com/", Rules: []contract.JobRule{
		{ID: "1", Subject: "certificate", Operator: "expires_within", Value: "14"},
		{ID: "2", Subject: "certificate", Operator: "chain_invalid"},
	}})
	rules := newRunner(exec, nil).Run(context.Background(), job, nil).Calls[0].Rules
	if rules[0].Passed || !strings.HasPrefix(rules[0].Observed, "9 days left") {
		t.Fatalf("an intermediate expiring in 10 days must trip the 14 days rule: %+v", rules[0])
	}
	if !rules[1].Passed {
		t.Fatalf("valid chain must pass: %+v", rules[1])
	}
}

func TestRun_LargeBodyGoesToObjectStore(t *testing.T) {
	large := `{"data":"` + strings.Repeat("a", InlineBodyBytes) + `"}`
	exec := &fakeExecutor{handler: func(req Request) (*Response, error) { return jsonResponse(200, large), nil }}
	store := &fakeStore{}
	job := newJob(contract.JobCall{Name: "big", URL: "https://api.example.com/"})

	cr := newRunner(exec, store).Run(context.Background(), job, nil).Calls[0]
	if !cr.Response.Truncated || len(cr.Response.Body) != InlineBodyBytes || cr.Response.Size != int64(len(large)) {
		t.Fatalf("body must be truncated inline: truncated=%v len=%d size=%d", cr.Response.Truncated, len(cr.Response.Body), cr.Response.Size)
	}
	if !strings.HasPrefix(cr.BodyRef, "s3://bodies/") || len(store.puts) != 1 {
		t.Fatalf("full body must be uploaded, ref %q", cr.BodyRef)
	}
}

func TestRun_LargeBodyWithoutStoreIsTruncated(t *testing.T) {
	large := `{"data":"` + strings.Repeat("a", InlineBodyBytes) + `"}`
	exec := &fakeExecutor{handler: func(req Request) (*Response, error) { return jsonResponse(200, large), nil }}
	job := newJob(contract.JobCall{Name: "big", URL: "https://api.example.com/"})

	cr := newRunner(exec, nil).Run(context.Background(), job, nil).Calls[0]
	if !cr.Response.Truncated || len(cr.Response.Body) != InlineBodyBytes || cr.BodyRef != "" {
		t.Fatalf("body must be truncated without ref: truncated=%v len=%d ref=%q", cr.Response.Truncated, len(cr.Response.Body), cr.BodyRef)
	}
	if cr.Status != contract.CallStatusSuccess {
		t.Fatalf("truncation must not fail the call, got %q", cr.Status)
	}
}

func TestRun_InlineBudgetKeepsResultUnderKafkaLimit(t *testing.T) {
	body := strings.Repeat("b", InlineBodyBytes)
	exec := &fakeExecutor{handler: func(req Request) (*Response, error) { return jsonResponse(200, body), nil }}
	calls := make([]contract.JobCall, 20)
	for i := range calls {
		calls[i] = contract.JobCall{Name: uuid.NewString(), URL: "https://api.example.com/"}
	}
	result := newRunner(exec, &fakeStore{}).Run(context.Background(), newJob(calls...), nil)
	total := 0
	for _, c := range result.Calls {
		total += len(c.Response.Body)
	}
	if total > InlineBudgetBytes {
		t.Fatalf("inline bodies %d exceed the run budget %d", total, InlineBudgetBytes)
	}
}

func TestRun_RejectsNonHTTPTargets(t *testing.T) {
	exec := &fakeExecutor{handler: func(req Request) (*Response, error) { return jsonResponse(200, `{}`), nil }}
	job := newJob(contract.JobCall{Name: "file", URL: "{{target}}"})
	job.Variables = append(job.Variables, contract.JobVariable{Name: "target", Type: "static", Value: "file:///etc/passwd"})

	cr := newRunner(exec, nil).Run(context.Background(), job, nil).Calls[0]
	if cr.Status != contract.CallStatusError || cr.Error.Type != contract.ErrorTypeInvalid || len(exec.requests) != 0 {
		t.Fatalf("non http scheme must be refused before any request: %+v", cr)
	}
}

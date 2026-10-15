package execution

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"go-api/internal/application/messaging"
	"go-api/internal/contract"

	"github.com/google/uuid"
)

type stubOpener struct{ err error }

func (o stubOpener) Open(*contract.SealedSecrets) (map[string]string, error) {
	return map[string]string{}, o.err
}

type stubRunner struct{ calls int }

func (r *stubRunner) Run(_ context.Context, job contract.Job, _ map[string]string) contract.Result {
	r.calls++
	return contract.Result{RunID: job.RunID, WorkflowID: job.WorkflowID, Status: contract.RunStatusSuccess}
}

type stubPublisher struct {
	err       error
	published int
}

func (p *stubPublisher) PublishResult(context.Context, contract.Result) error {
	p.published++
	return p.err
}

func (p *stubPublisher) PublishCallProgress(context.Context, contract.CallProgress) error {
	return nil
}

type clockAt time.Time

func (c clockAt) Now() time.Time { return time.Time(c) }

func validJob(deadline time.Time) []byte {
	job := contract.Job{
		V: contract.JobVersion, ID: uuid.NewString(), Type: contract.JobType,
		UserID: uuid.NewString(), WorkflowID: uuid.NewString(), RunID: uuid.NewString(),
		Trigger: contract.TriggerSchedule, TimeoutMS: 1000, Deadline: deadline,
		Calls: []contract.JobCall{{ID: uuid.NewString(), Order: 1, Name: "a", Method: "GET", URL: "https://x.io", TimeoutMS: 1000}},
	}
	raw, _ := json.Marshal(job)
	return raw
}

func handler(opener stubOpener, runner *stubRunner, pub *stubPublisher) *ExecuteJobHandler {
	return NewExecuteJobHandler(opener, runner, pub, clockAt(time.Now()), slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestExecuteJob_Success(t *testing.T) {
	runner, pub := &stubRunner{}, &stubPublisher{}
	if err := handler(stubOpener{}, runner, pub).Handle(context.Background(), validJob(time.Now().Add(time.Minute))); err != nil {
		t.Fatal(err)
	}
	if runner.calls != 1 || pub.published != 1 {
		t.Fatalf("runner=%d published=%d", runner.calls, pub.published)
	}
}

func TestExecuteJob_InvalidJobIsNotRetried(t *testing.T) {
	var nonRetryable *messaging.NonRetryableError
	for _, payload := range [][]byte{[]byte("{"), []byte(`{"v":2}`)} {
		err := handler(stubOpener{}, &stubRunner{}, &stubPublisher{}).Handle(context.Background(), payload)
		if !errors.As(err, &nonRetryable) {
			t.Fatalf("expected non retryable error, got %v", err)
		}
	}
}

func TestExecuteJob_PastDeadlineIsDropped(t *testing.T) {
	runner, pub := &stubRunner{}, &stubPublisher{}
	if err := handler(stubOpener{}, runner, pub).Handle(context.Background(), validJob(time.Now().Add(-time.Second))); err != nil {
		t.Fatal(err)
	}
	if runner.calls != 0 || pub.published != 0 {
		t.Fatal("a job past its deadline must not run")
	}
}

func TestExecuteJob_UnreadableSecretsAreNotRetried(t *testing.T) {
	runner := &stubRunner{}
	err := handler(stubOpener{err: errors.New("bad key")}, runner, &stubPublisher{}).Handle(context.Background(), validJob(time.Now().Add(time.Minute)))
	var nonRetryable *messaging.NonRetryableError
	if !errors.As(err, &nonRetryable) || runner.calls != 0 {
		t.Fatalf("expected non retryable error without execution, got %v", err)
	}
}

func TestExecuteJob_PublishFailureIsRetryable(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	pub := &stubPublisher{err: errors.New("broker down")}
	err := handler(stubOpener{}, &stubRunner{}, pub).Handle(ctx, validJob(time.Now().Add(time.Minute)))
	var retryable *messaging.RetryableError
	if !errors.As(err, &retryable) || pub.published != 1 {
		t.Fatalf("expected retryable error after one attempt on a cancelled context, got %v (%d attempts)", err, pub.published)
	}
}

//go:build integration

package persistence_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"go-api/internal/application/command/execution"
	commandrun "go-api/internal/application/command/run"
	"go-api/internal/application/command/schedule"
	"go-api/internal/contract"
	"go-api/internal/domain/port"
	domainschedule "go-api/internal/domain/schedule"
	"go-api/internal/domain/variable"
	domainworkflow "go-api/internal/domain/workflow"
	"go-api/internal/engine"
	"go-api/internal/infrastructure/crypto"
	"go-api/internal/infrastructure/httpclient"
	"go-api/internal/infrastructure/persistence/outbox"
	"go-api/internal/infrastructure/persistence/processed"
	"go-api/internal/infrastructure/persistence/write"

	"github.com/google/uuid"
)

type capture struct {
	mu      sync.Mutex
	jobs    []contract.Job
	results []contract.Result
}

func (c *capture) PublishJob(_ context.Context, job contract.Job) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.jobs = append(c.jobs, job)
	return nil
}

func (c *capture) PublishResult(_ context.Context, r contract.Result) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.results = append(c.results, r)
	return nil
}

func (c *capture) jobFor(workflowID uuid.UUID) *contract.Job {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := len(c.jobs) - 1; i >= 0; i-- {
		if c.jobs[i].WorkflowID == workflowID.String() {
			return &c.jobs[i]
		}
	}
	return nil
}

func TestScheduleExecuteRecord(t *testing.T) {
	db := openDB(t)
	ctx := context.Background()
	userID := createUser(t, db)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			body, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(body), "hunter22") {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte(`{"access_token":"tok-abcdef"}`))
		case "/orders":
			if r.Header.Get("Authorization") != "Bearer tok-abcdef" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte(`{"items":[{"id":"o-1"}]}`))
		}
	}))
	defer target.Close()

	key, _ := crypto.GenerateSecretKey()
	cipher, _ := crypto.NewAESCipher(key)
	pub, priv, _ := crypto.GenerateSealKeys()
	sealer, _ := crypto.NewSealer(pub)
	opener, _ := crypto.NewOpener(pub, priv)

	globals := write.NewGlobalVariableRepository(db, cipher)
	secret, _ := variable.NewGlobal(userID, "password", "hunter22", true)
	if err := globals.Save(ctx, secret); err != nil {
		t.Fatal(err)
	}

	def := domainworkflow.Definition{
		Name: "flow", Interval: time.Minute, Enabled: true,
		Statics: []domainworkflow.StaticVariable{{Name: "base_url", Value: target.URL}},
		Calls: []domainworkflow.CallDefinition{
			{Name: "login", Method: "POST", URL: "{{base_url}}/login", Body: `{"password":"{{password}}"}`,
				Extractions: []domainworkflow.Extraction{{Name: "access_token", Source: domainworkflow.ExtractJSON, Expression: "$.access_token"}}},
			{Name: "orders", URL: "{{base_url}}/orders", Headers: map[string]string{"Authorization": "Bearer {{access_token}}"}},
		},
	}
	w, err := domainworkflow.New(userID, def, []string{"password"})
	if err != nil {
		t.Fatal(err)
	}
	workflows := write.NewWorkflowWriteRepository(db)
	states := write.NewStateRepository(db)
	if err := workflows.Save(ctx, w); err != nil {
		t.Fatal(err)
	}
	if err := states.Save(ctx, domainschedule.New(w.ID, time.Now().Add(-time.Hour), w.Interval)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Exec("DELETE FROM rule_results WHERE run_id IN (SELECT id FROM runs WHERE workflow_id = ?)", w.ID)
		db.Exec("DELETE FROM call_results WHERE workflow_id = ?", w.ID)
		db.Exec("DELETE FROM runs WHERE workflow_id = ?", w.ID)
		db.Exec("DELETE FROM workflows WHERE id = ?", w.ID)
	})

	runs := write.NewRunWriteRepository(db)
	claimer := processed.NewTxClaimer(db)
	outboxRepo := outbox.NewRepository(db)
	bus := &capture{}
	dispatch := schedule.NewDispatchDueRunsHandler(states, workflows, globals, runs, claimer, outboxRepo, sealer, bus,
		port.SystemClock{}, schedule.DispatchConfig{BatchSize: 1000, LeaseMargin: time.Minute}, logger)

	if _, err := dispatch.Handle(ctx); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	job := bus.jobFor(w.ID)
	if job == nil {
		t.Fatal("no job published")
	}
	raw, _ := json.Marshal(job)
	if strings.Contains(string(raw), "hunter22") || job.Secrets == nil || job.Secrets.Values["password"] == "" {
		t.Fatal("secret must travel sealed only")
	}
	state, _ := states.GetForUpdate(ctx, w.ID)
	if !state.Running() || state.LeaseRunID.String() != job.RunID || !state.NextRunAt.After(time.Now()) {
		t.Fatalf("lease not taken: %+v", state)
	}

	exec := execution.NewExecuteJobHandler(opener,
		engine.NewRunner(httpclient.NewExecutor(httpclient.Config{AllowPrivate: true}), nil, port.SystemClock{}, logger),
		bus, port.SystemClock{}, logger)
	if err := exec.Handle(ctx, raw); err != nil {
		t.Fatalf("execute: %v", err)
	}
	result := bus.results[len(bus.results)-1]
	if result.Status != contract.RunStatusSuccess {
		t.Fatalf("run must succeed: %+v", result.Calls)
	}

	record := commandrun.NewRecordResultHandler(states, runs, write.NewCertificateRepository(db), claimer, outboxRepo)
	payload, _ := json.Marshal(result)
	for i := 0; i < 2; i++ {
		if err := record.Handle(ctx, payload); err != nil {
			t.Fatalf("record #%d: %v", i, err)
		}
	}
	var count int64
	db.Model(&write.RunModel{}).Where("workflow_id = ?", w.ID).Count(&count)
	if count != 1 {
		t.Fatalf("expected 1 run, got %d", count)
	}
	state, _ = states.GetForUpdate(ctx, w.ID)
	if state.Running() || state.LastStatus != "success" || state.ConsecutiveSuccesses != 1 {
		t.Fatalf("state not completed: %+v", state)
	}
	var storedBody string
	db.Raw("SELECT request->>'body' FROM call_results WHERE workflow_id = ? AND position = 1", w.ID).Scan(&storedBody)
	if strings.Contains(storedBody, "hunter22") {
		t.Fatal("secret stored in clear in the call result")
	}

	// Overlap: a due pass while the lease is valid is recorded as skipped.
	db.Exec("UPDATE workflow_state SET next_run_at = now() - interval '1 minute' WHERE workflow_id = ?", w.ID)
	if _, err := dispatch.Handle(ctx); err != nil {
		t.Fatal(err)
	}
	db.Exec("UPDATE workflow_state SET next_run_at = now() - interval '1 minute' WHERE workflow_id = ?", w.ID)
	if _, err := dispatch.Handle(ctx); err != nil {
		t.Fatal(err)
	}
	var skipped int64
	db.Model(&write.RunModel{}).Where("workflow_id = ? AND status = 'skipped' AND reason = 'overlap'", w.ID).Count(&skipped)
	if skipped != 1 {
		t.Fatalf("expected 1 skipped run, got %d", skipped)
	}

	// Timeout: an expired lease becomes an error run, and its late result is dropped.
	leased := bus.jobFor(w.ID)
	db.Exec("UPDATE workflow_state SET lease_until = now() - interval '1 second', next_run_at = now() - interval '1 minute' WHERE workflow_id = ?", w.ID)
	if _, err := dispatch.Handle(ctx); err != nil {
		t.Fatal(err)
	}
	var timedOut write.RunModel
	if err := db.Where("id = ? AND status = 'error' AND reason = 'timeout'", leased.RunID).Take(&timedOut).Error; err != nil {
		t.Fatalf("timed out run not recorded: %v", err)
	}
	late := result
	late.RunID = leased.RunID
	late.ID = uuid.NewString()
	latePayload, _ := json.Marshal(late)
	if err := record.Handle(ctx, latePayload); err != nil {
		t.Fatal(err)
	}
	db.Model(&write.RunModel{}).Where("id = ?", leased.RunID).Count(&count)
	if count != 1 {
		t.Fatalf("a late result must not be recorded next to its timeout, got %d rows", count)
	}

	var events int64
	db.Table("outbox_events").Where("aggregate_id = ? AND event_type = 'run.recorded.v1'", w.ID.String()).Count(&events)
	if events != 3 {
		t.Fatalf("expected run.recorded.v1 for success, skipped and timeout, got %d", events)
	}
	db.Exec("DELETE FROM outbox_events WHERE aggregate_id = ?", w.ID.String())
}

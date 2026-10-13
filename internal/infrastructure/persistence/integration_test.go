//go:build integration

// Runs against a migrated Postgres: TEST_DATABASE_URL=postgres://... go test -tags integration ./internal/infrastructure/persistence/
package persistence_test

import (
	"context"
	"os"
	"testing"
	"time"

	"go-api/internal/contract"
	"go-api/internal/domain/certificate"
	"go-api/internal/domain/paginate"
	domainrun "go-api/internal/domain/run"
	"go-api/internal/domain/variable"
	domainworkflow "go-api/internal/domain/workflow"
	"go-api/internal/infrastructure/crypto"
	"go-api/internal/infrastructure/persistence/processed"
	"go-api/internal/infrastructure/persistence/read"
	"go-api/internal/infrastructure/persistence/write"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func openDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func createUser(t *testing.T, db *gorm.DB) uuid.UUID {
	t.Helper()
	id := uuid.New()
	now := time.Now().UTC()
	err := db.Create(&write.UserModel{
		ID: id, ClerkID: "user_" + id.String(), Email: id.String() + "@test.local", CreatedAt: now, UpdatedAt: now,
	}).Error
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Delete(&write.UserModel{}, "id = ?", id) })
	return id
}

func TestWorkflowLifecycle(t *testing.T) {
	db := openDB(t)
	ctx := context.Background()
	userID := createUser(t, db)

	key, _ := crypto.GenerateSecretKey()
	cipher, err := crypto.NewAESCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	globals := write.NewGlobalVariableRepository(db, cipher)
	secret, _ := variable.NewGlobal(userID, "password", "hunter2", true)
	if err := globals.Save(ctx, secret); err != nil {
		t.Fatal(err)
	}
	var stored write.VariableModel
	db.First(&stored, "id = ?", secret.ID)
	if stored.Value == "hunter2" {
		t.Fatal("secret stored in clear")
	}
	listed, err := globals.ListByUser(ctx, userID)
	if err != nil || len(listed) != 1 || listed[0].Value != "hunter2" {
		t.Fatalf("decrypt failed: %v %v", listed, err)
	}

	rule, _ := domainworkflow.ParseRule("response time exceeds 800 ms")
	def := domainworkflow.Definition{
		Name: "Checkout", Interval: 5 * time.Minute, Enabled: true,
		Statics: []domainworkflow.StaticVariable{{Name: "base_url", Value: "https://api.example.com"}},
		Calls: []domainworkflow.CallDefinition{
			{
				Name: "login", Method: "POST", URL: "{{base_url}}/login", Body: `{"p":"{{password}}"}`,
				Extractions: []domainworkflow.Extraction{{Name: "token", Source: domainworkflow.ExtractJSON, Expression: "$.token"}},
				Rules:       []domainworkflow.Rule{rule},
			},
			{Name: "orders", URL: "{{base_url}}/orders", Headers: map[string]string{"Authorization": "Bearer {{token}}"}},
		},
	}
	w, err := domainworkflow.New(userID, def, []string{"password"})
	if err != nil {
		t.Fatal(err)
	}
	past := time.Now().UTC().Add(-time.Hour)
	workflows := write.NewWorkflowWriteRepository(db)
	if err := workflows.Save(ctx, w, domainworkflow.NewState(w.ID, past, w.Interval)); err != nil {
		t.Fatal(err)
	}

	loaded, err := workflows.GetByID(ctx, w.ID)
	if err != nil || loaded == nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded.Calls) != 2 || len(loaded.Calls[0].Rules) != 1 || loaded.Calls[0].Extractions[0].Name != "token" || len(loaded.Statics) != 1 {
		t.Fatalf("unexpected workflow %+v", loaded)
	}
	if _, err := loaded.Update(def, []string{"password"}); err != nil {
		t.Fatal(err)
	}
	if err := workflows.Update(ctx, loaded, nil); err != nil {
		t.Fatalf("update: %v", err)
	}

	states := write.NewStateRepository(db)
	runID := uuid.New()
	err = states.WithTransaction(ctx, func(txCtx context.Context) error {
		due, err := states.LockDue(txCtx, time.Now().UTC(), 1000)
		if err != nil {
			return err
		}
		found := false
		for _, d := range due {
			if d.State.WorkflowID == w.ID {
				found = true
				if d.UserID != userID || d.Interval != 5*time.Minute {
					t.Errorf("unexpected due %+v", d)
				}
				d.State.Lease(runID, time.Now().UTC(), 6*time.Minute)
				if err := states.Save(txCtx, d.State); err != nil {
					return err
				}
			}
		}
		if !found {
			t.Error("workflow not due")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("lock due: %v", err)
	}

	claimer := processed.NewTxClaimer(db)
	runs := write.NewRunWriteRepository(db)
	certs := write.NewCertificateRepository(db)
	started := time.Now().UTC().Add(-2 * time.Second)
	callID := w.Calls[0].ID
	r := &domainrun.Run{
		ID: runID, WorkflowID: w.ID, UserID: userID, Trigger: contract.TriggerSchedule,
		Status: domainrun.StatusFailed, StartedAt: started, FinishedAt: started.Add(time.Second),
		Counts:    domainrun.Counts{Success: 1, Failed: 1},
		Variables: map[string]string{"password": variable.MaskedValue},
		Calls: []domainrun.CallResult{{
			CallID: &callID, Name: "login", Position: 1, Status: domainrun.CallFailed, HTTPStatus: 200,
			Request:  contract.RequestInfo{Method: "POST", URL: "https://api.example.com/login"},
			Response: contract.ResponseInfo{Body: `{"token":"x"}`, Size: 13},
			TLS:      &contract.TLSInfo{Host: "api.example.com", Port: 443, ExpiresAt: started.Add(240 * time.Hour)},
			Rules: []domainrun.RuleResult{{Position: 1, Subject: "response_time", Operator: "exceeds",
				Text: "response time exceeds 800 ms", Observed: "950 ms", Expected: "800 ms", Passed: false}},
			StartedAt: started, FinishedAt: started.Add(time.Second),
		}},
	}
	for attempt := 0; attempt < 2; attempt++ {
		err = states.WithTransaction(ctx, func(txCtx context.Context) error {
			claimed, err := claimer.MarkProcessed(txCtx, runID.String(), "record_run")
			if err != nil || !claimed {
				return err
			}
			if err := runs.Save(txCtx, r); err != nil {
				return err
			}
			if err := certs.Upsert(txCtx, []certificate.Certificate{{
				UserID: userID, Host: "api.example.com", Port: 443, ExpiresAt: started, ChainExpiresAt: started, SeenAt: started,
			}}); err != nil {
				return err
			}
			state, err := states.GetForUpdate(txCtx, w.ID)
			if err != nil {
				return err
			}
			state.Complete(runID, string(r.Status), r.FinishedAt)
			return states.Save(txCtx, *state)
		})
		if err != nil {
			t.Fatalf("record attempt %d: %v", attempt, err)
		}
	}
	t.Cleanup(func() {
		db.Exec("DELETE FROM processed_events WHERE event_id = ?", runID)
		db.Exec("DELETE FROM rule_results WHERE run_id = ?", runID)
		db.Exec("DELETE FROM call_results WHERE run_id = ?", runID)
		db.Exec("DELETE FROM runs WHERE id = ?", runID)
	})

	var count int64
	db.Model(&write.RunModel{}).Where("id = ?", runID).Count(&count)
	if count != 1 {
		t.Fatalf("redelivered result must be recorded once, got %d runs", count)
	}

	runReader := read.NewRunQueryRepository(db)
	q := paginate.PaginateQuery{}
	q.Normalize()
	views, total, err := runReader.ListByWorkflow(ctx, userID, w.ID, q)
	if err != nil || total != 1 || len(views) != 1 || views[0].Status != domainrun.StatusFailed {
		t.Fatalf("list runs: %v %d %v", views, total, err)
	}
	detail, err := runReader.FindByID(ctx, userID, runID)
	if err != nil || detail == nil || len(detail.Calls) != 1 || len(detail.Calls[0].Rules) != 1 || detail.Calls[0].TLS == nil {
		t.Fatalf("run detail: %+v %v", detail, err)
	}

	wfView, err := read.NewWorkflowQueryRepository(db).FindByID(ctx, userID, w.ID)
	if err != nil || wfView == nil || len(wfView.Calls) != 2 || wfView.Calls[0].RuleCount != 1 || wfView.LastStatus != "failed" {
		t.Fatalf("workflow view: %+v %v", wfView, err)
	}

	aggregates := write.NewAggregateRepository(db)
	from := started.Truncate(time.Hour)
	if _, err := aggregates.ComputeHourly(ctx, from, from.Add(time.Hour)); err != nil {
		t.Fatalf("hourly aggregates: %v", err)
	}
	day := time.Date(started.Year(), started.Month(), started.Day(), 0, 0, 0, 0, time.UTC)
	if _, err := aggregates.ComputeDaily(ctx, day, day.Add(24*time.Hour)); err != nil {
		t.Fatalf("daily aggregates: %v", err)
	}
	var agg write.HourlyAggregateModel
	if err := db.Where("workflow_id = ?", w.ID).First(&agg).Error; err != nil || agg.RunsTotal != 1 || agg.RunsSuccess != 0 {
		t.Fatalf("hourly aggregate: %+v %v", agg, err)
	}
	t.Cleanup(func() {
		db.Exec("DELETE FROM hourly_aggregates WHERE workflow_id = ?", w.ID)
		db.Exec("DELETE FROM daily_aggregates WHERE workflow_id = ?", w.ID)
	})

	partitions := write.NewPartitionMaintainer(db)
	if err := partitions.EnsurePartitions(ctx, 2); err != nil {
		t.Fatalf("ensure partitions: %v", err)
	}
	dropped, err := partitions.DropPartitionsBefore(ctx, time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil || len(dropped) != 0 {
		t.Fatalf("drop partitions: %v %v", dropped, err)
	}

	if err := workflows.Delete(ctx, w.ID); err != nil {
		t.Fatal(err)
	}
}

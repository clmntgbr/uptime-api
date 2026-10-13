//go:build integration

// Runs against a migrated Postgres: TEST_DATABASE_URL=postgres://... go test -tags integration ./internal/infrastructure/persistence/
package persistence_test

import (
	"context"
	"os"
	"testing"
	"time"

	"go-api/internal/contract"
	domaincall "go-api/internal/domain/call"
	"go-api/internal/domain/certificate"
	"go-api/internal/domain/paginate"
	domainrule "go-api/internal/domain/rule"
	domainrun "go-api/internal/domain/run"
	domainschedule "go-api/internal/domain/schedule"
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

	rule, _ := domainrule.Parse("response time exceeds 800 ms")
	def := domainworkflow.Definition{
		Name: "Checkout", Interval: 5 * time.Minute, Enabled: true,
		Statics: []domainworkflow.StaticVariable{{Name: "base_url", Value: "https://api.example.com"}},
		Calls: []domaincall.Definition{
			{
				Name: "login", Method: "POST", URL: "{{base_url}}/login", Body: `{"p":"{{password}}"}`,
				Extractions: []domaincall.Extraction{{Name: "token", Source: domaincall.ExtractJSON, Expression: "$.token"}},
				Rules:       []domainrule.Rule{rule},
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
	states := write.NewStateRepository(db)
	if err := workflows.Save(ctx, w); err != nil {
		t.Fatal(err)
	}
	if err := states.Save(ctx, domainschedule.New(w.ID, past, w.Interval)); err != nil {
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
	if err := workflows.Update(ctx, loaded); err != nil {
		t.Fatalf("update: %v", err)
	}

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
		db.Exec("DELETE FROM call_runs WHERE run_id = ?", runID)
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
	if err != nil || wfView == nil || wfView.Name != def.Name || wfView.Status == "" || wfView.LastStatus != "failed" || wfView.LastRunAt == nil {
		t.Fatalf("workflow view: %+v %v", wfView, err)
	}

	listQuery := paginate.PaginateQuery{Page: 1, Limit: 10, SortBy: "name", OrderBy: paginate.OrderByDesc, Search: def.Name[:3]}
	list, total, err := read.NewWorkflowQueryRepository(db).List(ctx, userID, listQuery)
	if err != nil || total != 1 || len(list) != 1 || list[0].ID != w.ID || list[0].Name != def.Name {
		t.Fatalf("workflow list: %+v total=%d %v", list, total, err)
	}
	listQuery.Search = "100%_match"
	if list, total, err = read.NewWorkflowQueryRepository(db).List(ctx, userID, listQuery); err != nil || total != 0 || len(list) != 0 {
		t.Fatalf("search must treat %% and _ literally: %+v total=%d %v", list, total, err)
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

	retention := write.NewRunRetention(db)
	if deleted, err := retention.DeleteRunsBefore(ctx, time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)); err != nil || deleted != 0 {
		t.Fatalf("retention must keep recent runs: %d %v", deleted, err)
	}
	if err := db.Exec("DELETE FROM runs WHERE id = ?", runID).Error; err != nil {
		t.Fatal(err)
	}
	var orphans int64
	db.Raw("SELECT (SELECT COUNT(*) FROM call_runs WHERE run_id = @id) + (SELECT COUNT(*) FROM rule_results WHERE run_id = @id)",
		map[string]any{"id": runID}).Scan(&orphans)
	if orphans != 0 {
		t.Fatalf("deleting a run must delete its call runs and rule results, %d left", orphans)
	}

	db.Exec("UPDATE workflow_state SET next_run_at = ? WHERE workflow_id = ?", past, w.ID)
	if err := workflows.Delete(ctx, w.ID); err != nil {
		t.Fatal(err)
	}
	var kept write.WorkflowModel
	if err := db.First(&kept, "id = ?", w.ID).Error; err != nil || kept.DeletedAt == nil {
		t.Fatalf("delete must be soft: %+v %v", kept, err)
	}
	var keptCalls int64
	db.Model(&write.CallModel{}).Where("workflow_id = ?", w.ID).Count(&keptCalls)
	if keptCalls != 2 {
		t.Fatalf("calls of a deleted workflow must be kept, got %d", keptCalls)
	}
	if deleted, err := workflows.GetByID(ctx, w.ID); err != nil || deleted != nil {
		t.Fatalf("a deleted workflow must not load: %+v %v", deleted, err)
	}
	if owned, err := workflows.ListByUser(ctx, userID); err != nil || len(owned) != 0 {
		t.Fatalf("a deleted workflow must not be listed for the user: %+v %v", owned, err)
	}
	if view, err := read.NewWorkflowQueryRepository(db).FindByID(ctx, userID, w.ID); err != nil || view != nil {
		t.Fatalf("a deleted workflow must not have a detail: %+v %v", view, err)
	}
	listQuery.Search = ""
	if list, total, err = read.NewWorkflowQueryRepository(db).List(ctx, userID, listQuery); err != nil || total != 0 || len(list) != 0 {
		t.Fatalf("a deleted workflow must not be listed: %+v total=%d %v", list, total, err)
	}
	err = states.WithTransaction(ctx, func(txCtx context.Context) error {
		due, err := states.LockDue(txCtx, time.Now().UTC(), 1000)
		for _, d := range due {
			if d.State.WorkflowID == w.ID {
				t.Error("a deleted workflow must not be scheduled")
			}
		}
		return err
	})
	if err != nil {
		t.Fatalf("lock due after delete: %v", err)
	}
}

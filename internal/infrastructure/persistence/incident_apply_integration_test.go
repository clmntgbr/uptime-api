//go:build integration

package persistence_test

import (
	"context"
	"testing"
	"time"

	cmdincident "go-api/internal/application/command/incident"
	domaincall "go-api/internal/domain/call"
	domainincident "go-api/internal/domain/incident"
	domainschedule "go-api/internal/domain/schedule"
	domainworkflow "go-api/internal/domain/workflow"
	"go-api/internal/infrastructure/crypto"
	"go-api/internal/infrastructure/persistence/outbox"
	"go-api/internal/infrastructure/persistence/write"

	"github.com/google/uuid"
)

func TestApplySignals_PostgresIdempotenceAndReopen(t *testing.T) {
	db := openDB(t)
	ctx := context.Background()
	userID := createUser(t, db)

	key, _ := crypto.GenerateSecretKey()
	cipher, err := crypto.NewAESCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	workflows := write.NewWorkflowWriteRepository(db, cipher)
	states := write.NewStateRepository(db)
	def := domainworkflow.Definition{
		Name: "incidents-it", Interval: time.Minute, Enabled: false,
		Calls: []domaincall.Definition{
			{Name: "a", Method: "GET", URL: "https://example.com"},
		},
	}
	w, err := domainworkflow.New(userID, def, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := workflows.Save(ctx, w); err != nil {
		t.Fatal(err)
	}
	if err := states.Save(ctx, domainschedule.New(w.ID, time.Now().Add(-time.Hour))); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = db.Exec("DELETE FROM incident_events WHERE incident_id IN (SELECT id FROM incidents WHERE workflow_id = ?)", w.ID).Error
		_ = db.Exec("DELETE FROM incidents WHERE workflow_id = ?", w.ID).Error
		_ = db.Exec("DELETE FROM signal_state WHERE workflow_id = ?", w.ID).Error
		_ = db.Exec("DELETE FROM incident_policies WHERE workflow_id = ?", w.ID).Error
		_ = db.Exec("DELETE FROM workflow_state WHERE workflow_id = ?", w.ID).Error
		_ = workflows.Delete(ctx, w.ID)
	})

	apply := cmdincident.NewApplySignalsHandler(
		write.NewIncidentPolicyRepository(db),
		write.NewSignalStateRepository(db),
		write.NewIncidentRepository(db),
		write.NewIncidentEventRepository(db),
		outbox.NewRepository(db),
	)
	callID := w.Calls[0].ID
	subject := domainincident.CallSubject(callID)
	fp := domainincident.Fingerprint(w.ID, domainincident.KindCallFailed, subject)

	base := time.Now().UTC().Add(-time.Hour)
	var lastRun uuid.UUID
	err = states.WithTransaction(ctx, func(txCtx context.Context) error {
		for i := 0; i < 3; i++ {
			runID := uuid.New()
			lastRun = runID
			if _, err := apply.Handle(txCtx, cmdincident.ApplySignalsCommand{
				UserID: userID, WorkflowID: w.ID,
				Signals: []domainincident.Signal{{
					Kind: domainincident.KindCallFailed, Failing: true,
					CallID: &callID, Subject: subject, RunID: &runID,
					ObservedAt: base.Add(time.Duration(i) * time.Minute),
				}},
			}); err != nil {
				return err
			}
		}
		if _, err := apply.Handle(txCtx, cmdincident.ApplySignalsCommand{
			UserID: userID, WorkflowID: w.ID,
			Signals: []domainincident.Signal{{
				Kind: domainincident.KindCallFailed, Failing: true,
				CallID: &callID, Subject: subject, RunID: &lastRun,
				ObservedAt: base.Add(2 * time.Minute),
			}},
		}); err != nil {
			return err
		}
		older := uuid.New()
		if _, err := apply.Handle(txCtx, cmdincident.ApplySignalsCommand{
			UserID: userID, WorkflowID: w.ID,
			Signals: []domainincident.Signal{{
				Kind: domainincident.KindCallFailed, Failing: true,
				CallID: &callID, Subject: subject, RunID: &older,
				ObservedAt: base,
			}},
		}); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	incRepo := write.NewIncidentRepository(db)
	active, err := incRepo.FindActive(ctx, w.ID, fp)
	if err != nil || active == nil {
		t.Fatalf("expected active incident: %v %#v", err, active)
	}
	openedID := active.ID

	err = states.WithTransaction(ctx, func(txCtx context.Context) error {
		for i := 0; i < 2; i++ {
			runID := uuid.New()
			if _, err := apply.Handle(txCtx, cmdincident.ApplySignalsCommand{
				UserID: userID, WorkflowID: w.ID,
				Signals: []domainincident.Signal{{
					Kind: domainincident.KindCallFailed, Failing: false,
					CallID: &callID, Subject: subject, RunID: &runID,
					ObservedAt: base.Add(time.Duration(10+i) * time.Minute),
				}},
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	active, _ = incRepo.FindActive(ctx, w.ID, fp)
	if active != nil {
		t.Fatal("expected resolved")
	}

	err = states.WithTransaction(ctx, func(txCtx context.Context) error {
		runID := uuid.New()
		_, err := apply.Handle(txCtx, cmdincident.ApplySignalsCommand{
			UserID: userID, WorkflowID: w.ID,
			Signals: []domainincident.Signal{{
				Kind: domainincident.KindCallFailed, Failing: true,
				CallID: &callID, Subject: subject, RunID: &runID,
				ObservedAt: time.Now().UTC(),
			}},
		})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := incRepo.FindActive(ctx, w.ID, fp)
	if err != nil || reopened == nil {
		t.Fatal("expected reopened active incident")
	}
	if reopened.ID != openedID {
		t.Fatalf("reopen must keep same id: %s vs %s", reopened.ID, openedID)
	}
	if reopened.ReminderCount != 0 || reopened.ReopenCount != 1 {
		t.Fatalf("unexpected reopen state %+v", reopened)
	}
}

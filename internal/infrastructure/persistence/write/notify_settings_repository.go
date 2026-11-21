package write

import (
	"context"
	"errors"
	"time"

	"go-api/internal/domain/notifysettings"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type notifySettingsRepository struct{ db *gorm.DB }

func NewNotifySettingsRepository(db *gorm.DB) notifysettings.Repository {
	return &notifySettingsRepository{db: db}
}

func (r *notifySettingsRepository) Get(ctx context.Context, workflowID uuid.UUID) (*notifysettings.Settings, error) {
	var m WorkflowNotificationSettingsModel
	err := DBWithContext(ctx, r.db).Where("workflow_id = ?", workflowID).Take(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return notifySettingsFromModel(m), nil
}

func (r *notifySettingsRepository) Upsert(ctx context.Context, s notifysettings.Settings) error {
	mode := string(s.RunMode)
	if mode == "" {
		mode = string(notifysettings.RunModeTransition)
	}
	m := WorkflowNotificationSettingsModel{
		WorkflowID: s.WorkflowID, UserID: s.UserID,
		Emails:           pq.StringArray(s.Emails),
		IncidentsEnabled: s.IncidentsEnabled, TLSEnabled: s.TLSEnabled,
		RunsEnabled: s.RunsEnabled, RunStatuses: pq.StringArray(s.RunStatuses),
		RunMode: mode, UpdatedAt: s.UpdatedAt,
	}
	if m.UpdatedAt.IsZero() {
		m.UpdatedAt = time.Now().UTC()
	}
	return DBWithContext(ctx, r.db).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "workflow_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"emails", "incidents_enabled", "tls_enabled",
			"runs_enabled", "run_statuses", "run_mode", "updated_at",
		}),
	}).Create(&m).Error
}

func notifySettingsFromModel(m WorkflowNotificationSettingsModel) *notifysettings.Settings {
	emails := []string(m.Emails)
	if emails == nil {
		emails = []string{}
	}
	statuses := notifysettings.NormalizeRunStatuses([]string(m.RunStatuses))
	mode, _ := notifysettings.ParseRunMode(m.RunMode)
	return &notifysettings.Settings{
		WorkflowID: m.WorkflowID, UserID: m.UserID, Emails: emails,
		IncidentsEnabled: m.IncidentsEnabled, TLSEnabled: m.TLSEnabled,
		RunsEnabled: m.RunsEnabled, RunStatuses: statuses, RunMode: mode,
		UpdatedAt: m.UpdatedAt,
	}
}

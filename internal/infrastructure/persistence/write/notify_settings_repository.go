package write

import (
	"context"
	"errors"
	"fmt"
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
		RunMode:     mode,
		DigestDaily: s.DigestDaily, DigestWeekly: s.DigestWeekly, DigestMonthly: s.DigestMonthly,
		UpdatedAt: s.UpdatedAt,
	}
	if m.UpdatedAt.IsZero() {
		m.UpdatedAt = time.Now().UTC()
	}
	return DBWithContext(ctx, r.db).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "workflow_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"emails", "incidents_enabled", "tls_enabled",
			"runs_enabled", "run_statuses", "run_mode",
			"digest_daily", "digest_weekly", "digest_monthly", "updated_at",
		}),
	}).Create(&m).Error
}

func (r *notifySettingsRepository) ListDigestCandidates(
	ctx context.Context, cadence notifysettings.DigestCadence,
) ([]notifysettings.DigestCandidate, error) {
	col, err := digestColumn(cadence)
	if err != nil {
		return nil, err
	}
	type row struct {
		WorkflowNotificationSettingsModel
		WorkflowName string `gorm:"column:workflow_name"`
	}
	var rows []row
	err = DBWithContext(ctx, r.db).Raw(`
SELECT s.*, w.name AS workflow_name
FROM workflow_notification_settings s
JOIN workflows w ON w.id = s.workflow_id AND w.deleted_at IS NULL
WHERE s.`+col+` = TRUE
  AND cardinality(s.emails) > 0
`, nil).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]notifysettings.DigestCandidate, 0, len(rows))
	for _, row := range rows {
		s := notifySettingsFromModel(row.WorkflowNotificationSettingsModel)
		out = append(out, notifysettings.DigestCandidate{
			Settings: *s, WorkflowName: row.WorkflowName,
		})
	}
	return out, nil
}

func digestColumn(c notifysettings.DigestCadence) (string, error) {
	switch c {
	case notifysettings.DigestDaily:
		return "digest_daily", nil
	case notifysettings.DigestWeekly:
		return "digest_weekly", nil
	case notifysettings.DigestMonthly:
		return "digest_monthly", nil
	default:
		return "", fmt.Errorf("unknown digest cadence %q", c)
	}
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
		DigestDaily: m.DigestDaily, DigestWeekly: m.DigestWeekly, DigestMonthly: m.DigestMonthly,
		UpdatedAt: m.UpdatedAt,
	}
}

type digestStatsRepository struct{ db *gorm.DB }

func NewDigestStatsRepository(db *gorm.DB) notifysettings.DigestStatsRepository {
	return &digestStatsRepository{db: db}
}

func (r *digestStatsRepository) Stats(
	ctx context.Context, workflowID uuid.UUID, from, to time.Time,
) (notifysettings.DigestStats, error) {
	var runRow struct {
		Success int64
		Failed  int64
		Error   int64
	}
	err := DBWithContext(ctx, r.db).Raw(`
SELECT
  COUNT(*) FILTER (WHERE status = 'success') AS success,
  COUNT(*) FILTER (WHERE status = 'failed') AS failed,
  COUNT(*) FILTER (WHERE status = 'error') AS error
FROM runs
WHERE workflow_id = @workflow_id
  AND finished_at >= @from
  AND finished_at < @to
  AND status IN ('success', 'failed', 'error')
`, map[string]any{
		"workflow_id": workflowID, "from": from, "to": to,
	}).Scan(&runRow).Error
	if err != nil {
		return notifysettings.DigestStats{}, err
	}

	var opened, resolved int64
	err = DBWithContext(ctx, r.db).Raw(`
SELECT COUNT(*)
FROM incident_events e
JOIN incidents i ON i.id = e.incident_id
WHERE i.workflow_id = @workflow_id
  AND e.type = 'opened'
  AND e.at >= @from AND e.at < @to
`, map[string]any{
		"workflow_id": workflowID, "from": from, "to": to,
	}).Scan(&opened).Error
	if err != nil {
		return notifysettings.DigestStats{}, err
	}
	err = DBWithContext(ctx, r.db).Raw(`
SELECT COUNT(*)
FROM incident_events e
JOIN incidents i ON i.id = e.incident_id
WHERE i.workflow_id = @workflow_id
  AND e.type = 'resolved'
  AND e.at >= @from AND e.at < @to
`, map[string]any{
		"workflow_id": workflowID, "from": from, "to": to,
	}).Scan(&resolved).Error
	if err != nil {
		return notifysettings.DigestStats{}, err
	}

	return notifysettings.DigestStats{
		RunsTotal:         runRow.Success + runRow.Failed + runRow.Error,
		RunsSuccess:       runRow.Success,
		RunsFailed:        runRow.Failed,
		RunsError:         runRow.Error,
		IncidentsOpened:   opened,
		IncidentsResolved: resolved,
	}, nil
}

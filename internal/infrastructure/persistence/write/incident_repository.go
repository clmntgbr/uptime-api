package write

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"go-api/internal/domain/incident"
	"go-api/internal/domain/paginate"
	"go-api/internal/infrastructure/persistence/dbtype"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type incidentPolicyRepository struct{ db *gorm.DB }
type signalStateRepository struct{ db *gorm.DB }
type incidentRepository struct{ db *gorm.DB }
type incidentEventRepository struct{ db *gorm.DB }

func NewIncidentPolicyRepository(db *gorm.DB) incident.PolicyRepository {
	return &incidentPolicyRepository{db: db}
}

func NewSignalStateRepository(db *gorm.DB) incident.SignalStateRepository {
	return &signalStateRepository{db: db}
}

func NewIncidentRepository(db *gorm.DB) incident.IncidentRepository {
	return &incidentRepository{db: db}
}

func NewIncidentEventRepository(db *gorm.DB) incident.EventRepository {
	return &incidentEventRepository{db: db}
}

func (r *incidentPolicyRepository) Get(ctx context.Context, workflowID uuid.UUID, kind incident.Kind) (*incident.Policy, error) {
	var m IncidentPolicyModel
	err := DBWithContext(ctx, r.db).
		Where("workflow_id = ? AND kind = ?", workflowID, string(kind)).
		Take(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return incidentPolicyFromModel(m)
}

func (r *incidentPolicyRepository) ListByWorkflow(ctx context.Context, userID, workflowID uuid.UUID) ([]incident.Policy, error) {
	var models []IncidentPolicyModel
	err := DBWithContext(ctx, r.db).
		Where("user_id = ? AND workflow_id = ?", userID, workflowID).
		Find(&models).Error
	if err != nil {
		return nil, err
	}
	out := make([]incident.Policy, 0, len(models))
	for _, m := range models {
		p, err := incidentPolicyFromModel(m)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, nil
}

func (r *incidentPolicyRepository) Upsert(ctx context.Context, p incident.Policy) error {
	params := p.Params
	if params == nil {
		params = map[string]any{}
	}
	raw, err := json.Marshal(params)
	if err != nil {
		return err
	}
	m := IncidentPolicyModel{
		ID: p.ID, UserID: p.UserID, WorkflowID: p.WorkflowID,
		Kind: string(p.Kind), Enabled: p.Enabled, Severity: string(p.Severity),
		OpenAfter: p.OpenAfter, CloseAfter: p.CloseAfter,
		Params: dbtype.JSONB(raw), UpdatedAt: p.UpdatedAt,
	}
	return DBWithContext(ctx, r.db).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "workflow_id"}, {Name: "kind"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"enabled", "severity", "open_after", "close_after", "params", "updated_at",
		}),
	}).Create(&m).Error
}

func (r *signalStateRepository) LockGetOrCreate(ctx context.Context, workflowID uuid.UUID, fingerprint string) (incident.SignalState, error) {
	db := DBWithContext(ctx, r.db)
	empty := SignalStateModel{
		WorkflowID: workflowID, Fingerprint: fingerprint, UpdatedAt: time.Now().UTC(),
	}
	if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&empty).Error; err != nil {
		return incident.SignalState{}, err
	}
	var m SignalStateModel
	err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("workflow_id = ? AND fingerprint = ?", workflowID, fingerprint).
		Take(&m).Error
	if err != nil {
		return incident.SignalState{}, err
	}
	return signalStateFromModel(m), nil
}

func (r *signalStateRepository) Save(ctx context.Context, s incident.SignalState) error {
	m := SignalStateModel{
		WorkflowID: s.WorkflowID, Fingerprint: s.Fingerprint,
		ConsecutiveFailures: s.ConsecutiveFailures, ConsecutiveSuccesses: s.ConsecutiveSuccesses,
		LastRunID: s.LastRunID, LastSignalAt: s.LastSignalAt, UpdatedAt: s.UpdatedAt,
	}
	return DBWithContext(ctx, r.db).Save(&m).Error
}

func (r *incidentRepository) FindActive(ctx context.Context, workflowID uuid.UUID, fingerprint string) (*incident.Incident, error) {
	var m IncidentModel
	err := DBWithContext(ctx, r.db).
		Where("workflow_id = ? AND fingerprint = ? AND state <> ?", workflowID, fingerprint, string(incident.StateResolved)).
		Take(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return incidentFromModel(m)
}

func (r *incidentRepository) FindLatestResolved(ctx context.Context, workflowID uuid.UUID, fingerprint string) (*incident.Incident, error) {
	var m IncidentModel
	err := DBWithContext(ctx, r.db).
		Where("workflow_id = ? AND fingerprint = ? AND state = ?", workflowID, fingerprint, string(incident.StateResolved)).
		Order("resolved_at DESC").
		Take(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return incidentFromModel(m)
}

func (r *incidentRepository) Insert(ctx context.Context, inc incident.Incident) error {
	m, err := incidentToModel(inc)
	if err != nil {
		return err
	}
	return DBWithContext(ctx, r.db).Create(&m).Error
}

func (r *incidentRepository) Update(ctx context.Context, inc incident.Incident) error {
	m, err := incidentToModel(inc)
	if err != nil {
		return err
	}
	return DBWithContext(ctx, r.db).Save(&m).Error
}

func (r *incidentRepository) GetByID(ctx context.Context, userID, id uuid.UUID) (*incident.Incident, error) {
	var m IncidentModel
	err := DBWithContext(ctx, r.db).
		Where("id = ? AND user_id = ?", id, userID).
		Take(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return incidentFromModel(m)
}

func (r *incidentRepository) GetByIDForUpdate(ctx context.Context, userID, id uuid.UUID) (*incident.Incident, error) {
	var m IncidentModel
	err := DBWithContext(ctx, r.db).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND user_id = ?", id, userID).
		Take(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return incidentFromModel(m)
}

func (r *incidentRepository) ClaimReminder(ctx context.Context, id uuid.UUID, expectedCount int, now time.Time) (bool, error) {
	res := DBWithContext(ctx, r.db).Exec(`
UPDATE incidents
SET reminder_count = reminder_count + 1, last_reminder_at = @now
WHERE id = @id AND state = @state AND reminder_count = @expected
`, map[string]any{
		"id": id, "state": string(incident.StateOpen),
		"expected": expectedCount, "now": now.UTC(),
	})
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected == 1, nil
}

func (r *incidentRepository) ListByWorkflow(
	ctx context.Context,
	userID, workflowID uuid.UUID,
	states []incident.State,
	severities []incident.Severity,
	q paginate.PaginateQuery,
) ([]incident.Incident, int64, error) {
	db := DBWithContext(ctx, r.db).Model(&IncidentModel{}).
		Where("user_id = ? AND workflow_id = ?", userID, workflowID)
	if len(states) > 0 {
		raw := make([]string, len(states))
		for i, s := range states {
			raw[i] = string(s)
		}
		db = db.Where("state IN ?", raw)
	}
	if len(severities) > 0 {
		raw := make([]string, len(severities))
		for i, s := range severities {
			raw[i] = string(s)
		}
		db = db.Where("severity IN ?", raw)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var models []IncidentModel
	err := db.
		Order("CASE state WHEN 'open' THEN 0 WHEN 'acknowledged' THEN 1 ELSE 2 END").
		Order("CASE severity WHEN 'critical' THEN 0 WHEN 'warning' THEN 1 ELSE 2 END").
		Order("opened_at DESC").
		Offset(q.Offset()).Limit(q.Limit).
		Find(&models).Error
	if err != nil {
		return nil, 0, err
	}
	out := make([]incident.Incident, 0, len(models))
	for _, m := range models {
		inc, err := incidentFromModel(m)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *inc)
	}
	return out, total, nil
}

func (r *incidentRepository) ListOpenForReminders(ctx context.Context, now time.Time) ([]incident.Incident, error) {
	firstCutoff := now.Add(-incident.ReminderAfter)
	secondCutoff := now.Add(-incident.ReminderSecondAfter)
	var models []IncidentModel
	err := DBWithContext(ctx, r.db).
		Where("state = ?", string(incident.StateOpen)).
		Where(`(
			(reminder_count = 0 AND opened_at <= ?) OR
			(reminder_count = 1 AND opened_at <= ?)
		)`, firstCutoff, secondCutoff).
		Find(&models).Error
	if err != nil {
		return nil, err
	}
	out := make([]incident.Incident, 0, len(models))
	for _, m := range models {
		inc, err := incidentFromModel(m)
		if err != nil {
			return nil, err
		}
		out = append(out, *inc)
	}
	return out, nil
}

func (r *incidentEventRepository) Append(ctx context.Context, e incident.Event) error {
	payload := e.Payload
	if payload == nil {
		payload = map[string]any{}
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	m := IncidentEventModel{
		ID: e.ID, IncidentID: e.IncidentID, At: e.At,
		Type: string(e.Type), RunID: e.RunID, Payload: dbtype.JSONB(raw),
	}
	return DBWithContext(ctx, r.db).Create(&m).Error
}

func (r *incidentEventRepository) ListByIncident(ctx context.Context, incidentID uuid.UUID) ([]incident.Event, error) {
	var models []IncidentEventModel
	err := DBWithContext(ctx, r.db).
		Where("incident_id = ?", incidentID).
		Order("at ASC").
		Find(&models).Error
	if err != nil {
		return nil, err
	}
	out := make([]incident.Event, 0, len(models))
	for _, m := range models {
		e, err := incidentEventFromModel(m)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}

func incidentPolicyFromModel(m IncidentPolicyModel) (*incident.Policy, error) {
	params := map[string]any{}
	if len(m.Params) > 0 {
		if err := json.Unmarshal(m.Params, &params); err != nil {
			return nil, err
		}
	}
	return &incident.Policy{
		ID: m.ID, UserID: m.UserID, WorkflowID: m.WorkflowID,
		Kind: incident.Kind(m.Kind), Enabled: m.Enabled,
		Severity: incident.Severity(m.Severity),
		OpenAfter: m.OpenAfter, CloseAfter: m.CloseAfter,
		Params: params, UpdatedAt: m.UpdatedAt,
	}, nil
}

func signalStateFromModel(m SignalStateModel) incident.SignalState {
	return incident.SignalState{
		WorkflowID: m.WorkflowID, Fingerprint: m.Fingerprint,
		ConsecutiveFailures: m.ConsecutiveFailures, ConsecutiveSuccesses: m.ConsecutiveSuccesses,
		LastRunID: m.LastRunID, LastSignalAt: m.LastSignalAt, UpdatedAt: m.UpdatedAt,
	}
}

func incidentToModel(inc incident.Incident) (IncidentModel, error) {
	cause := inc.Cause
	if cause == nil {
		cause = map[string]any{}
	}
	raw, err := json.Marshal(cause)
	if err != nil {
		return IncidentModel{}, err
	}
	var ackBy *string
	if inc.AcknowledgedBy != "" {
		v := inc.AcknowledgedBy
		ackBy = &v
	}
	return IncidentModel{
		ID: inc.ID, UserID: inc.UserID, WorkflowID: inc.WorkflowID,
		Kind: string(inc.Kind), CallID: inc.CallID, Fingerprint: inc.Fingerprint,
		Severity: string(inc.Severity), State: string(inc.State),
		Cause: dbtype.JSONB(raw), OpenedAt: inc.OpenedAt,
		AcknowledgedAt: inc.AcknowledgedAt, AcknowledgedBy: ackBy,
		ResolvedAt: inc.ResolvedAt, ReopenCount: inc.ReopenCount,
		OpenedByRunID: inc.OpenedByRunID, LastFailureRunID: inc.LastFailureRunID,
		LastReminderAt: inc.LastReminderAt, ReminderCount: inc.ReminderCount,
	}, nil
}

func incidentFromModel(m IncidentModel) (*incident.Incident, error) {
	cause := map[string]any{}
	if len(m.Cause) > 0 {
		if err := json.Unmarshal(m.Cause, &cause); err != nil {
			return nil, err
		}
	}
	ackBy := ""
	if m.AcknowledgedBy != nil {
		ackBy = *m.AcknowledgedBy
	}
	return &incident.Incident{
		ID: m.ID, UserID: m.UserID, WorkflowID: m.WorkflowID,
		Kind: incident.Kind(m.Kind), CallID: m.CallID, Fingerprint: m.Fingerprint,
		Severity: incident.Severity(m.Severity), State: incident.State(m.State),
		Cause: cause, OpenedAt: m.OpenedAt,
		AcknowledgedAt: m.AcknowledgedAt, AcknowledgedBy: ackBy,
		ResolvedAt: m.ResolvedAt, ReopenCount: m.ReopenCount,
		OpenedByRunID: m.OpenedByRunID, LastFailureRunID: m.LastFailureRunID,
		LastReminderAt: m.LastReminderAt, ReminderCount: m.ReminderCount,
	}, nil
}

func incidentEventFromModel(m IncidentEventModel) (incident.Event, error) {
	payload := map[string]any{}
	if len(m.Payload) > 0 {
		if err := json.Unmarshal(m.Payload, &payload); err != nil {
			return incident.Event{}, err
		}
	}
	return incident.Event{
		ID: m.ID, IncidentID: m.IncidentID, At: m.At,
		Type: incident.EventType(m.Type), RunID: m.RunID, Payload: payload,
	}, nil
}

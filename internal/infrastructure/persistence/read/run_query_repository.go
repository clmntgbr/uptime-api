package read

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"go-api/internal/contract"
	"go-api/internal/domain/paginate"
	domainrun "go-api/internal/domain/run"
	"go-api/internal/infrastructure/persistence/dbtype"
	"go-api/internal/infrastructure/persistence/write"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type runQueryRepository struct {
	db *gorm.DB
}

func NewRunQueryRepository(db *gorm.DB) domainrun.ReadRepository {
	return &runQueryRepository{db: db}
}

var runSortColumns = map[string]string{
	"startedAt":  "started_at",
	"started_at": "started_at",
	"duration":   "duration_ms",
	"status":     "status",
}

func (r *runQueryRepository) ListByWorkflow(ctx context.Context, userID, workflowID uuid.UUID, q paginate.PaginateQuery) ([]domainrun.View, int64, error) {
	column, ok := runSortColumns[q.SortBy]
	if !ok {
		column = "started_at"
	}
	q.SortBy = column

	base := r.db.WithContext(ctx).
		Model(&write.RunModel{}).
		Where("user_id = ? AND workflow_id = ?", userID, workflowID)
	if q.Search != "" {
		base = base.Where("status = ?", q.Search)
	}

	scoped, total, err := Paginate(base, q)
	if err != nil {
		return nil, 0, err
	}
	var models []write.RunModel
	if err := scoped.Find(&models).Error; err != nil {
		return nil, 0, err
	}
	views := make([]domainrun.View, 0, len(models))
	for _, m := range models {
		views = append(views, runView(m))
	}
	return views, total, nil
}

func (r *runQueryRepository) FindByID(ctx context.Context, userID, runID uuid.UUID) (*domainrun.DetailView, error) {
	var model write.RunModel
	err := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", runID, userID).
		Take(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	var calls []write.CallRunModel
	err = r.db.WithContext(ctx).
		Where("run_id = ?", model.ID).
		Order("position").
		Find(&calls).Error
	if err != nil {
		return nil, err
	}
	var rules []write.RuleRunModel
	err = r.db.WithContext(ctx).
		Where("run_id = ?", model.ID).
		Order("call_run_id, position").
		Find(&rules).Error
	if err != nil {
		return nil, err
	}
	rulesByCall := map[uuid.UUID][]domainrun.RuleRun{}
	for _, rr := range rules {
		rulesByCall[rr.CallRunID] = append(rulesByCall[rr.CallRunID], domainrun.RuleRun{
			ID: rr.ID, Position: rr.Position, Subject: rr.Subject, Operator: rr.Operator,
			Text: rr.Text, Observed: rr.Observed, Expected: rr.Expected, Passed: rr.Passed,
		})
	}

	detail := &domainrun.DetailView{
		View:      runView(model),
		Variables: map[string]string{},
		Calls:     make([]domainrun.CallRunView, 0, len(calls)),
	}
	if err := unmarshalOptional(model.Variables, &detail.Variables); err != nil {
		return nil, err
	}
	for _, c := range calls {
		view, err := callRunView(c, rulesByCall[c.ID])
		if err != nil {
			return nil, err
		}
		detail.Calls = append(detail.Calls, view)
	}
	return detail, nil
}

func (r *runQueryRepository) FindLatestTerminalByWorkflow(ctx context.Context, userID, workflowID uuid.UUID) (*domainrun.LatestView, error) {
	var model write.RunModel
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND workflow_id = ?", userID, workflowID).
		Where("status IN ?", []string{
			string(domainrun.StatusSuccess),
			string(domainrun.StatusFailed),
			string(domainrun.StatusError),
			string(domainrun.StatusSkipped),
		}).
		Order("started_at DESC").
		Limit(1).
		Take(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	type leanCall struct {
		CallID     *uuid.UUID `gorm:"column:call_id"`
		Name       string     `gorm:"column:name"`
		Position   int        `gorm:"column:position"`
		Status     string     `gorm:"column:status"`
		HTTPStatus *int       `gorm:"column:http_status"`
		StartedAt  *time.Time `gorm:"column:started_at"`
		FinishedAt *time.Time `gorm:"column:finished_at"`
	}
	var calls []leanCall
	err = r.db.WithContext(ctx).
		Table("call_runs").
		Select("call_id, name, position, status, http_status, started_at, finished_at").
		Where("run_id = ?", model.ID).
		Order("position").
		Find(&calls).Error
	if err != nil {
		return nil, err
	}

	out := &domainrun.LatestView{
		ID:         model.ID,
		Status:     domainrun.Status(model.Status),
		StartedAt:  model.StartedAt,
		DurationMS: model.DurationMS,
		Calls:      make([]domainrun.LatestCallView, 0, len(calls)),
	}
	for _, c := range calls {
		view := domainrun.LatestCallView{
			CallID:   c.CallID,
			Name:     c.Name,
			Position: c.Position,
			Status:   domainrun.CallStatus(c.Status),
		}
		if c.StartedAt != nil {
			view.OffsetMS = c.StartedAt.Sub(model.StartedAt).Milliseconds()
			if c.FinishedAt != nil {
				view.DurationMS = c.FinishedAt.Sub(*c.StartedAt).Milliseconds()
			}
		}
		if c.Status == string(domainrun.CallSkipped) {
			view.DurationMS = 0
		} else if c.HTTPStatus != nil {
			view.HTTPStatus = c.HTTPStatus
		}
		out.Calls = append(out.Calls, view)
	}
	return out, nil
}

func (r *runQueryRepository) FindLatestExtractedByWorkflow(ctx context.Context, userID, workflowID uuid.UUID) (map[uuid.UUID]map[string]string, error) {
	var runID uuid.UUID
	err := r.db.WithContext(ctx).
		Model(&write.RunModel{}).
		Select("id").
		Where("user_id = ? AND workflow_id = ?", userID, workflowID).
		Where("status IN ?", []string{
			string(domainrun.StatusSuccess),
			string(domainrun.StatusFailed),
			string(domainrun.StatusError),
			string(domainrun.StatusSkipped),
		}).
		Order("started_at DESC").
		Limit(1).
		Take(&runID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return map[uuid.UUID]map[string]string{}, nil
		}
		return nil, err
	}

	type row struct {
		CallID             *uuid.UUID   `gorm:"column:call_id"`
		ExtractedVariables dbtype.JSONB `gorm:"column:extracted_variables"`
	}
	var rows []row
	err = r.db.WithContext(ctx).
		Table("call_runs").
		Select("call_id, extracted_variables").
		Where("run_id = ?", runID).
		Order("position").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}

	out := make(map[uuid.UUID]map[string]string, len(rows))
	for _, row := range rows {
		if row.CallID == nil {
			continue
		}
		values := map[string]string{}
		if err := unmarshalOptional(row.ExtractedVariables, &values); err != nil {
			return nil, err
		}
		out[*row.CallID] = values
	}
	return out, nil
}

func (r *runQueryRepository) LatestResponseKeysForCall(ctx context.Context, userID, workflowID, callID uuid.UUID) ([]string, error) {
	var model write.CallRunModel
	err := r.db.WithContext(ctx).
		Model(&write.CallRunModel{}).
		Select("call_runs.response_keys").
		Joins("JOIN runs ON runs.id = call_runs.run_id").
		Where("runs.user_id = ? AND call_runs.workflow_id = ? AND call_runs.call_id = ?", userID, workflowID, callID).
		Where("jsonb_array_length(call_runs.response_keys) > 0").
		Order("call_runs.finished_at DESC NULLS LAST").
		Limit(1).
		Take(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return []string{}, nil
		}
		return nil, err
	}
	if len(model.ResponseKeys) == 0 {
		return []string{}, nil
	}
	var keys []string
	if err := json.Unmarshal(model.ResponseKeys, &keys); err != nil {
		return nil, err
	}
	if keys == nil {
		return []string{}, nil
	}
	return keys, nil
}

func runView(m write.RunModel) domainrun.View {
	v := domainrun.View{
		ID:         m.ID,
		WorkflowID: m.WorkflowID,
		Trigger:    m.Trigger,
		Status:     domainrun.Status(m.Status),
		Reason:     deref(m.Reason),
		StartedAt:  m.StartedAt,
		DurationMS: m.DurationMS,
		Counts: domainrun.Counts{
			Success: m.CallsSuccess, Failed: m.CallsFailed, Error: m.CallsError, Skipped: m.CallsSkipped,
		},
	}
	if m.FinishedAt != nil {
		v.FinishedAt = *m.FinishedAt
	}
	return v
}

func callRunView(m write.CallRunModel, rules []domainrun.RuleRun) (domainrun.CallRunView, error) {
	v := domainrun.CallRunView{
		ID:           m.ID,
		CallID:       m.CallID,
		Name:         m.Name,
		Position:     m.Position,
		Status:       domainrun.CallStatus(m.Status),
		Reason:       deref(m.Reason),
		BodyRef:      deref(m.BodyRef),
		BodyHash:     deref(m.BodyHash),
		BodyEncoding: deref(m.BodyEncoding),
		Rules:        rules,
		StartedAt:    m.StartedAt,
		FinishedAt:   m.FinishedAt,
	}
	if v.Rules == nil {
		v.Rules = []domainrun.RuleRun{}
	}
	if len(m.ResponseKeys) > 0 {
		if err := json.Unmarshal(m.ResponseKeys, &v.ResponseKeys); err != nil {
			return domainrun.CallRunView{}, err
		}
	}
	if m.HTTPStatus != nil {
		v.HTTPStatus = *m.HTTPStatus
	}
	if m.ErrorType != nil {
		v.Error = &contract.CallError{Type: *m.ErrorType, Message: deref(m.ErrorMessage)}
	}
	for _, field := range []struct {
		raw []byte
		dst any
	}{
		{m.Request, &v.Request},
		{m.Response, &v.Response},
		{m.Timings, &v.Timings},
		{m.Network, &v.Network},
		{m.ExtractedVariables, &v.ExtractedVariables},
		{m.UsedVariables, &v.UsedVariables},
	} {
		if err := unmarshalOptional(field.raw, field.dst); err != nil {
			return domainrun.CallRunView{}, err
		}
	}
	if len(m.TLS) > 0 {
		var tls contract.TLSInfo
		if err := json.Unmarshal(m.TLS, &tls); err != nil {
			return domainrun.CallRunView{}, err
		}
		v.TLS = &tls
	}
	return v, nil
}

func unmarshalOptional(raw []byte, dst any) error {
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, dst)
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

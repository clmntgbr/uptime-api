package read

import (
	"context"
	"encoding/json"
	"errors"

	"go-api/internal/contract"
	"go-api/internal/domain/paginate"
	domainrun "go-api/internal/domain/run"
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

func runView(m write.RunModel) domainrun.View {
	return domainrun.View{
		ID:         m.ID,
		WorkflowID: m.WorkflowID,
		Trigger:    m.Trigger,
		Status:     domainrun.Status(m.Status),
		Reason:     deref(m.Reason),
		StartedAt:  m.StartedAt,
		FinishedAt: m.FinishedAt,
		DurationMS: m.DurationMS,
		Counts: domainrun.Counts{
			Success: m.CallsSuccess, Failed: m.CallsFailed, Error: m.CallsError, Skipped: m.CallsSkipped,
		},
	}
}

func callRunView(m write.CallRunModel, rules []domainrun.RuleRun) (domainrun.CallRunView, error) {
	v := domainrun.CallRunView{
		ID:         m.ID,
		CallID:     m.CallID,
		Name:       m.Name,
		Position:   m.Position,
		Status:     domainrun.CallStatus(m.Status),
		Reason:     deref(m.Reason),
		Rules:      rules,
		StartedAt:  m.StartedAt,
		FinishedAt: m.FinishedAt,
	}
	if v.Rules == nil {
		v.Rules = []domainrun.RuleRun{}
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

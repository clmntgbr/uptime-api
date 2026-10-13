package write

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	domainrule "go-api/internal/domain/rule"
	"go-api/internal/domain/variable"
	domainworkflow "go-api/internal/domain/workflow"
	"go-api/internal/infrastructure/persistence/dbtype"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type workflowWriteRepository struct {
	db *gorm.DB
}

func NewWorkflowWriteRepository(db *gorm.DB) domainworkflow.WriteRepository {
	return &workflowWriteRepository{db: db}
}

func (r *workflowWriteRepository) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(txCtxKey{}).(*gorm.DB); ok {
		return fn(ctx)
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(ContextWithTx(ctx, tx))
	})
}

func (r *workflowWriteRepository) Save(ctx context.Context, w *domainworkflow.Workflow) error {
	return r.WithTransaction(ctx, func(ctx context.Context) error {
		if err := DBWithContext(ctx, r.db).Create(workflowModelFromDomain(w)).Error; err != nil {
			return err
		}
		return r.insertChildren(ctx, w)
	})
}

func (r *workflowWriteRepository) Update(ctx context.Context, w *domainworkflow.Workflow) error {
	return r.WithTransaction(ctx, func(ctx context.Context) error {
		db := DBWithContext(ctx, r.db)
		model := workflowModelFromDomain(w)
		err := db.Model(&WorkflowModel{}).Where("id = ?", w.ID).Updates(map[string]any{
			"name":             model.Name,
			"interval_seconds": model.IntervalSeconds,
			"timeout_ms":       model.TimeoutMS,
			"enabled":          model.Enabled,
			"updated_at":       model.UpdatedAt,
		}).Error
		if err != nil {
			return err
		}
		if err := db.Where("workflow_id = ?", w.ID).Delete(&CallModel{}).Error; err != nil {
			return err
		}
		if err := db.Where("workflow_id = ? AND type = ?", w.ID, string(variable.TypeStatic)).Delete(&VariableModel{}).Error; err != nil {
			return err
		}
		return r.insertChildren(ctx, w)
	})
}

func (r *workflowWriteRepository) insertChildren(ctx context.Context, w *domainworkflow.Workflow) error {
	db := DBWithContext(ctx, r.db)
	now := time.Now().UTC()

	calls := make([]CallModel, 0, len(w.Calls))
	var rules []RuleModel
	for _, c := range w.Calls {
		model, err := callModelFromDomain(w.ID, c, now)
		if err != nil {
			return err
		}
		calls = append(calls, model)
		for i, rule := range c.Rules {
			rules = append(rules, RuleModel{
				ID:        rule.ID,
				CallID:    c.ID,
				Position:  i + 1,
				Subject:   string(rule.Subject),
				Operator:  string(rule.Operator),
				Target:    nullableString(rule.Target),
				Value:     nullableString(rule.Value),
				CreatedAt: now,
			})
		}
	}
	if err := db.Create(&calls).Error; err != nil {
		return err
	}
	if len(rules) > 0 {
		if err := db.Create(&rules).Error; err != nil {
			return err
		}
	}

	if len(w.Statics) == 0 {
		return nil
	}
	workflowID := w.ID
	statics := make([]VariableModel, 0, len(w.Statics))
	for _, s := range w.Statics {
		statics = append(statics, VariableModel{
			ID:         s.ID,
			UserID:     w.UserID,
			WorkflowID: &workflowID,
			Type:       string(variable.TypeStatic),
			Name:       s.Name,
			Value:      s.Value,
			CreatedAt:  now,
			UpdatedAt:  now,
		})
	}
	return db.Create(&statics).Error
}

func (r *workflowWriteRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return DBWithContext(ctx, r.db).Delete(&WorkflowModel{}, "id = ?", id).Error
}

func (r *workflowWriteRepository) GetByID(ctx context.Context, id uuid.UUID) (*domainworkflow.Workflow, error) {
	var model WorkflowModel
	err := DBWithContext(ctx, r.db).First(&model, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	workflows, err := r.hydrate(ctx, []WorkflowModel{model})
	if err != nil {
		return nil, err
	}
	return workflows[0], nil
}

func (r *workflowWriteRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]*domainworkflow.Workflow, error) {
	var models []WorkflowModel
	if err := DBWithContext(ctx, r.db).Where("user_id = ?", userID).Order("created_at").Find(&models).Error; err != nil {
		return nil, err
	}
	return r.hydrate(ctx, models)
}

// hydrate loads calls, rules and static variables of the given workflows.
func (r *workflowWriteRepository) hydrate(ctx context.Context, models []WorkflowModel) ([]*domainworkflow.Workflow, error) {
	if len(models) == 0 {
		return nil, nil
	}
	db := DBWithContext(ctx, r.db)
	ids := make([]uuid.UUID, 0, len(models))
	for _, m := range models {
		ids = append(ids, m.ID)
	}

	var calls []CallModel
	if err := db.Where("workflow_id IN ?", ids).Order("workflow_id, position").Find(&calls).Error; err != nil {
		return nil, err
	}
	callIDs := make([]uuid.UUID, 0, len(calls))
	for _, c := range calls {
		callIDs = append(callIDs, c.ID)
	}
	rulesByCall := map[uuid.UUID][]domainrule.Rule{}
	if len(callIDs) > 0 {
		var rules []RuleModel
		if err := db.Where("call_id IN ?", callIDs).Order("call_id, position").Find(&rules).Error; err != nil {
			return nil, err
		}
		for _, rule := range rules {
			rulesByCall[rule.CallID] = append(rulesByCall[rule.CallID], domainrule.Rule{
				ID:       rule.ID,
				Subject:  domainrule.Subject(rule.Subject),
				Operator: domainrule.Operator(rule.Operator),
				Target:   derefString(rule.Target),
				Value:    derefString(rule.Value),
			})
		}
	}

	var statics []VariableModel
	if err := db.Where("workflow_id IN ? AND type = ?", ids, string(variable.TypeStatic)).Order("name").Find(&statics).Error; err != nil {
		return nil, err
	}

	callsByWorkflow := map[uuid.UUID][]domainworkflow.Call{}
	for _, c := range calls {
		call, err := callDomainFromModel(c, rulesByCall[c.ID])
		if err != nil {
			return nil, err
		}
		callsByWorkflow[c.WorkflowID] = append(callsByWorkflow[c.WorkflowID], call)
	}
	staticsByWorkflow := map[uuid.UUID][]domainworkflow.StaticVariable{}
	for _, s := range statics {
		staticsByWorkflow[*s.WorkflowID] = append(staticsByWorkflow[*s.WorkflowID], domainworkflow.StaticVariable{
			ID: s.ID, Name: s.Name, Value: s.Value,
		})
	}

	out := make([]*domainworkflow.Workflow, 0, len(models))
	for _, m := range models {
		out = append(out, &domainworkflow.Workflow{
			ID:        m.ID,
			UserID:    m.UserID,
			Name:      m.Name,
			Interval:  time.Duration(m.IntervalSeconds) * time.Second,
			Timeout:   time.Duration(m.TimeoutMS) * time.Millisecond,
			Enabled:   m.Enabled,
			Statics:   staticsByWorkflow[m.ID],
			Calls:     callsByWorkflow[m.ID],
			CreatedAt: m.CreatedAt,
			UpdatedAt: m.UpdatedAt,
		})
	}
	return out, nil
}

func workflowModelFromDomain(w *domainworkflow.Workflow) *WorkflowModel {
	return &WorkflowModel{
		ID:              w.ID,
		UserID:          w.UserID,
		Name:            w.Name,
		IntervalSeconds: int(w.Interval / time.Second),
		TimeoutMS:       int(w.Timeout / time.Millisecond),
		Enabled:         w.Enabled,
		CreatedAt:       w.CreatedAt,
		UpdatedAt:       w.UpdatedAt,
	}
}

type extractionJSON struct {
	Name       string `json:"name"`
	Source     string `json:"source"`
	Expression string `json:"expression,omitempty"`
}

func callModelFromDomain(workflowID uuid.UUID, c domainworkflow.Call, now time.Time) (CallModel, error) {
	headers, err := json.Marshal(c.Headers)
	if err != nil {
		return CallModel{}, err
	}
	query, err := json.Marshal(c.Query)
	if err != nil {
		return CallModel{}, err
	}
	extractions := make([]extractionJSON, 0, len(c.Extractions))
	for _, e := range c.Extractions {
		extractions = append(extractions, extractionJSON{Name: e.Name, Source: string(e.Source), Expression: e.Expression})
	}
	extractionsJSON, err := json.Marshal(extractions)
	if err != nil {
		return CallModel{}, err
	}
	return CallModel{
		ID:          c.ID,
		WorkflowID:  workflowID,
		Position:    c.Position,
		Name:        c.Name,
		Method:      c.Method,
		URL:         c.URL,
		Headers:     dbtype.JSONB(headers),
		Query:       dbtype.JSONB(query),
		Body:        nullableString(c.Body),
		TimeoutMS:   int(c.Timeout / time.Millisecond),
		VerifyTLS:   c.VerifyTLS,
		IPVersion:   string(c.IPVersion),
		ProxyURL:    nullableString(c.ProxyURL),
		Extractions: dbtype.JSONB(extractionsJSON),
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

func callDomainFromModel(m CallModel, rules []domainrule.Rule) (domainworkflow.Call, error) {
	headers := map[string]string{}
	if len(m.Headers) > 0 {
		if err := json.Unmarshal(m.Headers, &headers); err != nil {
			return domainworkflow.Call{}, err
		}
	}
	query := map[string]string{}
	if len(m.Query) > 0 {
		if err := json.Unmarshal(m.Query, &query); err != nil {
			return domainworkflow.Call{}, err
		}
	}
	var raw []extractionJSON
	if len(m.Extractions) > 0 {
		if err := json.Unmarshal(m.Extractions, &raw); err != nil {
			return domainworkflow.Call{}, err
		}
	}
	extractions := make([]domainworkflow.Extraction, 0, len(raw))
	for _, e := range raw {
		extractions = append(extractions, domainworkflow.Extraction{
			Name: e.Name, Source: domainworkflow.ExtractionSource(e.Source), Expression: e.Expression,
		})
	}
	return domainworkflow.Call{
		ID:          m.ID,
		Position:    m.Position,
		Name:        m.Name,
		Method:      m.Method,
		URL:         m.URL,
		Headers:     headers,
		Query:       query,
		Body:        derefString(m.Body),
		Timeout:     time.Duration(m.TimeoutMS) * time.Millisecond,
		VerifyTLS:   m.VerifyTLS,
		IPVersion:   domainworkflow.IPVersion(m.IPVersion),
		ProxyURL:    derefString(m.ProxyURL),
		Extractions: extractions,
		Rules:       rules,
	}, nil
}

func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

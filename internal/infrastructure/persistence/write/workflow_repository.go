package write

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	domaincall "go-api/internal/domain/call"
	"go-api/internal/domain/port"
	domainrule "go-api/internal/domain/rule"
	"go-api/internal/domain/variable"
	domainworkflow "go-api/internal/domain/workflow"
	"go-api/internal/infrastructure/persistence/dbtype"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type workflowWriteRepository struct {
	db     *gorm.DB
	cipher port.SecretCipher
}

func NewWorkflowWriteRepository(db *gorm.DB, cipher port.SecretCipher) domainworkflow.WriteRepository {
	return &workflowWriteRepository{db: db, cipher: cipher}
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
			"schedule_type":    model.ScheduleType,
			"interval_seconds": model.IntervalSeconds,
			"run_at":           model.RunAt,
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
		// Call variables also cascade-delete with calls; drop workflow-scoped rows explicitly.
		if err := db.Where("workflow_id = ? AND type IN ?", w.ID, []string{
			string(variable.TypeStatic), string(variable.TypeCall),
		}).Delete(&VariableModel{}).Error; err != nil {
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
	if len(calls) > 0 {
		if err := db.Create(&calls).Error; err != nil {
			return err
		}
	}
	if len(rules) > 0 {
		if err := db.Create(&rules).Error; err != nil {
			return err
		}
	}

	workflowID := w.ID
	vars := make([]VariableModel, 0, len(w.Statics))
	for _, s := range w.Statics {
		value := s.Value
		if s.Secret {
			encrypted, err := r.cipher.Encrypt(s.Value)
			if err != nil {
				return err
			}
			value = encrypted
		}
		vars = append(vars, VariableModel{
			ID:         s.ID,
			UserID:     w.UserID,
			WorkflowID: &workflowID,
			Type:       string(variable.TypeStatic),
			Name:       s.Name,
			Value:      value,
			Secret:     s.Secret,
			CreatedAt:  now,
			UpdatedAt:  now,
		})
	}
	for _, c := range w.Calls {
		callID := c.ID
		for _, e := range c.Extractions {
			id := e.ID
			if id == uuid.Nil {
				id = uuid.New()
			}
			source := string(e.Source)
			vars = append(vars, VariableModel{
				ID:         id,
				UserID:     w.UserID,
				WorkflowID: &workflowID,
				CallID:     &callID,
				Type:       string(variable.TypeCall),
				Name:       e.Name,
				Value:      "",
				Secret:     e.Secret,
				Source:     &source,
				Path:       nullableString(e.Expression),
				CreatedAt:  now,
				UpdatedAt:  now,
			})
		}
	}
	if len(vars) == 0 {
		return nil
	}
	return db.Create(&vars).Error
}

// Delete is a soft delete: calls, rules, variables and past runs are kept.
func (r *workflowWriteRepository) Delete(ctx context.Context, id uuid.UUID) error {
	now := time.Now().UTC()
	return DBWithContext(ctx, r.db).Model(&WorkflowModel{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]any{"deleted_at": now, "updated_at": now}).Error
}

func (r *workflowWriteRepository) GetByID(ctx context.Context, id uuid.UUID) (*domainworkflow.Workflow, error) {
	var model WorkflowModel
	err := DBWithContext(ctx, r.db).First(&model, "id = ? AND deleted_at IS NULL", id).Error
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
	if err := DBWithContext(ctx, r.db).Where("user_id = ? AND deleted_at IS NULL", userID).Order("created_at").Find(&models).Error; err != nil {
		return nil, err
	}
	return r.hydrate(ctx, models)
}

// hydrate loads calls, rules and variables (static + call) of the given workflows.
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

	var vars []VariableModel
	if err := db.Where("workflow_id IN ? AND type IN ?", ids, []string{
		string(variable.TypeStatic), string(variable.TypeCall),
	}).Order("created_at, name").Find(&vars).Error; err != nil {
		return nil, err
	}

	extractionsByCall := map[uuid.UUID][]domaincall.Extraction{}
	staticsByWorkflow := map[uuid.UUID][]domainworkflow.StaticVariable{}
	for _, v := range vars {
		switch v.Type {
		case string(variable.TypeStatic):
			value := v.Value
			if v.Secret {
				decrypted, err := r.cipher.Decrypt(v.Value)
				if err != nil {
					return nil, err
				}
				value = decrypted
			}
			staticsByWorkflow[*v.WorkflowID] = append(staticsByWorkflow[*v.WorkflowID], domainworkflow.StaticVariable{
				ID: v.ID, Name: v.Name, Value: value, Secret: v.Secret,
			})
		case string(variable.TypeCall):
			if v.CallID == nil {
				continue
			}
			extractionsByCall[*v.CallID] = append(extractionsByCall[*v.CallID], domaincall.Extraction{
				ID:         v.ID,
				Name:       v.Name,
				Source:     domaincall.ExtractionSource(derefString(v.Source)),
				Expression: derefString(v.Path),
				Secret:     v.Secret,
			})
		}
	}

	callsByWorkflow := map[uuid.UUID][]domaincall.Call{}
	for _, c := range calls {
		call, err := callDomainFromModel(c, rulesByCall[c.ID], extractionsByCall[c.ID])
		if err != nil {
			return nil, err
		}
		callsByWorkflow[c.WorkflowID] = append(callsByWorkflow[c.WorkflowID], call)
	}

	out := make([]*domainworkflow.Workflow, 0, len(models))
	for _, m := range models {
		out = append(out, workflowDomainFromModel(m, staticsByWorkflow[m.ID], callsByWorkflow[m.ID]))
	}
	return out, nil
}

func workflowDomainFromModel(m WorkflowModel, statics []domainworkflow.StaticVariable, calls []domaincall.Call) *domainworkflow.Workflow {
	w := &domainworkflow.Workflow{
		ID:           m.ID,
		UserID:       m.UserID,
		Name:         m.Name,
		ScheduleType: domainworkflow.ScheduleType(m.ScheduleType),
		Timeout:      time.Duration(m.TimeoutMS) * time.Millisecond,
		Enabled:      m.Enabled,
		Statics:      statics,
		Calls:        calls,
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
	}
	if w.ScheduleType == "" {
		w.ScheduleType = domainworkflow.ScheduleInterval
	}
	if m.IntervalSeconds != nil {
		w.Interval = time.Duration(*m.IntervalSeconds) * time.Second
	}
	if m.RunAt != nil {
		t := m.RunAt.UTC()
		w.RunAt = &t
	}
	return w
}

func workflowModelFromDomain(w *domainworkflow.Workflow) *WorkflowModel {
	model := &WorkflowModel{
		ID:           w.ID,
		UserID:       w.UserID,
		Name:         w.Name,
		ScheduleType: string(w.ScheduleType),
		TimeoutMS:    int(w.Timeout / time.Millisecond),
		Enabled:      w.Enabled,
		CreatedAt:    w.CreatedAt,
		UpdatedAt:    w.UpdatedAt,
	}
	if model.ScheduleType == "" {
		model.ScheduleType = string(domainworkflow.ScheduleInterval)
	}
	if w.ScheduleType == domainworkflow.ScheduleInterval || w.ScheduleType == "" {
		seconds := int(w.Interval / time.Second)
		model.IntervalSeconds = &seconds
	}
	if w.RunAt != nil {
		t := w.RunAt.UTC()
		model.RunAt = &t
	}
	return model
}

var emptyExtractionsJSON = dbtype.JSONB([]byte("[]"))

func callModelFromDomain(workflowID uuid.UUID, c domaincall.Call, now time.Time) (CallModel, error) {
	headers, err := json.Marshal(c.Headers)
	if err != nil {
		return CallModel{}, err
	}
	query, err := json.Marshal(c.Query)
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
		Extractions: emptyExtractionsJSON, // legacy column; call vars live in `variables`
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

func callDomainFromModel(m CallModel, rules []domainrule.Rule, extractions []domaincall.Extraction) (domaincall.Call, error) {
	headers := map[string]string{}
	if len(m.Headers) > 0 {
		if err := json.Unmarshal(m.Headers, &headers); err != nil {
			return domaincall.Call{}, err
		}
	}
	query := map[string]string{}
	if len(m.Query) > 0 {
		if err := json.Unmarshal(m.Query, &query); err != nil {
			return domaincall.Call{}, err
		}
	}
	if extractions == nil {
		extractions = []domaincall.Extraction{}
	}
	return domaincall.Call{
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
		IPVersion:   domaincall.IPVersion(m.IPVersion),
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

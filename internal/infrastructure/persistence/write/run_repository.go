package write

import (
	"context"
	"encoding/json"
	"time"

	"go-api/internal/domain/certificate"
	domainrun "go-api/internal/domain/run"
	"go-api/internal/infrastructure/persistence/dbtype"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type runWriteRepository struct {
	db *gorm.DB
}

func NewRunWriteRepository(db *gorm.DB) domainrun.WriteRepository {
	return &runWriteRepository{db: db}
}

func (r *runWriteRepository) Save(ctx context.Context, run *domainrun.Run) error {
	db := DBWithContext(ctx, r.db)
	variables, err := marshalJSONB(run.Variables)
	if err != nil {
		return err
	}
	model := RunModel{
		ID:           run.ID,
		WorkflowID:   run.WorkflowID,
		UserID:       run.UserID,
		Trigger:      run.Trigger,
		Status:       string(run.Status),
		Reason:       nullableString(run.Reason),
		StartedAt:    run.StartedAt.UTC(),
		FinishedAt:   run.FinishedAt.UTC(),
		DurationMS:   int(run.Duration() / time.Millisecond),
		CallsTotal:   run.Counts.Total(),
		CallsSuccess: run.Counts.Success,
		CallsFailed:  run.Counts.Failed,
		CallsError:   run.Counts.Error,
		CallsSkipped: run.Counts.Skipped,
		Variables:    variables,
		RecordedAt:   time.Now().UTC(),
	}
	if err := db.Create(&model).Error; err != nil {
		return err
	}
	if len(run.Calls) == 0 {
		return nil
	}

	calls := make([]CallRunModel, 0, len(run.Calls))
	var rules []RuleRunModel
	for _, c := range run.Calls {
		m, err := callRunModel(run, c)
		if err != nil {
			return err
		}
		calls = append(calls, m)
		for _, rr := range c.Rules {
			id := rr.ID
			if id == uuid.Nil {
				id = uuid.New()
			}
			rules = append(rules, RuleRunModel{
				ID:        id,
				CallRunID: m.ID,
				RunID:     run.ID,
				Position:  rr.Position,
				Subject:   rr.Subject,
				Operator:  rr.Operator,
				Text:      rr.Text,
				Observed:  rr.Observed,
				Expected:  rr.Expected,
				Passed:    rr.Passed,
			})
		}
	}
	if err := db.CreateInBatches(&calls, 100).Error; err != nil {
		return err
	}
	if len(rules) > 0 {
		return db.CreateInBatches(&rules, 500).Error
	}
	return nil
}

func callRunModel(run *domainrun.Run, c domainrun.CallRun) (CallRunModel, error) {
	request, err := marshalJSONB(c.Request)
	if err != nil {
		return CallRunModel{}, err
	}
	response, err := marshalJSONB(c.Response)
	if err != nil {
		return CallRunModel{}, err
	}
	timings, err := marshalJSONB(c.Timings)
	if err != nil {
		return CallRunModel{}, err
	}
	network, err := marshalJSONB(c.Network)
	if err != nil {
		return CallRunModel{}, err
	}
	var tls dbtype.JSONB
	if c.TLS != nil {
		raw, err := json.Marshal(c.TLS)
		if err != nil {
			return CallRunModel{}, err
		}
		tls = dbtype.JSONB(raw)
	}
	extracted, err := marshalJSONB(c.ExtractedVariables)
	if err != nil {
		return CallRunModel{}, err
	}
	used, err := marshalJSONBOr(c.UsedVariables, "[]")
	if err != nil {
		return CallRunModel{}, err
	}

	id := c.ID
	if id == uuid.Nil {
		id = uuid.New()
	}
	m := CallRunModel{
		ID:                 id,
		RunID:              run.ID,
		WorkflowID:         run.WorkflowID,
		CallID:             c.CallID,
		Name:               c.Name,
		Position:           c.Position,
		Status:             string(c.Status),
		Reason:             nullableString(c.Reason),
		Request:            request,
		Response:           response,
		Timings:            timings,
		Network:            network,
		TLS:                tls,
		ExtractedVariables: extracted,
		UsedVariables:      used,
		BodyRef:            nullableString(c.BodyRef),
		BodyHash:           nullableString(c.BodyHash),
		BodyEncoding:       nullableString(c.BodyEncoding),
		Truncated:          c.Response.Truncated,
		StartedAt:          nullableTime(c.StartedAt),
		FinishedAt:         nullableTime(c.FinishedAt),
	}
	keys, err := marshalJSONBOr(c.ResponseKeys, "[]")
	if err != nil {
		return CallRunModel{}, err
	}
	m.ResponseKeys = keys
	if c.HTTPStatus != 0 {
		status := c.HTTPStatus
		m.HTTPStatus = &status
	}
	if c.Error != nil {
		m.ErrorType = nullableString(c.Error.Type)
		m.ErrorMessage = nullableString(c.Error.Message)
	}
	return m, nil
}

func marshalJSONB(v any) (dbtype.JSONB, error) {
	return marshalJSONBOr(v, "{}")
}

// marshalJSONBOr writes empty (nil) values as the column default: the jsonb
// columns are NOT NULL and GORM sends an explicit NULL otherwise.
func marshalJSONBOr(v any, empty string) (dbtype.JSONB, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	if string(raw) == "null" {
		return dbtype.JSONB(empty), nil
	}
	return dbtype.JSONB(raw), nil
}

func nullableTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	utc := t.UTC()
	return &utc
}

type runRetention struct {
	db *gorm.DB
}

func NewRunRetention(db *gorm.DB) domainrun.Retention {
	return &runRetention{db: db}
}

// Deleting in small batches keeps each transaction and its locks short.
const deleteRunsBeforeSQL = `
DELETE FROM runs
WHERE id IN (SELECT id FROM runs WHERE started_at < @cutoff LIMIT @limit)`

const deleteRunsBatchSize = 1000

func (r *runRetention) DeleteRunsBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	var total int64
	for {
		res := r.db.WithContext(ctx).Exec(deleteRunsBeforeSQL, map[string]any{"cutoff": cutoff.UTC(), "limit": deleteRunsBatchSize})
		if res.Error != nil {
			return total, res.Error
		}
		total += res.RowsAffected
		if res.RowsAffected < deleteRunsBatchSize {
			return total, nil
		}
	}
}

type certificateRepository struct {
	db *gorm.DB
}

func NewCertificateRepository(db *gorm.DB) certificate.Repository {
	return &certificateRepository{db: db}
}

func (r *certificateRepository) Upsert(ctx context.Context, certificates []certificate.Certificate) error {
	if len(certificates) == 0 {
		return nil
	}
	rows := make([]CertificateModel, 0, len(certificates))
	for _, c := range certificates {
		seen := c.SeenAt.UTC()
		rows = append(rows, CertificateModel{
			ID:             uuid.New(),
			UserID:         c.UserID,
			Host:           c.Host,
			Port:           c.Port,
			Subject:        c.Subject,
			Issuer:         c.Issuer,
			SerialNumber:   c.SerialNumber,
			ExpiresAt:      c.ExpiresAt.UTC(),
			ChainExpiresAt: c.ChainExpiresAt.UTC(),
			ChainValid:     c.ChainValid,
			NameMatch:      c.NameMatch,
			FirstSeenAt:    seen,
			LastSeenAt:     seen,
		})
	}
	return DBWithContext(ctx, r.db).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "host"}, {Name: "port"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"subject", "issuer", "serial_number", "expires_at", "chain_expires_at",
			"chain_valid", "name_match", "last_seen_at",
		}),
		Where: clause.Where{Exprs: []clause.Expression{
			clause.Expr{SQL: "certificates.last_seen_at <= EXCLUDED.last_seen_at"},
		}},
	}).Create(&rows).Error
}

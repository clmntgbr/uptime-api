package read

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"go-api/internal/domain/paginate"
	domainschedule "go-api/internal/domain/schedule"
	domainworkflow "go-api/internal/domain/workflow"
	"go-api/internal/infrastructure/persistence/dbtype"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type workflowQueryRepository struct {
	db *gorm.DB
}

func NewWorkflowQueryRepository(db *gorm.DB) domainworkflow.ReadRepository {
	return &workflowQueryRepository{db: db}
}

type workflowRow struct {
	ID              uuid.UUID  `gorm:"column:id"`
	UserID          uuid.UUID  `gorm:"column:user_id"`
	Name            string     `gorm:"column:name"`
	ScheduleType    string     `gorm:"column:schedule_type"`
	IntervalSeconds *int       `gorm:"column:interval_seconds"`
	RunAt           *time.Time `gorm:"column:run_at"`
	Enabled         bool       `gorm:"column:enabled"`
	CreatedAt       time.Time  `gorm:"column:created_at"`
	UpdatedAt       time.Time  `gorm:"column:updated_at"`
	Status          *string    `gorm:"column:status"`
	NextRunAt       *time.Time `gorm:"column:next_run_at"`
	LastRunAt       *time.Time `gorm:"column:last_run_at"`
	LastStatus      *string    `gorm:"column:last_status"`
}

type workflowListRow struct {
	ID   uuid.UUID `gorm:"column:id"`
	Name string    `gorm:"column:name"`
}

var workflowSortColumns = map[string]string{
	"name":       "name",
	"createdAt":  "created_at",
	"created_at": "created_at",
	"updatedAt":  "updated_at",
	"updated_at": "updated_at",
}

func (r *workflowQueryRepository) List(ctx context.Context, userID uuid.UUID, q paginate.PaginateQuery) ([]domainworkflow.ListView, int64, error) {
	column, ok := workflowSortColumns[q.SortBy]
	if !ok {
		column = "created_at"
	}
	q.SortBy = column

	base := r.db.WithContext(ctx).
		Table("workflows").
		Where("user_id = ? AND deleted_at IS NULL", userID)
	if search := strings.TrimSpace(q.Search); search != "" {
		base = base.Where("name ILIKE ?", "%"+escapeLike(search)+"%")
	}

	scoped, total, err := Paginate(base, q)
	if err != nil {
		return nil, 0, err
	}
	var rows []workflowListRow
	if err := scoped.Select("id, name").Scan(&rows).Error; err != nil {
		return nil, 0, err
	}

	views := make([]domainworkflow.ListView, 0, len(rows))
	for _, row := range rows {
		views = append(views, domainworkflow.ListView{ID: row.ID, Name: row.Name})
	}
	return views, total, nil
}

// escapeLike makes % and _ typed by the user match literally.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func (r *workflowQueryRepository) FindByID(ctx context.Context, userID, id uuid.UUID) (*domainworkflow.View, error) {
	var row workflowRow
	err := r.db.WithContext(ctx).
		Table("workflows w").
		Select("w.id, w.user_id, w.name, w.schedule_type, w.interval_seconds, w.run_at, w.enabled, w.created_at, w.updated_at, s.status, s.next_run_at, s.last_run_at, s.last_status").
		Joins("LEFT JOIN workflow_state s ON s.workflow_id = w.id").
		Where("w.id = ? AND w.user_id = ? AND w.deleted_at IS NULL", id, userID).
		Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	view := &domainworkflow.View{
		ID:           row.ID,
		UserID:       row.UserID,
		Name:         row.Name,
		ScheduleType: domainworkflow.ScheduleType(row.ScheduleType),
		Enabled:      row.Enabled,
		RunAt:        row.RunAt,
		NextRunAt:    row.NextRunAt,
		LastRunAt:    row.LastRunAt,
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
	}
	if view.ScheduleType == "" {
		view.ScheduleType = domainworkflow.ScheduleInterval
	}
	if row.IntervalSeconds != nil {
		view.IntervalSeconds = *row.IntervalSeconds
	}
	if row.Status != nil {
		view.Status = domainschedule.Status(*row.Status)
	}
	if row.LastStatus != nil {
		view.LastStatus = *row.LastStatus
	}
	return view, nil
}

type callListRow struct {
	ID         uuid.UUID    `gorm:"column:id"`
	Name       string       `gorm:"column:name"`
	Method     string       `gorm:"column:method"`
	URL        string       `gorm:"column:url"`
	Position   int          `gorm:"column:position"`
	RecentRuns dbtype.JSONB `gorm:"column:recent_runs"`
}

type callRecentRunJSON struct {
	ID         uuid.UUID  `json:"id"`
	RunID      uuid.UUID  `json:"run_id"`
	Status     string     `json:"status"`
	HTTPStatus *int       `json:"http_status"`
	FinishedAt *time.Time `json:"finished_at"`
	LatencyMs  *int64     `json:"latency_ms"`
}

// listCallsSQL returns call definitions with up to CallRecentRunLimit past runs
// (oldest→newest) for the list sparkline. Named params only — no Postgres ? op.
const listCallsSQL = `
SELECT
  c.id,
  c.name,
  c.method,
  c.url,
  c.position,
  COALESCE(hist.recent_runs, '[]'::jsonb) AS recent_runs
FROM calls c
JOIN workflows w ON w.id = c.workflow_id
LEFT JOIN LATERAL (
  SELECT jsonb_agg(
    jsonb_build_object(
      'id', t.id,
      'run_id', t.run_id,
      'status', t.status,
      'http_status', t.http_status,
      'finished_at', t.finished_at,
      'latency_ms', t.latency_ms
    )
    ORDER BY t.finished_at ASC NULLS LAST, t.started_at ASC NULLS LAST
  ) AS recent_runs
  FROM (
    SELECT
      cr.id,
      cr.run_id,
      cr.status,
      cr.http_status,
      cr.finished_at,
      cr.started_at,
      ROUND((cr.timings->>'total')::double precision)::bigint AS latency_ms
    FROM call_runs cr
    WHERE cr.call_id = c.id
    ORDER BY cr.finished_at DESC NULLS LAST, cr.started_at DESC NULLS LAST
    LIMIT @recent_limit
  ) t
) hist ON TRUE
WHERE c.workflow_id = @workflow_id
  AND w.user_id = @user_id
  AND w.deleted_at IS NULL
ORDER BY c.position`

func (r *workflowQueryRepository) ListCalls(ctx context.Context, userID, workflowID uuid.UUID) ([]domainworkflow.CallListView, error) {
	var rows []callListRow
	err := r.db.WithContext(ctx).Raw(listCallsSQL, map[string]any{
		"workflow_id":  workflowID,
		"user_id":      userID,
		"recent_limit": domainworkflow.CallRecentRunLimit,
	}).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	views := make([]domainworkflow.CallListView, 0, len(rows))
	for _, row := range rows {
		recent, err := decodeCallRecentRuns(row.RecentRuns)
		if err != nil {
			return nil, err
		}
		v := domainworkflow.CallListView{
			ID:         row.ID,
			Name:       row.Name,
			Method:     row.Method,
			URL:        row.URL,
			Position:   row.Position,
			RecentRuns: recent,
		}
		if n := len(recent); n > 0 {
			last := recent[n-1]
			v.LastStatus = last.Status
			v.LastHTTPStatus = last.HTTPStatus
			v.LastFinishedAt = last.FinishedAt
			v.LastLatencyMs = last.LatencyMs
		}
		views = append(views, v)
	}
	return views, nil
}

func decodeCallRecentRuns(raw dbtype.JSONB) ([]domainworkflow.CallRecentRunView, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return []domainworkflow.CallRecentRunView{}, nil
	}
	var rows []callRecentRunJSON
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, err
	}
	out := make([]domainworkflow.CallRecentRunView, 0, len(rows))
	for _, row := range rows {
		out = append(out, domainworkflow.CallRecentRunView{
			ID:         row.ID,
			RunID:      row.RunID,
			Status:     row.Status,
			HTTPStatus: row.HTTPStatus,
			FinishedAt: row.FinishedAt,
			LatencyMs:  row.LatencyMs,
		})
	}
	return out, nil
}

type callDetailRow struct {
	ID        uuid.UUID    `gorm:"column:id"`
	Name      string       `gorm:"column:name"`
	Method    string       `gorm:"column:method"`
	URL       string       `gorm:"column:url"`
	Headers   dbtype.JSONB `gorm:"column:headers"`
	Query     dbtype.JSONB `gorm:"column:query"`
	Body      *string      `gorm:"column:body"`
	TimeoutMS int          `gorm:"column:timeout_ms"`
	VerifyTLS bool         `gorm:"column:verify_tls"`
	IPVersion string       `gorm:"column:ip_version"`
	ProxyURL  *string      `gorm:"column:proxy_url"`
}

func (r *workflowQueryRepository) FindCall(ctx context.Context, userID, workflowID, callID uuid.UUID) (*domainworkflow.CallView, error) {
	var row callDetailRow
	err := r.db.WithContext(ctx).
		Table("calls c").
		Select("c.id, c.name, c.method, c.url, c.headers, c.query, c.body, c.timeout_ms, c.verify_tls, c.ip_version, c.proxy_url").
		Joins("JOIN workflows w ON w.id = c.workflow_id").
		Where("c.id = ? AND c.workflow_id = ? AND w.user_id = ? AND w.deleted_at IS NULL", callID, workflowID, userID).
		Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	headers := map[string]string{}
	if len(row.Headers) > 0 {
		if err := json.Unmarshal(row.Headers, &headers); err != nil {
			return nil, err
		}
	}
	query := map[string]string{}
	if len(row.Query) > 0 {
		if err := json.Unmarshal(row.Query, &query); err != nil {
			return nil, err
		}
	}

	view := &domainworkflow.CallView{
		ID:        row.ID,
		Name:      row.Name,
		Method:    row.Method,
		URL:       row.URL,
		Headers:   headers,
		Query:     query,
		TimeoutMS: row.TimeoutMS,
		VerifyTLS: row.VerifyTLS,
		IPVersion: row.IPVersion,
	}
	if row.Body != nil {
		view.Body = *row.Body
	}
	if row.ProxyURL != nil {
		view.ProxyURL = *row.ProxyURL
	}
	return view, nil
}

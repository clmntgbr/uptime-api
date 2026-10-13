package paginate

import "math"

const (
	OrderByAsc   = "asc"
	OrderByDesc  = "desc"
	DefaultLimit = 20
	MaxLimit     = 200
)

type PaginateQuery struct {
	Page    int    `query:"page" validate:"omitempty,min=1"`
	Limit   int    `query:"limit" validate:"omitempty,min=1,max=200"`
	SortBy  string `query:"sortBy" validate:"omitempty,max=64"`
	OrderBy string `query:"orderBy" validate:"omitempty,oneof=asc desc"`
	Search  string `query:"search" validate:"omitempty,max=200"`
}

type PaginateResponse struct {
	Total      int `json:"total"`
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	TotalPages int `json:"totalPages"`
	Members    any `json:"members"`
}

func NewPaginateResponse(members any, total int, query PaginateQuery) PaginateResponse {
	totalPages := int(math.Ceil(float64(total) / float64(query.Limit)))
	return PaginateResponse{
		Members:    members,
		Total:      total,
		Page:       query.Page,
		Limit:      query.Limit,
		TotalPages: totalPages,
	}
}

func (p *PaginateQuery) Normalize() {
	if p.Page <= 0 {
		p.Page = 1
	}
	if p.Limit <= 0 {
		p.Limit = DefaultLimit
	}
	if p.Limit > MaxLimit {
		p.Limit = MaxLimit
	}
	if p.OrderBy != OrderByAsc && p.OrderBy != OrderByDesc {
		p.OrderBy = OrderByAsc
	}
}

func (p *PaginateQuery) Offset() int {
	return (p.Page - 1) * p.Limit
}

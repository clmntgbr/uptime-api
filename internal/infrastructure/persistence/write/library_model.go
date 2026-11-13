package write

import (
	"encoding/json"
	"time"

	domaincall "go-api/internal/domain/call"
	domainlibrary "go-api/internal/domain/library"
	domainrule "go-api/internal/domain/rule"
	"go-api/internal/infrastructure/persistence/dbtype"

	"github.com/google/uuid"
)

// LibraryItemModel is one row of library_items. The definition lives in Payload.
type LibraryItemModel struct {
	ID          uuid.UUID    `gorm:"column:id;primaryKey"`
	UserID      uuid.UUID    `gorm:"column:user_id"`
	Name        string       `gorm:"column:name"`
	Description string       `gorm:"column:description"`
	Payload     dbtype.JSONB `gorm:"column:payload"`
	CreatedAt   time.Time    `gorm:"column:created_at"`
	UpdatedAt   time.Time    `gorm:"column:updated_at"`
}

func (LibraryItemModel) TableName() string { return "library_items" }

// libraryPayload is the stored shape of a definition. Keys match the API request
// (CallRequest) so the JSON stays readable in the database.
type libraryPayload struct {
	Method    string            `json:"method"`
	URL       string            `json:"url"`
	Headers   map[string]string `json:"headers,omitempty"`
	Query     map[string]string `json:"query,omitempty"`
	Body      string            `json:"body,omitempty"`
	TimeoutMS int64             `json:"timeoutMs"`
	VerifyTLS bool              `json:"verifyTls"`
	IPVersion string            `json:"ipVersion"`
	ProxyURL  string            `json:"proxyUrl,omitempty"`
	Rules     []libraryRule     `json:"rules,omitempty"`
}

type libraryRule struct {
	Subject  string `json:"subject"`
	Operator string `json:"operator"`
	Target   string `json:"target,omitempty"`
	Value    string `json:"value,omitempty"`
}

func newLibraryItemModel(item domainlibrary.Item) (LibraryItemModel, error) {
	def := item.Definition
	payload := libraryPayload{
		Method:    def.Method,
		URL:       def.URL,
		Headers:   def.Headers,
		Query:     def.Query,
		Body:      def.Body,
		TimeoutMS: def.Timeout.Milliseconds(),
		VerifyTLS: def.VerifyTLS == nil || *def.VerifyTLS,
		IPVersion: string(def.IPVersion),
		ProxyURL:  def.ProxyURL,
	}
	for _, r := range def.Rules {
		payload.Rules = append(payload.Rules, libraryRule{
			Subject: string(r.Subject), Operator: string(r.Operator), Target: r.Target, Value: r.Value,
		})
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return LibraryItemModel{}, err
	}
	return LibraryItemModel{
		ID:          item.ID,
		UserID:      item.UserID,
		Name:        def.Name,
		Description: def.Description,
		Payload:     raw,
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
	}, nil
}

// toLibraryItem rebuilds the domain item. Stored data was validated on write,
// so no validation runs here.
func toLibraryItem(m LibraryItemModel) (domainlibrary.Item, error) {
	var payload libraryPayload
	if err := json.Unmarshal(m.Payload, &payload); err != nil {
		return domainlibrary.Item{}, err
	}
	verifyTLS := payload.VerifyTLS
	def := domaincall.Definition{
		Name:        m.Name,
		Description: m.Description,
		Method:      payload.Method,
		URL:         payload.URL,
		Headers:     payload.Headers,
		Query:       payload.Query,
		Body:        payload.Body,
		Timeout:     time.Duration(payload.TimeoutMS) * time.Millisecond,
		VerifyTLS:   &verifyTLS,
		IPVersion:   domaincall.IPVersion(payload.IPVersion),
		ProxyURL:    payload.ProxyURL,
	}
	for _, r := range payload.Rules {
		def.Rules = append(def.Rules, domainrule.Rule{
			Subject: domainrule.Subject(r.Subject), Operator: domainrule.Operator(r.Operator),
			Target: r.Target, Value: r.Value,
		})
	}
	return domainlibrary.Item{
		ID:         m.ID,
		UserID:     m.UserID,
		Definition: def,
		CreatedAt:  m.CreatedAt,
		UpdatedAt:  m.UpdatedAt,
	}, nil
}

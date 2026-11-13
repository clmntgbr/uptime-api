package importer

import (
	"encoding/json"
	"slices"
	"strings"

	domaincall "go-api/internal/domain/call"
	"go-api/internal/domain/library"
)

type pmCollection struct {
	Info struct {
		Schema string `json:"schema"`
	} `json:"info"`
	Item []pmItem `json:"item"`
	Auth *pmAuth  `json:"auth"`
}

// pmItem is a folder when Item is set, otherwise a request when Request is set.
type pmItem struct {
	Name    string          `json:"name"`
	Item    []pmItem        `json:"item"`
	Request json.RawMessage `json:"request"`
	Auth    *pmAuth         `json:"auth"`
}

type pmRequest struct {
	Method      string          `json:"method"`
	Header      []pmField       `json:"header"`
	URL         json.RawMessage `json:"url"`
	Description json.RawMessage `json:"description"`
	Body        *pmBody         `json:"body"`
	Auth        *pmAuth         `json:"auth"`
}

type pmURL struct {
	Raw   string    `json:"raw"`
	Host  pmParts   `json:"host"`
	Path  pmParts   `json:"path"`
	Query []pmField `json:"query"`
}

type pmBody struct {
	Mode    string `json:"mode"`
	Raw     string `json:"raw"`
	Options struct {
		Raw struct {
			Language string `json:"language"`
		} `json:"raw"`
	} `json:"options"`
}

type pmAuth struct {
	Type   string    `json:"type"`
	APIKey []pmField `json:"apikey"`
}

type pmField struct {
	Key      pmScalar `json:"key"`
	Value    pmScalar `json:"value"`
	Disabled bool     `json:"disabled"`
}

// pmScalar accepts a JSON string, number, boolean or null and keeps its text.
type pmScalar string

func (s *pmScalar) UnmarshalJSON(data []byte) error {
	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		*s = pmScalar(text)
		return nil
	}
	*s = pmScalar(data)
	return nil
}

// pmParts is a host or path given either as a list of segments or as one string.
type pmParts []pmScalar

func (p *pmParts) UnmarshalJSON(data []byte) error {
	var list []pmScalar
	if err := json.Unmarshal(data, &list); err == nil {
		*p = list
		return nil
	}
	var one pmScalar
	if err := json.Unmarshal(data, &one); err != nil {
		return err
	}
	*p = pmParts{one}
	return nil
}

func (p pmParts) join(sep string) string {
	parts := make([]string, len(p))
	for i, segment := range p {
		parts[i] = string(segment)
	}
	return strings.Join(parts, sep)
}

// postmanParser holds the result while the collection tree is walked.
type postmanParser struct {
	result library.ImportResult
}

// parsePostman converts a Postman v2.1 collection. Folders are flattened; their
// names are kept in the request description.
func parsePostman(content []byte) (library.ImportResult, error) {
	var col pmCollection
	if err := json.Unmarshal(content, &col); err != nil || col.Item == nil {
		return library.ImportResult{}, importError("content is not a Postman collection")
	}
	if strings.Contains(col.Info.Schema, "v2.0") {
		return library.ImportResult{}, importError("Postman v2.0 is not supported")
	}
	p := &postmanParser{}
	p.walk(col.Item, effectiveAuth(col.Auth, nil), nil)
	return p.result, nil
}

// effectiveAuth returns the item's own auth unless it is absent or inherits.
func effectiveAuth(own, inherited *pmAuth) *pmAuth {
	if own != nil && own.Type != "" && own.Type != "inherit" {
		return own
	}
	return inherited
}

func (p *postmanParser) walk(items []pmItem, inherited *pmAuth, folders []string) {
	for _, item := range items {
		auth := effectiveAuth(item.Auth, inherited)
		if item.Item != nil {
			p.walk(item.Item, auth, slices.Concat(folders, []string{item.Name}))
			continue
		}
		if len(item.Request) == 0 || string(item.Request) == "null" {
			continue
		}
		p.request(item, auth, folders)
	}
}

func (p *postmanParser) request(item pmItem, auth *pmAuth, folders []string) {
	source := item.Name
	if source == "" {
		source = "unnamed request"
	}
	req, err := decodeRequest(item.Request)
	if err != nil {
		addSkipped(&p.result, source, "request is not valid")
		return
	}
	method := strings.ToUpper(strings.TrimSpace(req.Method))
	if method == "" {
		method = "GET"
	}
	rawURL, query := postmanURL(req.URL)
	if rawURL == "" {
		addSkipped(&p.result, source, "no URL")
		return
	}

	var ws warningList
	def := domaincall.Definition{
		Method:      method,
		URL:         rawURL,
		Description: postmanDescription(folders, req.Description),
		Headers:     map[string]string{},
		Query:       query,
	}
	def.Name = strings.TrimSpace(item.Name)
	if def.Name == "" {
		def.Name = endpointName(method, rawURL)
	}
	for _, h := range req.Header {
		if !h.Disabled && h.Key != "" {
			setHeader(def.Headers, string(h.Key), string(h.Value))
		}
	}
	postmanBody(&def, req.Body, &ws)
	applyPostmanAuth(&def, effectiveAuth(req.Auth, auth), &ws)
	addCandidate(&p.result, "req", def, ws)
}

// decodeRequest accepts a request object or a bare URL string.
func decodeRequest(raw json.RawMessage) (pmRequest, error) {
	if len(raw) > 0 && raw[0] == '"' {
		return pmRequest{URL: raw}, nil
	}
	var req pmRequest
	err := json.Unmarshal(raw, &req)
	return req, err
}

// postmanURL returns the URL without its query, and the enabled query
// parameters. A raw URL without query[] entries keeps its inline query as
// parameters.
func postmanURL(raw json.RawMessage) (string, map[string]string) {
	query := map[string]string{}
	var plain string
	if err := json.Unmarshal(raw, &plain); err == nil {
		return plain, query
	}
	var obj pmURL
	if err := json.Unmarshal(raw, &obj); err != nil {
		return "", query
	}
	base, rawQuery, _ := strings.Cut(obj.Raw, "?")
	if obj.Raw == "" {
		base = obj.Host.join(".")
		if path := obj.Path.join("/"); path != "" {
			base += "/" + path
		}
	}
	if len(obj.Query) == 0 {
		return base, parseQueryPairs(rawQuery)
	}
	for _, f := range obj.Query {
		if !f.Disabled && f.Key != "" {
			query[string(f.Key)] = string(f.Value)
		}
	}
	return base, query
}

// postmanBody imports raw bodies. Form, GraphQL, file and other modes are reported.
func postmanBody(def *domaincall.Definition, body *pmBody, ws *warningList) {
	if body == nil || body.Mode == "" {
		return
	}
	if body.Mode != "raw" {
		ws.add(warnUnsupportedBody, "body", "Postman %s bodies are not supported; the body was not imported", body.Mode)
		return
	}
	if body.Raw == "" {
		return
	}
	def.Body = body.Raw
	if (body.Options.Raw.Language == "json" || looksLikeJSON(body.Raw)) && !hasHeader(def.Headers, "Content-Type") {
		def.Headers["Content-Type"] = "application/json"
	}
}

func looksLikeJSON(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	return (strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[")) && json.Valid([]byte(trimmed))
}

// applyPostmanAuth turns the auth scheme into placeholders only.
func applyPostmanAuth(def *domaincall.Definition, auth *pmAuth, ws *warningList) {
	if auth == nil {
		return
	}
	switch auth.Type {
	case "bearer":
		setAuthPlaceholder(def, "header", "Authorization", "Bearer {{token}}", ws)
	case "basic":
		setAuthPlaceholder(def, "header", "Authorization", "Basic {{credentials}}", ws)
	case "apikey":
		name := apiKeyField(auth.APIKey, "key")
		if name == "" {
			return
		}
		in := apiKeyField(auth.APIKey, "in")
		if in == "" {
			in = "header"
		}
		if in == "cookie" {
			ws.add(warnUnsupportedOption, "cookie."+name, "Cookie authentication is not translated; add the cookie manually")
			return
		}
		setAuthPlaceholder(def, in, name, "{{api_key}}", ws)
	}
}

// apiKeyField returns the value of the apikey entry whose key is key.
func apiKeyField(fields []pmField, key string) string {
	for _, f := range fields {
		if string(f.Key) == key {
			return string(f.Value)
		}
	}
	return ""
}

// postmanDescription combines the folder path and the request description.
func postmanDescription(folders []string, raw json.RawMessage) string {
	var parts []string
	if len(folders) > 0 {
		parts = append(parts, "Folder: "+strings.Join(folders, " / "))
	}
	if text := strings.TrimSpace(descriptionText(raw)); text != "" {
		parts = append(parts, text)
	}
	return strings.Join(parts, "\n\n")
}

// descriptionText reads a description given as a string or as {"content": "..."}.
func descriptionText(raw json.RawMessage) string {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}
	var obj struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(raw, &obj); err == nil {
		return obj.Content
	}
	return ""
}

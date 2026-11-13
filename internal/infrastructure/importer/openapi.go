package importer

import (
	"encoding/json"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"

	domaincall "go-api/internal/domain/call"
	"go-api/internal/domain/library"

	"gopkg.in/yaml.v3"
)

const (
	// maxRefDepth bounds $ref chains so cyclic references cannot loop.
	maxRefDepth = 16
	// maxSchemaDepth bounds the example generated from nested schemas.
	maxSchemaDepth = 8
)

var (
	pathParamPattern = regexp.MustCompile(`\{([^{}]*)\}`)
	openAPIMethods   = []string{"get", "put", "post", "delete", "patch", "head", "options"}
)

// openAPIParser converts one OpenAPI document.
type openAPIParser struct {
	root   map[string]any
	result library.ImportResult
}

// parseOpenAPI converts an OpenAPI 3 document, in JSON or YAML, into one
// candidate per operation. Paths are visited in sorted order.
func parseOpenAPI(content []byte) (library.ImportResult, error) {
	var doc any
	if err := yaml.Unmarshal(content, &doc); err != nil {
		return library.ImportResult{}, importError("content is not a valid JSON or YAML document")
	}
	root, ok := asMap(doc)
	if !ok {
		return library.ImportResult{}, importError("content is not an OpenAPI document")
	}
	if _, ok := root["swagger"]; ok {
		return library.ImportResult{}, importError("Swagger 2.0 is not supported")
	}
	paths, hasPaths := asMap(root["paths"])
	if !strings.HasPrefix(stringValue(root["openapi"]), "3") && !hasPaths {
		return library.ImportResult{}, importError("content is not an OpenAPI 3 document")
	}
	p := &openAPIParser{root: root}
	for _, path := range slices.Sorted(maps.Keys(paths)) {
		p.pathItem(path, paths[path])
	}
	return p.result, nil
}

func (p *openAPIParser) pathItem(path string, raw any) {
	item, err := p.deref(raw)
	if err != nil {
		addSkipped(&p.result, path, err.Error())
		return
	}
	for _, method := range openAPIMethods {
		op, ok := item[method]
		if !ok {
			continue
		}
		p.operation(method, path, item, op)
	}
}

func (p *openAPIParser) operation(method, path string, pathItem map[string]any, raw any) {
	def, ws, err := p.buildOperation(method, path, pathItem, raw)
	if err != nil {
		addSkipped(&p.result, strings.ToUpper(method)+" "+path, err.Error())
		return
	}
	addCandidate(&p.result, "op", def, ws)
}

// buildOperation converts one operation. An unresolvable reference makes the
// whole operation fail, so the caller reports it as skipped.
func (p *openAPIParser) buildOperation(method, path string, pathItem map[string]any, raw any) (domaincall.Definition, warningList, error) {
	var ws warningList
	op, err := p.deref(raw)
	if err != nil {
		return domaincall.Definition{}, nil, err
	}
	params, err := p.parameters(pathItem["parameters"], op["parameters"])
	if err != nil {
		return domaincall.Definition{}, nil, err
	}

	def := domaincall.Definition{
		Method:      strings.ToUpper(method),
		URL:         p.operationURL(path, op, pathItem, &ws),
		Description: strings.TrimSpace(stringValue(op["description"])),
		Headers:     map[string]string{},
		Query:       map[string]string{},
	}
	def.Name = strings.TrimSpace(stringValue(op["summary"]))
	if def.Name == "" {
		def.Name = strings.TrimSpace(stringValue(op["operationId"]))
	}
	if def.Name == "" {
		def.Name = def.Method + " " + path
	}
	if deprecated, _ := op["deprecated"].(bool); deprecated {
		ws.add(warnDeprecated, "", "Operation is marked as deprecated")
	}

	for _, param := range params {
		if err := p.applyParameter(&def, param, &ws); err != nil {
			return domaincall.Definition{}, nil, err
		}
	}
	if err := p.applyRequestBody(&def, op, &ws); err != nil {
		return domaincall.Definition{}, nil, err
	}
	if err := p.applySecurity(&def, op, &ws); err != nil {
		return domaincall.Definition{}, nil, err
	}
	return def, ws, nil
}

// operationURL prefixes the path with {{base_url}} when a server is declared,
// and turns each {param} into a workflow variable.
func (p *openAPIParser) operationURL(path string, op, pathItem map[string]any, ws *warningList) string {
	prefix := ""
	if server := serverURL(op, pathItem, p.root); server != "" {
		ws.add(warnVariableToDefine, "url", "Set base_url to the server URL %s", server)
		prefix = "{{base_url}}"
	}
	seen := map[string]bool{}
	resolved := pathParamPattern.ReplaceAllStringFunc(path, func(match string) string {
		variable := templateVariable(match[1:len(match)-1], "url", ws)
		if !seen[variable] {
			seen[variable] = true
			ws.add(warnVariableToDefine, "url", "Provide the value of {{%s}} in the workflow", variable)
		}
		return "{{" + variable + "}}"
	})
	return prefix + resolved
}

// serverURL returns the first server URL of the operation, path item or
// document, in that order of precedence.
func serverURL(op, pathItem, root map[string]any) string {
	for _, level := range []map[string]any{op, pathItem, root} {
		servers, _ := asList(level["servers"])
		if len(servers) == 0 {
			continue
		}
		first, _ := asMap(servers[0])
		if u := stringValue(first["url"]); u != "" {
			return u
		}
	}
	return ""
}

// parameters merges path-level and operation-level parameters; the operation
// wins when both declare the same name and location.
func (p *openAPIParser) parameters(pathLevel, operationLevel any) ([]map[string]any, error) {
	var params []map[string]any
	index := map[string]int{}
	for _, list := range []any{pathLevel, operationLevel} {
		items, _ := asList(list)
		for _, raw := range items {
			param, err := p.deref(raw)
			if err != nil {
				return nil, err
			}
			key := stringValue(param["in"]) + "\x00" + stringValue(param["name"])
			if i, ok := index[key]; ok {
				params[i] = param
				continue
			}
			index[key] = len(params)
			params = append(params, param)
		}
	}
	return params, nil
}

// applyParameter maps query and header parameters. Path parameters are already
// part of the URL.
func (p *openAPIParser) applyParameter(def *domaincall.Definition, param map[string]any, ws *warningList) error {
	name := stringValue(param["name"])
	if name == "" {
		return nil
	}
	switch stringValue(param["in"]) {
	case "query":
		value, err := p.parameterValue(param)
		if err != nil {
			return err
		}
		def.Query[name] = value
	case "header":
		value, err := p.parameterValue(param)
		if err != nil {
			return err
		}
		setHeader(def.Headers, name, value)
	case "cookie":
		ws.add(warnCookieParameter, "cookie."+name, "Cookie parameter %s is not imported", name)
	}
	return nil
}

// parameterValue returns the example, default or first enum value of a
// parameter, looking at the parameter first and then at its schema.
func (p *openAPIParser) parameterValue(param map[string]any) (string, error) {
	if value, ok := firstScalar(param); ok {
		return value, nil
	}
	schema, err := p.deref(param["schema"])
	if err != nil {
		return "", err
	}
	value, _ := firstScalar(schema)
	return value, nil
}

// firstScalar returns the example, default or first enum value of a
// schema-like node, formatted as text.
func firstScalar(node map[string]any) (string, bool) {
	for _, key := range []string{"example", "default"} {
		if v, ok := node[key]; ok && v != nil {
			return fmt.Sprint(v), true
		}
	}
	if values, _ := asList(node["enum"]); len(values) > 0 && values[0] != nil {
		return fmt.Sprint(values[0]), true
	}
	return "", false
}

// applyRequestBody imports a JSON request body. Other media types are reported
// and left out.
func (p *openAPIParser) applyRequestBody(def *domaincall.Definition, op map[string]any, ws *warningList) error {
	body, err := p.deref(op["requestBody"])
	if err != nil {
		return err
	}
	content, _ := asMap(body["content"])
	if len(content) == 0 {
		return nil
	}
	mediaType := jsonMediaType(content)
	if mediaType == "" {
		ws.add(warnUnsupportedBody, "body", "Request body type %s is not supported; the body was not imported", slices.Sorted(maps.Keys(content))[0])
		return nil
	}
	media, err := p.deref(content[mediaType])
	if err != nil {
		return err
	}
	value, err := p.bodyExample(media)
	if err != nil {
		return err
	}
	if value == nil {
		return nil
	}
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		ws.add(warnUnsupportedBody, "body", "Request body example cannot be converted to JSON; the body was not imported")
		return nil
	}
	def.Body = string(encoded)
	if !hasHeader(def.Headers, "Content-Type") {
		def.Headers["Content-Type"] = "application/json"
	}
	return nil
}

// jsonMediaType returns application/json when present, otherwise the first
// media type whose name contains "json", or "" when there is none.
func jsonMediaType(content map[string]any) string {
	if _, ok := content["application/json"]; ok {
		return "application/json"
	}
	for _, name := range slices.Sorted(maps.Keys(content)) {
		if strings.Contains(strings.ToLower(name), "json") {
			return name
		}
	}
	return ""
}

// bodyExample returns the media example, else the first named example value,
// else an example generated from the schema.
func (p *openAPIParser) bodyExample(media map[string]any) (any, error) {
	if v, ok := media["example"]; ok && v != nil {
		return v, nil
	}
	examples, _ := asMap(media["examples"])
	for _, name := range slices.Sorted(maps.Keys(examples)) {
		example, err := p.deref(examples[name])
		if err != nil {
			return nil, err
		}
		if v := example["value"]; v != nil {
			return v, nil
		}
	}
	schema, err := p.deref(media["schema"])
	if err != nil {
		return nil, err
	}
	return p.schemaExample(schema, 0)
}

// schemaExample builds an example value from a schema. It returns nil when no
// value can be derived, including beyond maxSchemaDepth.
func (p *openAPIParser) schemaExample(schema map[string]any, depth int) (any, error) {
	if schema == nil || depth > maxSchemaDepth {
		return nil, nil
	}
	if v, ok := schema["example"]; ok && v != nil {
		return v, nil
	}
	if v, ok := schema["default"]; ok && v != nil {
		return v, nil
	}
	if values, _ := asList(schema["enum"]); len(values) > 0 {
		return values[0], nil
	}
	for _, key := range []string{"oneOf", "anyOf"} {
		if options, _ := asList(schema[key]); len(options) > 0 {
			first, err := p.deref(options[0])
			if err != nil {
				return nil, err
			}
			return p.schemaExample(first, depth+1)
		}
	}
	_, hasAllOf := schema["allOf"]
	_, hasProperties := schema["properties"]
	if hasAllOf || hasProperties || stringValue(schema["type"]) == "object" {
		return p.objectExample(schema, depth)
	}
	switch stringValue(schema["type"]) {
	case "string":
		return "", nil
	case "integer", "number":
		return 0, nil
	case "boolean":
		return false, nil
	case "array":
		items, err := p.deref(schema["items"])
		if err != nil {
			return nil, err
		}
		item, err := p.schemaExample(items, depth+1)
		if err != nil {
			return nil, err
		}
		if item == nil {
			return []any{}, nil
		}
		return []any{item}, nil
	}
	return nil, nil
}

// objectExample builds an object from the schema properties and from the
// allOf members, merged into one map.
func (p *openAPIParser) objectExample(schema map[string]any, depth int) (any, error) {
	out := map[string]any{}
	props, _ := asMap(schema["properties"])
	for _, name := range slices.Sorted(maps.Keys(props)) {
		prop, err := p.deref(props[name])
		if err != nil {
			return nil, err
		}
		value, err := p.schemaExample(prop, depth+1)
		if err != nil {
			return nil, err
		}
		out[name] = value
	}
	members, _ := asList(schema["allOf"])
	for _, raw := range members {
		member, err := p.deref(raw)
		if err != nil {
			return nil, err
		}
		value, err := p.schemaExample(member, depth+1)
		if err != nil {
			return nil, err
		}
		if m, ok := value.(map[string]any); ok {
			maps.Copy(out, m)
		}
	}
	return out, nil
}

// applySecurity applies the first security requirement that names a scheme.
// Only placeholders are inserted, never secrets.
func (p *openAPIParser) applySecurity(def *domaincall.Definition, op map[string]any, ws *warningList) error {
	requirements, ok := op["security"]
	if !ok {
		requirements = p.root["security"]
	}
	list, _ := asList(requirements)
	for _, raw := range list {
		requirement, _ := asMap(raw)
		if len(requirement) == 0 {
			continue
		}
		for _, name := range slices.Sorted(maps.Keys(requirement)) {
			scheme, err := p.securityScheme(name)
			if err != nil {
				return err
			}
			p.applyScheme(def, scheme, ws)
		}
		return nil
	}
	return nil
}

func (p *openAPIParser) securityScheme(name string) (map[string]any, error) {
	components, err := p.deref(p.root["components"])
	if err != nil {
		return nil, err
	}
	schemes, _ := asMap(components["securitySchemes"])
	return p.deref(schemes[name])
}

func (p *openAPIParser) applyScheme(def *domaincall.Definition, scheme map[string]any, ws *warningList) {
	switch stringValue(scheme["type"]) {
	case "http":
		switch strings.ToLower(stringValue(scheme["scheme"])) {
		case "bearer":
			setAuthPlaceholder(def, "header", "Authorization", "Bearer {{token}}", ws)
		case "basic":
			setAuthPlaceholder(def, "header", "Authorization", "Basic {{credentials}}", ws)
		}
	case "oauth2", "openIdConnect":
		setAuthPlaceholder(def, "header", "Authorization", "Bearer {{token}}", ws)
	case "apiKey":
		name := stringValue(scheme["name"])
		if name == "" {
			return
		}
		if in := stringValue(scheme["in"]); in == "cookie" {
			ws.add(warnUnsupportedOption, "cookie."+name, "Cookie authentication is not translated; add the cookie manually")
		} else {
			setAuthPlaceholder(def, in, name, "{{api_key}}", ws)
		}
	}
}

// deref follows internal $ref chains. A nil node yields a nil map and no error.
// External references and chains longer than maxRefDepth are errors.
func (p *openAPIParser) deref(v any) (map[string]any, error) {
	for range maxRefDepth {
		if v == nil {
			return nil, nil
		}
		node, ok := asMap(v)
		if !ok {
			return nil, fmt.Errorf("node is not an object")
		}
		ref, isRef := node["$ref"].(string)
		if !isRef {
			return node, nil
		}
		target, err := p.lookupRef(ref)
		if err != nil {
			return nil, err
		}
		v = target
	}
	return nil, fmt.Errorf("reference chain is too deep")
}

// lookupRef resolves an internal JSON pointer ("#/..."), unescaping ~1 and ~0.
func (p *openAPIParser) lookupRef(ref string) (any, error) {
	if !strings.HasPrefix(ref, "#/") {
		return nil, fmt.Errorf("external references are not supported")
	}
	var current any = p.root
	for _, token := range strings.Split(ref[2:], "/") {
		token = strings.ReplaceAll(strings.ReplaceAll(token, "~1", "/"), "~0", "~")
		switch node := current.(type) {
		case map[string]any:
			next, ok := node[token]
			if !ok {
				return nil, fmt.Errorf("reference %s cannot be resolved", ref)
			}
			current = next
		case []any:
			index, err := strconv.Atoi(token)
			if err != nil || index < 0 || index >= len(node) {
				return nil, fmt.Errorf("reference %s cannot be resolved", ref)
			}
			current = node[index]
		default:
			return nil, fmt.Errorf("reference %s cannot be resolved", ref)
		}
	}
	return current, nil
}

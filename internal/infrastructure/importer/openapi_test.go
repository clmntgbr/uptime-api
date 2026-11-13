package importer

import (
	"encoding/json"
	"errors"
	"testing"

	"go-api/internal/domain/library"
)

const petstoreYAML = `
openapi: 3.0.3
info: {title: Pets, version: "1"}
servers:
  - url: https://api.example.com/v1
paths:
  /pets/{petId}:
    parameters:
      - name: petId
        in: path
        required: true
        schema: {type: integer}
    get:
      operationId: getPet
      summary: Get a pet
      parameters:
        - name: verbose
          in: query
          schema: {type: boolean, default: true}
        - name: X-Request-Id
          in: header
          example: abc
      responses: {"200": {description: ok}}
    put:
      summary: Update a pet
      deprecated: true
      security:
        - bearerAuth: []
      requestBody:
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/Pet'
  /files:
    post:
      requestBody:
        content:
          multipart/form-data:
            schema: {type: object}
components:
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
  schemas:
    Pet:
      type: object
      properties:
        name: {type: string}
        age: {type: integer}
        tags: {type: array, items: {type: string}}
        owner: {$ref: '#/components/schemas/Owner'}
    Owner:
      type: object
      properties:
        email: {type: string, example: a@b.c}
`

const petBody = `{
  "age": 0,
  "name": "",
  "owner": {
    "email": "a@b.c"
  },
  "tags": [
    ""
  ]
}`

func TestParseOpenAPIPetstore(t *testing.T) {
	res, err := NewParser().Parse(library.FormatOpenAPI, []byte(petstoreYAML))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Candidates) != 3 || len(res.Skipped) != 0 {
		t.Fatalf("got %d candidates, %d skipped; want 3, 0", len(res.Candidates), len(res.Skipped))
	}
	// Paths are visited in sorted order: /files before /pets/{petId}.
	checkCandidate(t, res.Candidates[0], wantCandidate{
		name:     "POST /files",
		method:   "POST",
		url:      "{{base_url}}/files",
		warnings: []string{"unsupported_body", "variable_to_define"},
	})
	checkCandidate(t, res.Candidates[1], wantCandidate{
		name:     "Get a pet",
		method:   "GET",
		url:      "{{base_url}}/pets/{{petId}}",
		query:    map[string]string{"verbose": "true"},
		headers:  map[string]string{"X-Request-Id": "abc"},
		warnings: []string{"variable_to_define", "variable_to_define"},
	})
	checkCandidate(t, res.Candidates[2], wantCandidate{
		name:     "Update a pet",
		method:   "PUT",
		url:      "{{base_url}}/pets/{{petId}}",
		body:     petBody,
		headers:  map[string]string{"Authorization": "Bearer {{token}}", "Content-Type": "application/json"},
		warnings: []string{"deprecated_operation", "auth_placeholder", "variable_to_define", "variable_to_define"},
	})
	if res.Candidates[1].Key != "op-2" || res.Candidates[2].Key != "op-3" {
		t.Errorf("keys = %q, %q; want op-2, op-3", res.Candidates[1].Key, res.Candidates[2].Key)
	}
}

func TestParseOpenAPIJSONWithCookieAndAPIKeyQuery(t *testing.T) {
	doc := `{
	  "openapi": "3.1.0",
	  "paths": {
	    "/orders": {
	      "post": {
	        "operationId": "createOrder",
	        "description": "Creates an order",
	        "parameters": [{"name": "session", "in": "cookie", "schema": {"type": "string"}}],
	        "requestBody": {"content": {"application/json": {"example": {"sku": "A-1", "qty": 2}}}},
	        "security": [{"partner": []}]
	      }
	    }
	  },
	  "components": {"securitySchemes": {"partner": {"type": "apiKey", "in": "query", "name": "key"}}}
	}`
	res, err := NewParser().Parse(library.FormatOpenAPI, []byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Candidates) != 1 {
		t.Fatalf("got %d candidates, want 1", len(res.Candidates))
	}
	checkCandidate(t, res.Candidates[0], wantCandidate{
		name:        "createOrder",
		method:      "POST",
		url:         "/orders",
		body:        "{\n  \"qty\": 2,\n  \"sku\": \"A-1\"\n}",
		description: "Creates an order",
		headers:     map[string]string{"Content-Type": "application/json"},
		query:       map[string]string{"key": "{{api_key}}"},
		warnings:    []string{"cookie_parameter_ignored", "auth_placeholder"},
	})
}

func TestParseOpenAPISkipsUnresolvableOperations(t *testing.T) {
	doc := `
openapi: 3.0.0
paths:
  /external:
    get:
      parameters:
        - $ref: "https://example.com/params.yaml#/p"
  /loop-body:
    get:
      requestBody: {$ref: "#/components/requestBodies/Loop"}
  /loop-schema:
    post:
      requestBody:
        content:
          application/json:
            schema: {$ref: "#/components/schemas/Loop"}
components:
  requestBodies:
    Loop: {$ref: "#/components/requestBodies/Loop"}
  schemas:
    Loop: {$ref: "#/components/schemas/Loop"}
`
	res, err := NewParser().Parse(library.FormatOpenAPI, []byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Candidates) != 0 || len(res.Skipped) != 3 {
		t.Fatalf("got %d candidates, %d skipped; want 0, 3", len(res.Candidates), len(res.Skipped))
	}
	if res.Skipped[0].Source != "GET /external" || res.Skipped[0].Reason != "external references are not supported" {
		t.Errorf("first skipped = %+v", res.Skipped[0])
	}
}

func TestParseOpenAPIRecursiveSchemaTerminates(t *testing.T) {
	doc := `
openapi: 3.0.0
paths:
  /nodes:
    post:
      requestBody:
        content:
          application/json:
            schema: {$ref: "#/components/schemas/Node"}
components:
  schemas:
    Node:
      type: object
      properties:
        children:
          type: array
          items: {$ref: "#/components/schemas/Node"}
`
	res, err := NewParser().Parse(library.FormatOpenAPI, []byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Candidates) != 1 {
		t.Fatalf("got %d candidates, want 1", len(res.Candidates))
	}
	var decoded any
	if err := json.Unmarshal([]byte(res.Candidates[0].Definition.Body), &decoded); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
}

func TestParseOpenAPIRejections(t *testing.T) {
	tests := []struct {
		name    string
		content string
		message string
	}{
		{"swagger 2", "swagger: \"2.0\"\npaths: {}\n", "Swagger 2.0 is not supported"},
		{"not a document", "just text", "content is not an OpenAPI document"},
		{"not openapi", `{"info": {}}`, "content is not an OpenAPI 3 document"},
		{"malformed", "paths: [unclosed", "content is not a valid JSON or YAML document"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewParser().Parse(library.FormatOpenAPI, []byte(tt.content))
			var importErr *library.ImportError
			if !errors.As(err, &importErr) {
				t.Fatalf("err = %v, want ImportError", err)
			}
			if importErr.Message != tt.message {
				t.Errorf("message = %q, want %q", importErr.Message, tt.message)
			}
		})
	}
}

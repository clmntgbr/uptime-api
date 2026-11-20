package validation

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
)

type nameDTO struct {
	Name string `json:"name" validate:"required,max=20"`
	Note string `json:"note" validate:"max=20"`
}

// app echoes the (possibly trimmed) name, or returns the validation response.
func app() *fiber.App {
	a := fiber.New(fiber.Config{ErrorHandler: FiberErrorHandler})
	a.Get("/", func(c fiber.Ctx) error {
		d := nameDTO{Name: c.Query("name"), Note: c.Query("note")}
		if err := Struct(c, &d); err != nil {
			return err
		}
		return c.SendString("[" + d.Name + "]")
	})
	return a
}

func get(t *testing.T, a *fiber.App, q url.Values) (int, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/?"+q.Encode(), nil)
	resp, err := a.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body)
}

func TestStruct_TrimsRequiredStrings(t *testing.T) {
	status, body := get(t, app(), url.Values{"name": {"  billing  "}})
	if status != http.StatusOK || body != "[billing]" {
		t.Fatalf("got %d %q, want 200 [billing]", status, body)
	}
}

func TestStruct_RejectsBlankRequired(t *testing.T) {
	status, body := get(t, app(), url.Values{"name": {"   "}})
	if status != http.StatusBadRequest {
		t.Fatalf("status: got %d want 400", status)
	}
	if !strings.Contains(body, `"name"`) {
		t.Fatalf("error must name the field, got %s", body)
	}
}

func TestStruct_DoesNotTrimOptionalFields(t *testing.T) {
	status, _ := get(t, app(), url.Values{"name": {"ok"}, "note": {"   "}})
	if status != http.StatusOK {
		t.Fatalf("status: got %d want 200", status)
	}
}

func TestStruct_RejectsTooLong(t *testing.T) {
	status, _ := get(t, app(), url.Values{"name": {strings.Repeat("x", 21)}})
	if status != http.StatusBadRequest {
		t.Fatalf("status: got %d want 400", status)
	}
}

func TestTrimRequiredStrings_IgnoresNonPointer(t *testing.T) {
	d := nameDTO{Name: " a "}
	trimRequiredStrings(d) // value, not pointer: must not panic nor change it
	if d.Name != " a " {
		t.Fatalf("value copy must be untouched, got %q", d.Name)
	}
}

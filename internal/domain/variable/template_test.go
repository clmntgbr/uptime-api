package variable

import (
	"reflect"
	"testing"
)

func TestReferences(t *testing.T) {
	got := References("{{base_url}}/users/{{ user_id }}?t={{base_url}}&x={{scope.name}}")
	want := []string{"base_url", "user_id"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestRender(t *testing.T) {
	values := map[string]string{"host": "example.com"}
	out, missing := Render("https://{{host}}/{{path}}", func(name string) (string, bool) {
		v, ok := values[name]
		return v, ok
	})
	if out != "https://example.com/{{path}}" {
		t.Fatalf("unexpected output %q", out)
	}
	if !reflect.DeepEqual(missing, []string{"path"}) {
		t.Fatalf("unexpected missing %v", missing)
	}
}

func TestValidateName(t *testing.T) {
	for _, ok := range []string{"token", "_x", "api_key2"} {
		if err := ValidateName(ok); err != nil {
			t.Errorf("%q: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "2fa", "a-b", "run_id", "uuid"} {
		if err := ValidateName(bad); err == nil {
			t.Errorf("%q must be rejected", bad)
		}
	}
}

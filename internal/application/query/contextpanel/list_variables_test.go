package contextpanel

import (
	"strings"
	"testing"

	domainworkflow "go-api/internal/domain/workflow"
)

func TestTruncateContextValue(t *testing.T) {
	short := strings.Repeat("a", domainworkflow.ContextValueMaxChars)
	got, truncated := truncateContextValue(short)
	if truncated || got != short {
		t.Fatalf("short value must stay intact")
	}
	long := strings.Repeat("b", domainworkflow.ContextValueMaxChars+10)
	got, truncated = truncateContextValue(long)
	if !truncated || got != strings.Repeat("b", domainworkflow.ContextValueMaxChars) {
		t.Fatalf("got %q truncated=%v", got, truncated)
	}
}

func TestContextValue_SecretAndMissing(t *testing.T) {
	secret := contextValue(domainworkflow.ContextScopeStatic, "api_key", "secret", true, nil, "")
	if secret.Value != nil || !secret.Secret {
		t.Fatalf("secret must null value: %+v", secret)
	}
	missing := contextValue(
		domainworkflow.ContextScopeWorkflow, "token", "", false,
		&domainworkflow.ContextProducerView{Name: "login"},
		domainworkflow.ContextVarStatusMissing,
	)
	if missing.Value != nil || missing.Status != domainworkflow.ContextVarStatusMissing {
		t.Fatalf("missing must null value: %+v", missing)
	}
}

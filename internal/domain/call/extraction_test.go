package call

import "testing"

func TestNormalizeExtraction_JSONPath(t *testing.T) {
	ok, err := NormalizeExtraction(Extraction{Name: "orderId", Source: ExtractJSON, Expression: "$[0].id"})
	if err != nil || ok.Expression != "$[0].id" {
		t.Fatalf("valid path: %+v %v", ok, err)
	}

	_, err = NormalizeExtraction(Extraction{Name: "orderId", Source: ExtractJSON, Expression: "$.[0].id"})
	if err == nil {
		t.Fatal("$.[0].id must be rejected")
	}
}

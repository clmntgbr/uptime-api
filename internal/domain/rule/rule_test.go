package rule

import "testing"

func TestParse(t *testing.T) {
	cases := []struct {
		text     string
		subject  Subject
		operator Operator
		target   string
		value    string
	}{
		{"status is 200", SubjectStatus, OpIs, "", "200"},
		{"status in 200, 201, 2xx", SubjectStatus, OpIn, "", "200,201,2xx"},
		{"response time exceeds 800 ms", SubjectResponseTime, OpExceeds, "", "800"},
		{"response time exceeds 2 s", SubjectResponseTime, OpExceeds, "", "2000"},
		{"body contains ok", SubjectBody, OpContains, "", "ok"},
		{"body does not contain error", SubjectBody, OpNotContains, "", "error"},
		{"json $.items exists", SubjectJSON, OpExists, "$.items", ""},
		{"json $.status is {{expected}}", SubjectJSON, OpIs, "$.status", "{{expected}}"},
		{"header Content-Type contains json", SubjectHeader, OpContains, "Content-Type", "json"},
		{"size exceeds 64 KB", SubjectSize, OpExceeds, "", "65536"},
		{"certificate expires in less than 14 days", SubjectCertificate, OpExpiresWithin, "", "14"},
		{"certificate chain is invalid", SubjectCertificate, OpChainInvalid, "", ""},
	}
	for _, c := range cases {
		r, err := Parse(c.text)
		if err != nil {
			t.Errorf("%q: %v", c.text, err)
			continue
		}
		if r.Subject != c.subject || r.Operator != c.operator || r.Target != c.target || r.Value != c.value {
			t.Errorf("%q: got %+v", c.text, r)
		}
	}

	for _, text := range []string{"status is 999", "latency is high", "response time exceeds fast", "json items exists"} {
		if _, err := Parse(text); err == nil {
			t.Errorf("%q must be rejected", text)
		}
	}
}

func TestText_RoundTrip(t *testing.T) {
	for _, text := range []string{
		"status is 200",
		"response time exceeds 800 ms",
		"size exceeds 64 KB",
		"json $.count is below 3",
		"certificate expires in less than 14 days",
	} {
		r, err := Parse(text)
		if err != nil {
			t.Fatal(err)
		}
		if got := r.Text(); got != text {
			t.Errorf("got %q want %q", got, text)
		}
	}
}

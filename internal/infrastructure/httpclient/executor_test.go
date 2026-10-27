package httpclient

import (
	"context"
	"crypto/x509"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"go-api/internal/contract"
	"go-api/internal/engine"
)

func TestIsBlocked(t *testing.T) {
	blocked := []string{"127.0.0.1", "10.1.2.3", "172.20.0.1", "192.168.1.1", "169.254.169.254", "100.64.0.1", "::1", "fe80::1", "fd00::1", "::ffff:127.0.0.1", "::ffff:169.254.169.254", "0.0.0.0"}
	for _, ip := range blocked {
		if !IsBlocked(netip.MustParseAddr(ip)) {
			t.Errorf("%s must be blocked", ip)
		}
	}
	for _, ip := range []string{"1.1.1.1", "8.8.8.8", "2606:4700:4700::1111", "172.32.0.1"} {
		if IsBlocked(netip.MustParseAddr(ip)) {
			t.Errorf("%s must be allowed", ip)
		}
	}
}

func request(url string) engine.Request {
	return engine.Request{Method: "GET", URL: url, Timeout: 5 * time.Second, VerifyTLS: true, IPVersion: "any"}
}

func TestSession_BlocksPrivateDestinations(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	_, err := NewExecutor(Config{}).NewSession().Do(context.Background(), request(srv.URL))
	var execErr *engine.ExecError
	if !errors.As(err, &execErr) || execErr.Type != contract.ErrorTypeBlocked {
		t.Fatalf("expected blocked error, got %v", err)
	}

	_, err = NewExecutor(Config{}).NewSession().Do(context.Background(), request("http://localhost:1/"))
	if !errors.As(err, &execErr) || execErr.Type != contract.ErrorTypeBlocked {
		t.Fatalf("a name resolving to loopback must be blocked, got %v", err)
	}
	if strings.Contains(execErr.Message, "127.0.0.1") || strings.Contains(execErr.Message, "::1") {
		t.Fatalf("blocked message must not leak the resolved IP, got %q", execErr.Message)
	}
}

func TestSession_TLSRedirectsCookiesAndTimings(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "sid", Value: "abc", Path: "/"})
		http.Redirect(w, r, "/me", http.StatusFound)
	})
	mux.HandleFunc("/me", func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie("sid")
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"sid":"` + c.Value + `"}`))
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()

	roots := x509.NewCertPool()
	roots.AddCert(srv.Certificate())
	session := NewExecutor(Config{AllowPrivate: true, RootCAs: roots, UserAgent: "uptime-test"}).NewSession()

	resp, err := session.Do(context.Background(), request(srv.URL+"/login"))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != 200 || string(resp.Body) != `{"sid":"abc"}` {
		t.Fatalf("unexpected response %d %s", resp.Status, resp.Body)
	}
	if len(resp.Redirects) != 1 || !strings.HasSuffix(resp.Redirects[0], "/me") {
		t.Fatalf("redirects not recorded: %v", resp.Redirects)
	}
	if resp.TLS == nil || !resp.TLS.ChainValid || !resp.TLS.NameMatch || resp.TLS.ExpiresAt.IsZero() || resp.TLS.Port == 0 {
		t.Fatalf("unexpected TLS info %+v", resp.TLS)
	}
	if resp.IP != "127.0.0.1" || resp.IPVersion != "ipv4" || resp.Timings.Total < resp.Timings.TTFB {
		t.Fatalf("unexpected network info %+v %+v", resp.IP, resp.Timings)
	}

	again, err := session.Do(context.Background(), request(srv.URL+"/me"))
	if err != nil || again.Status != 200 {
		t.Fatalf("cookie must be kept in the session: %v %v", again, err)
	}
	other, err := NewExecutor(Config{AllowPrivate: true, RootCAs: roots}).NewSession().Do(context.Background(), request(srv.URL+"/me"))
	if err != nil || other.Status != 401 {
		t.Fatalf("a new session must not share cookies: %v %v", other, err)
	}
}

func TestSession_InvalidCertificate(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	exec := NewExecutor(Config{AllowPrivate: true})

	_, err := exec.NewSession().Do(context.Background(), request(srv.URL))
	var execErr *engine.ExecError
	if !errors.As(err, &execErr) || execErr.Type != contract.ErrorTypeTLS {
		t.Fatalf("expected tls error, got %v", err)
	}
	if execErr.Partial == nil || execErr.Partial.TLS == nil || execErr.Partial.TLS.ChainValid {
		t.Fatal("certificate must still be reported")
	}

	insecure := request(srv.URL)
	insecure.VerifyTLS = false
	resp, err := exec.NewSession().Do(context.Background(), insecure)
	if err != nil || resp.TLS == nil || resp.TLS.ChainValid {
		t.Fatalf("verification off must accept and report the certificate: %v %+v", err, resp)
	}
}

func TestSession_LimitsRedirectsAndBody(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/loop", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/loop", http.StatusFound) })
	mux.HandleFunc("/big", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(make([]byte, 2048)) })
	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) { time.Sleep(500 * time.Millisecond) })
	srv := httptest.NewServer(mux)
	defer srv.Close()
	exec := NewExecutor(Config{AllowPrivate: true, MaxBodyBytes: 1024})

	var execErr *engine.ExecError
	if _, err := exec.NewSession().Do(context.Background(), request(srv.URL+"/loop")); !errors.As(err, &execErr) {
		t.Fatalf("redirect loop must fail, got %v", err)
	}
	if _, err := exec.NewSession().Do(context.Background(), request(srv.URL+"/big")); !errors.As(err, &execErr) || execErr.Type != contract.ErrorTypeBodyTooLarge {
		t.Fatalf("expected body_too_large, got %v", err)
	}
	slow := request(srv.URL + "/slow")
	slow.Timeout = 100 * time.Millisecond
	if _, err := exec.NewSession().Do(context.Background(), slow); !errors.As(err, &execErr) || execErr.Type != contract.ErrorTypeTimeout {
		t.Fatalf("expected timeout, got %v", err)
	}
}

func TestSession_RejectsHTTPSToHTTPRedirect(t *testing.T) {
	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer httpSrv.Close()

	tlsMux := http.NewServeMux()
	tlsMux.HandleFunc("/down", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, httpSrv.URL+"/ok", http.StatusFound)
	})
	tlsSrv := httptest.NewTLSServer(tlsMux)
	defer tlsSrv.Close()

	roots := x509.NewCertPool()
	roots.AddCert(tlsSrv.Certificate())
	_, err := NewExecutor(Config{AllowPrivate: true, RootCAs: roots}).NewSession().Do(
		context.Background(), request(tlsSrv.URL+"/down"),
	)
	var execErr *engine.ExecError
	if !errors.As(err, &execErr) || execErr.Type != contract.ErrorTypeInvalid {
		t.Fatalf("https→http redirect must be rejected, got %v", err)
	}
}

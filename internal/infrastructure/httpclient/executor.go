// Package httpclient is the engine HTTP executor: SSRF-safe dialing, httptrace
// timings, TLS chain inspection, bounded body reads.
package httpclient

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptrace"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"go-api/internal/contract"
	"go-api/internal/engine"
)

type Config struct {
	UserAgent    string
	AllowPrivate bool
	// MaxBodyBytes is the hard read limit (body_too_large). Defaults to engine.MaxBodyBytes (50 MiB).
	MaxBodyBytes int64
	MaxRedirects int
	// RootCAs overrides the system roots (tests).
	RootCAs *x509.CertPool
}

type Executor struct {
	cfg Config
}

func NewExecutor(cfg Config) *Executor {
	if cfg.MaxBodyBytes <= 0 {
		cfg.MaxBodyBytes = engine.MaxBodyBytes
	}
	if cfg.MaxRedirects <= 0 {
		cfg.MaxRedirects = engine.MaxRedirects
	}
	return &Executor{cfg: cfg}
}

var _ engine.Executor = (*Executor)(nil)

func (e *Executor) NewSession() engine.Session {
	jar, _ := cookiejar.New(nil)
	return &session{cfg: e.cfg, jar: jar}
}

type session struct {
	cfg Config
	jar http.CookieJar
}

// recorder collects what the transport observed during one call.
type recorder struct {
	mu        sync.Mutex
	start     time.Time
	dnsStart  time.Time
	dns       time.Duration
	connStart time.Time
	connect   time.Duration
	tlsStart  time.Time
	tlsDur    time.Duration
	firstByte time.Time
	ip        netip.Addr
	host      string
	tls       *contract.TLSInfo
	redirects []string
}

func (r *recorder) trace() *httptrace.ClientTrace {
	return &httptrace.ClientTrace{
		DNSStart: func(httptrace.DNSStartInfo) { r.mu.Lock(); r.dnsStart = time.Now(); r.mu.Unlock() },
		DNSDone: func(httptrace.DNSDoneInfo) {
			r.mu.Lock()
			if !r.dnsStart.IsZero() {
				r.dns += time.Since(r.dnsStart)
			}
			r.mu.Unlock()
		},
		ConnectStart: func(string, string) { r.mu.Lock(); r.connStart = time.Now(); r.mu.Unlock() },
		ConnectDone: func(string, string, error) {
			r.mu.Lock()
			if !r.connStart.IsZero() {
				r.connect += time.Since(r.connStart)
			}
			r.mu.Unlock()
		},
		TLSHandshakeStart: func() { r.mu.Lock(); r.tlsStart = time.Now(); r.mu.Unlock() },
		TLSHandshakeDone: func(tls.ConnectionState, error) {
			r.mu.Lock()
			if !r.tlsStart.IsZero() {
				r.tlsDur += time.Since(r.tlsStart)
			}
			r.mu.Unlock()
		},
		GotFirstResponseByte: func() { r.mu.Lock(); r.firstByte = time.Now(); r.mu.Unlock() },
	}
}

func (r *recorder) timings(end time.Time) contract.Timings {
	r.mu.Lock()
	defer r.mu.Unlock()
	t := contract.Timings{
		DNS:     r.dns.Milliseconds(),
		Connect: r.connect.Milliseconds(),
		TLS:     r.tlsDur.Milliseconds(),
		Total:   end.Sub(r.start).Milliseconds(),
	}
	if !r.firstByte.IsZero() {
		t.TTFB = r.firstByte.Sub(r.start).Milliseconds()
		t.Transfer = end.Sub(r.firstByte).Milliseconds()
	}
	return t
}

func (r *recorder) partial(end time.Time) *engine.Response {
	resp := &engine.Response{Timings: r.timings(end), Redirects: r.redirects}
	r.mu.Lock()
	resp.TLS = r.tls
	if r.ip.IsValid() {
		resp.IP = r.ip.String()
		resp.IPVersion = ipVersionOf(r.ip)
	}
	r.mu.Unlock()
	return resp
}

var errCertificate = errors.New("certificate verification failed")

func (s *session) Do(ctx context.Context, req engine.Request) (*engine.Response, error) {
	rec := &recorder{}
	ctx, cancel := context.WithTimeout(ctx, req.Timeout)
	defer cancel()

	transport := &http.Transport{
		DialContext: newDialer(s.cfg.AllowPrivate, req.IPVersion, req.Timeout, func(host string, ip netip.Addr) {
			rec.mu.Lock()
			rec.ip = ip
			rec.host = host
			rec.mu.Unlock()
		}).DialContext,
		ForceAttemptHTTP2:      true,
		DisableKeepAlives:      true,
		TLSHandshakeTimeout:    req.Timeout,
		ResponseHeaderTimeout:  req.Timeout,
		MaxResponseHeaderBytes: 64 * 1024,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS10,
			// Verification is done in VerifyConnection, so the chain is always
			// read and reported, even when the call accepts invalid certificates.
			InsecureSkipVerify: true,
			VerifyConnection: func(cs tls.ConnectionState) error {
				rec.mu.Lock()
				host := rec.host
				rec.mu.Unlock()
				if cs.ServerName != "" {
					host = cs.ServerName
				}
				info := inspectTLS(cs, host, s.cfg.RootCAs)
				rec.mu.Lock()
				rec.tls = info
				rec.mu.Unlock()
				if req.VerifyTLS && (!info.ChainValid || !info.NameMatch) {
					return fmt.Errorf("%w: %s", errCertificate, info.VerifyError)
				}
				return nil
			},
		},
	}
	if req.ProxyURL != "" {
		proxy, err := url.Parse(req.ProxyURL)
		if err != nil {
			return nil, &engine.ExecError{Type: contract.ErrorTypeInvalid, Message: "invalid proxy url"}
		}
		transport.Proxy = http.ProxyURL(proxy)
	}
	defer transport.CloseIdleConnections()

	client := &http.Client{
		Transport: transport,
		Jar:       s.jar,
		CheckRedirect: func(next *http.Request, via []*http.Request) error {
			if len(via) > s.cfg.MaxRedirects {
				return fmt.Errorf("stopped after %d redirects", s.cfg.MaxRedirects)
			}
			if next.URL.Scheme != "http" && next.URL.Scheme != "https" {
				return fmt.Errorf("redirect to unsupported scheme %q", next.URL.Scheme)
			}
			// Forbid TLS downgrade: https → http.
			if len(via) > 0 && via[len(via)-1].URL.Scheme == "https" && next.URL.Scheme == "http" {
				return fmt.Errorf("redirect to insecure scheme rejected")
			}
			rec.redirects = append(rec.redirects, next.URL.String())
			return nil
		},
	}

	var reqBody io.Reader
	if req.Body != "" {
		reqBody = strings.NewReader(req.Body)
	}
	httpReq, err := http.NewRequestWithContext(httptrace.WithClientTrace(ctx, rec.trace()), req.Method, req.URL, reqBody)
	if err != nil {
		return nil, &engine.ExecError{Type: contract.ErrorTypeInvalid, Message: "invalid request"}
	}
	for k, v := range req.Headers {
		if strings.EqualFold(k, "Host") {
			httpReq.Host = v
			continue
		}
		httpReq.Header.Set(k, v)
	}
	if httpReq.Header.Get("User-Agent") == "" && s.cfg.UserAgent != "" {
		httpReq.Header.Set("User-Agent", s.cfg.UserAgent)
	}

	rec.start = time.Now()
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, classify(err, rec.partial(time.Now()))
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, s.cfg.MaxBodyBytes+1))
	end := time.Now()
	if err != nil {
		return nil, classify(err, rec.partial(end))
	}
	if int64(len(data)) > s.cfg.MaxBodyBytes {
		return nil, &engine.ExecError{
			Type:    contract.ErrorTypeBodyTooLarge,
			Message: "response body exceeds " + strconv.FormatInt(s.cfg.MaxBodyBytes/(1024*1024), 10) + " MB",
			Partial: rec.partial(end),
		}
	}

	out := rec.partial(end)
	if out.TLS != nil {
		out.TLS.Port = portOf(resp.Request.URL)
	}
	out.Status = resp.StatusCode
	out.Protocol = resp.Proto
	out.Headers = resp.Header
	out.Body = data
	out.Compression = resp.Header.Get("Content-Encoding")
	if resp.Uncompressed {
		out.Compression = "gzip"
	}
	return out, nil
}

func classify(err error, partial *engine.Response) *engine.ExecError {
	e := &engine.ExecError{Type: contract.ErrorTypeOther, Message: err.Error(), Partial: partial}
	var dnsErr *net.DNSError
	var certErr *tls.CertificateVerificationError
	var recordErr tls.RecordHeaderError
	var netErr net.Error
	switch {
	case errors.Is(err, ErrBlocked):
		e.Type = contract.ErrorTypeBlocked
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &netErr) && netErr.Timeout():
		e.Type = contract.ErrorTypeTimeout
	case errors.As(err, &dnsErr):
		e.Type = contract.ErrorTypeDNS
	case errors.Is(err, errCertificate), errors.As(err, &certErr), errors.As(err, &recordErr),
		strings.Contains(err.Error(), "tls:"):
		e.Type = contract.ErrorTypeTLS
	case strings.Contains(err.Error(), "redirect"):
		e.Type = contract.ErrorTypeInvalid
	default:
		var opErr *net.OpError
		if errors.As(err, &opErr) {
			e.Type = contract.ErrorTypeConnect
		}
	}
	return e
}

func inspectTLS(cs tls.ConnectionState, host string, roots *x509.CertPool) *contract.TLSInfo {
	info := &contract.TLSInfo{
		Host:        host,
		Version:     tls.VersionName(cs.Version),
		CipherSuite: tls.CipherSuiteName(cs.CipherSuite),
	}
	if len(cs.PeerCertificates) == 0 {
		info.VerifyError = "no certificate presented"
		return info
	}
	leaf := cs.PeerCertificates[0]
	info.Subject = leaf.Subject.String()
	info.Issuer = leaf.Issuer.String()
	info.SerialNumber = leaf.SerialNumber.String()
	info.ExpiresAt = leaf.NotAfter.UTC()

	intermediates := x509.NewCertPool()
	for _, c := range cs.PeerCertificates[1:] {
		intermediates.AddCert(c)
	}
	chains, err := leaf.Verify(x509.VerifyOptions{Intermediates: intermediates, Roots: roots})
	chain := cs.PeerCertificates
	if err == nil && len(chains) > 0 {
		info.ChainValid = true
		chain = chains[0]
	} else if err != nil {
		info.VerifyError = err.Error()
	}
	if err := leaf.VerifyHostname(host); err == nil {
		info.NameMatch = true
	} else if info.VerifyError == "" {
		info.VerifyError = err.Error()
	}

	earliest := leaf.NotAfter
	for _, c := range chain {
		if c.NotAfter.Before(earliest) {
			earliest = c.NotAfter
		}
		info.Chain = append(info.Chain, contract.TLSCertificate{
			Subject: c.Subject.String(), Issuer: c.Issuer.String(), NotAfter: c.NotAfter.UTC(),
		})
	}
	info.ChainExpiresAt = earliest.UTC()
	return info
}

func ipVersionOf(ip netip.Addr) string {
	if ip.Is4() {
		return "ipv4"
	}
	return "ipv6"
}

func portOf(u *url.URL) int {
	if p, err := strconv.Atoi(u.Port()); err == nil {
		return p
	}
	if u.Scheme == "http" {
		return 80
	}
	return 443
}

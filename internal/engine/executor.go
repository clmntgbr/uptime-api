// Package engine runs a workflow job: variable resolution, HTTP calls through
// an Executor, extraction, rule evaluation and secret masking. It has no I/O of
// its own besides the Executor and optional BodyStore it is given.
package engine

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"go-api/internal/contract"
)

const (
	MaxRedirects = 5
	// MaxBodyKeysBytes is the largest body for which response_keys are collected.
	// Larger bodies still succeed and are claim-checked to object storage; keys are skipped.
	MaxBodyKeysBytes = 5 * 1024 * 1024
	// MaxBodyBytes is the most the executor reads from a response; beyond, the call fails with body_too_large.
	MaxBodyBytes = 50 * 1024 * 1024
)

// Executor creates one Session per run, so cookies are kept between the calls of a run only.
type Executor interface {
	NewSession() Session
}

type Session interface {
	Do(ctx context.Context, req Request) (*Response, error)
}

type Request struct {
	Method    string
	URL       string
	Headers   map[string]string
	Body      string
	Timeout   time.Duration
	VerifyTLS bool
	IPVersion string
	ProxyURL  string
}

type Response struct {
	Status      int
	Protocol    string
	Headers     http.Header
	Body        []byte
	Compression string
	Timings     contract.Timings
	IP          string
	IPVersion   string
	Redirects   []string
	TLS         *contract.TLSInfo
}

// ExecError is a classified failure: the call could not be executed.
type ExecError struct {
	Type    string
	Message string
	// Partial holds what was measured before the failure (timings, IP, TLS), when any.
	Partial *Response
}

func (e *ExecError) Error() string { return fmt.Sprintf("%s: %s", e.Type, e.Message) }

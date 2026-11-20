//go:build integration

package persistence_test

import "time"

// testNow is the fixed clock used by the integration tests.
var testNow = time.Date(2026, 11, 20, 9, 0, 0, 0, time.UTC)

package expirytest

import "time"

// testNow is the fixed clock used by this package's tests.
var testNow = time.Date(2026, 11, 20, 9, 0, 0, 0, time.UTC)

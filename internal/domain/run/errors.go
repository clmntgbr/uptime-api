package run

import "errors"

// ErrRunNotRunning means a late message targets a run that already finished.
// Handlers treat it as a no-op: the run's final state must not change.
var ErrRunNotRunning = errors.New("run is not running")

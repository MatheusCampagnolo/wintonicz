// Package privilege answers one question (are we elevated?) and performs one
// action (restart this program elevated, once). It never elevates anything else.
package privilege

import "errors"

// ErrCancelled means the user declined the UAC prompt. It is not a failure.
var ErrCancelled = errors.New("administrator approval was cancelled")

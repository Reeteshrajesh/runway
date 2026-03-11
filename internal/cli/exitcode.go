package cli

import "errors"

// ExitError is an error that carries a specific OS exit code.
// Commands return ExitError when a distinct exit code matters for scripting.
type ExitError struct {
	Code int
	Err  error
}

func (e *ExitError) Error() string { return e.Err.Error() }
func (e *ExitError) Unwrap() error { return e.Err }

// Exit codes used by runway. Documented in the man page and README.
const (
	ExitOK            = 0 // success
	ExitGeneralError  = 1 // unknown / unclassified error
	ExitLockHeld      = 2 // another deploy is in progress
	ExitBuildFailed   = 3 // setup or build step exited non-zero
	ExitStartFailed   = 4 // service failed to start (auto-rollback may have triggered)
	ExitGitError      = 5 // clone / checkout failed
	ExitManifestError = 6 // manifest.yml is invalid
	ExitNotFound      = 7 // rollback target release does not exist
)

// exitErr wraps an error with a specific exit code.
func exitErr(code int, err error) *ExitError {
	return &ExitError{Code: code, Err: err}
}

// ExitCode extracts the exit code from err.
// Returns ExitGeneralError (1) if err is not an ExitError.
func ExitCode(err error) int {
	var e *ExitError
	if errors.As(err, &e) {
		return e.Code
	}
	return ExitGeneralError
}

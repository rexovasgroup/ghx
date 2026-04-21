package gherrs

type ExitCode int

const (
	exitError   ExitCode = 1
	exitCancel  ExitCode = 2
	exitAuth    ExitCode = 4
	exitPending ExitCode = 8
)

type ExitCoder interface {
	error
	ExitCode() ExitCode
}

type Silenced interface {
	error
	silent()
}

var SilentError = silentError{}

type silentError struct{}

func (e silentError) Error() string {
	return "silent"
}

func (e silentError) ExitCode() ExitCode {
	return exitError
}

func (e silentError) silent() {}

var PendingError = pendingError{}

type pendingError struct{}

func (e pendingError) Error() string {
	return "pending"
}

func (e pendingError) ExitCode() ExitCode {
	return exitPending
}

func (e pendingError) silent() {}

var UserCancellationError = userCancellationError{}

type userCancellationError struct{}

func (e userCancellationError) Error() string {
	return "user cancellation"
}

func (e userCancellationError) ExitCode() ExitCode {
	return exitCancel
}

func (e userCancellationError) silent() {}

var AuthError = authError{}

type authError struct{}

func (e authError) Error() string {
	return "authentication error"
}

func (e authError) ExitCode() ExitCode {
	return exitAuth
}

func (e authError) silent() {}

type ExtensionExecError struct {
	Code int
}

func (e ExtensionExecError) Error() string {
	return "extension execution error"
}

func (e ExtensionExecError) ExitCode() ExitCode {
	return ExitCode(e.Code)
}

func (e ExtensionExecError) silent() {}

type GeneralError struct {
	WrappedErr error
	Message    string
}

func (e GeneralError) Error() string {
	switch {
	case e.WrappedErr != nil && e.Message != "":
		return e.WrappedErr.Error() + "\n" + e.Message
	case e.WrappedErr != nil:
		return e.WrappedErr.Error()
	default:
		return e.Message
	}
}

func (e GeneralError) Unwrap() error {
	return e.WrappedErr
}

func (e GeneralError) ExitCode() ExitCode {
	return exitError
}

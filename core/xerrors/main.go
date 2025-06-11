package xerrors

var (
	ObjectNotFound = newIndexedError("object not found", 2)
)

type indexedError struct {
	message  string
	exitCode int
}

// Error implements the error interface.
func (e *indexedError) Error() string {
	return e.message
}

// ExitCode returns the exit code associated with the error.
func (e *indexedError) ExitCode() int {
	return e.exitCode
}

// NewCustomError creates a new CustomError with the given message and exit code.
func newIndexedError(message string, exitCode int) error {
	return &indexedError{
		message:  message,
		exitCode: exitCode,
	}
}

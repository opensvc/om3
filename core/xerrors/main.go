package xerrors

var (
	ExitCodeObjectNotFound             = 2
	ExitCodeInstanceNotFound           = 3
	ExitCodeInstanceActionNotSupported = 4
	ExitCodeResizeNoSuchStage          = 5

	ObjectNotFound             = newIndexedError("object not found", ExitCodeObjectNotFound)
	InstanceNotFound           = newIndexedError("instance not found", ExitCodeInstanceNotFound)
	InstanceActionNotSupported = newIndexedError("instance action not supported", ExitCodeInstanceActionNotSupported)

	// ResizeNoSuchStage says the chain has no stage of that number to run
	// here. It is how a node driving a resize learns it has reached the end
	// of the chain, which no one can tell it: the stages are read from the
	// chain, and only the node walking it knows how many there are.
	ResizeNoSuchStage = newIndexedError("no such stage", ExitCodeResizeNoSuchStage)
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

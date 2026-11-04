package xerrors

var (
	ExitCodeObjectNotFound             = 2
	ExitCodeInstanceNotFound           = 3
	ExitCodeInstanceActionNotSupported = 4
	ExitCodeResizeNoSuchStage          = 5
	ExitCodeNothingToCollect           = 6
	ExitCodeInstanceDisabled           = 7

	ObjectNotFound             = newIndexedError("object not found", ExitCodeObjectNotFound)
	InstanceNotFound           = newIndexedError("instance not found", ExitCodeInstanceNotFound)
	InstanceActionNotSupported = newIndexedError("instance action not supported", ExitCodeInstanceActionNotSupported)

	// ResizeNoSuchStage says the chain has no stage of that number to run
	// here. It is how a node driving a resize learns it has reached the end
	// of the chain, which no one can tell it: the stages are read from the
	// chain, and only the node walking it knows how many there are.
	ResizeNoSuchStage = newIndexedError("no such stage", ExitCodeResizeNoSuchStage)

	// NothingToCollect says a push has nothing to collect on the node, as
	// the stats push on a node without sysstat: the push is not run, which
	// is no failure of the scheduled push, but a user running it is told.
	NothingToCollect = newIndexedError("nothing to collect", ExitCodeNothingToCollect)

	// InstanceDisabled says the instance is disabled: the action did
	// nothing, as the disable keyword says, which is no failure of the
	// action, but not its success either.
	InstanceDisabled = newIndexedError("object instance is disabled", ExitCodeInstanceDisabled)
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

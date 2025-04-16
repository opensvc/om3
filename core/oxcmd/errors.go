package oxcmd

import "errors"

var (
	ErrFlagInvalid = errors.New("invalid command flag")

	ErrPrint = errors.New("print")

	ErrClientStatusCode = errors.New("client request unexpected status code")

	ErrEventKindUnexpected = errors.New("unexpected event kind")

	ErrFetchFile = errors.New("fetch file")

	ErrInstallFile = errors.New("install file")
)

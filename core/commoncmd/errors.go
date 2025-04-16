package commoncmd

import "errors"

var (
	ErrClientRequest = errors.New("client request")

	ErrClientStatusCode = errors.New("client request unexpected status code")

	ErrEventKindUnexpected = errors.New("unexpected event kind")

	ErrFetchFile = errors.New("fetch file")

	ErrFlagInvalid = errors.New("invalid command flag")

	ErrInstallFile = errors.New("install file")

	ErrPrint = errors.New("print")
)

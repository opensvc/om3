package commands

import "errors"

var (
	ErrFlagInvalid = errors.New("invalid command flag")

	ErrPrint = errors.New("print")

	ErrClientRequest = errors.New("client request")

	ErrClientStatusCode = errors.New("client request unexpected status code")
)

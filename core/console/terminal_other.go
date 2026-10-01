//go:build !unix

package console

import (
	"context"
	"os"
)

func startTerminal(_ context.Context, _ *os.File, _ *wsWriter) (func(), error) {
	return nil, ErrNoTerminal
}

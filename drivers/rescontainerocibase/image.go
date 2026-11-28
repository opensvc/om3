package rescontainerocibase

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// engineOutput runs an engine command and returns its stdout, as
// exec.Cmd.Output does: the error of a command that failed carries what it
// wrote on its stderr.
type engineOutput func(ctx context.Context, a ...string) ([]byte, error)

// imageNotFoundMessages are what the engines say of an image their store
// does not hold, lowercased: docker "Error response from daemon: No such
// image: foo:latest", podman "Error: foo: image not known".
var imageNotFoundMessages = []string{
	"no such image",
	"image not known",
}

// IsImageNotFound says whether message, the stderr of an image inspect, is
// an engine telling its store does not hold the image.
func IsImageNotFound(message string) bool {
	message = strings.ToLower(message)
	for _, s := range imageNotFoundMessages {
		if strings.Contains(message, s) {
			return true
		}
	}
	return false
}

// hasImage says whether the store of an engine holds an image, and its id,
// from the image inspect command inspectArgs.
//
// An inspect fails the same way on a missing image and on an engine that
// does not answer, as a docker whose daemon is down: both exit 1. They are
// told apart by what the engine says, then by asking the engine its version
// when it says something else: an engine that does not answer is an error,
// which a start reports rather than trying a pull bound to fail, and which
// a prefetch retries later. A message not known from an engine that answers
// is a missing image, so that a wording changed by an engine release never
// blocks a start.
func hasImage(ctx context.Context, run engineOutput, inspectArgs []string) (bool, string, error) {
	b, err := run(ctx, inspectArgs...)
	if err == nil {
		return true, strings.TrimSuffix(string(b), "\n"), nil
	}
	if ctx != nil && ctx.Err() != nil {
		return false, "", ctx.Err()
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		// The command did not run: no engine installed, or no user to
		// run it as. Asking the version would fail the same.
		return false, "", err
	}
	message := engineErrorMessage(err)
	if IsImageNotFound(message) {
		return false, "", nil
	}
	if _, versionErr := run(ctx, "version"); versionErr != nil {
		if ctx != nil && ctx.Err() != nil {
			return false, "", ctx.Err()
		}
		if message == "" {
			message = engineErrorMessage(versionErr)
		}
		return false, "", fmt.Errorf("the engine does not answer: %s", message)
	}
	return false, "", nil
}

// engineErrorMessage is what a failed engine command said: the last line of
// its stderr, where the engines write their error after their warnings, and
// the error of the execution when it wrote none.
func engineErrorMessage(err error) string {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		lines := strings.Split(strings.TrimSpace(string(exitErr.Stderr)), "\n")
		if s := strings.TrimSpace(lines[len(lines)-1]); s != "" {
			return s
		}
	}
	return err.Error()
}

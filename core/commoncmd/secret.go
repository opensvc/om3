package commoncmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/opensvc/om3/v3/core/credential"
	"github.com/opensvc/om3/v3/core/env"
)

// SecretFromFileOrEnv returns the secret held by the file at filename, or the
// value of the envVar environment variable when filename is empty, or an
// empty string when neither is set.
//
// A secret never reaches a command as a flag value: a command line is
// readable by any user through the process table, and stays in the shell
// history. So the flags naming one, --token and --credential, name a file,
// and the environment is what the daemon uses to hand a secret to a command
// it forks.
//
// The file wins over the environment, so that an operator naming a file is
// told about a bad path instead of silently getting an inherited value.
func SecretFromFileOrEnv(filename, envVar string) (string, error) {
	if filename == "" {
		return strings.TrimSpace(os.Getenv(envVar)), nil
	}
	b, err := os.ReadFile(filename)
	if err != nil {
		return "", err
	}
	s := strings.TrimSpace(string(b))
	if s == "" {
		return "", fmt.Errorf("file %s is empty", filename)
	}
	return s, nil
}

// CollectorCredential returns the collector user and password held by the
// file at filename, or by the OSVC_COLLECTOR_CREDENTIAL environment variable
// when filename is empty, and two empty strings when neither is set: a node
// then registers with the id it already holds.
//
// It is parsed where the command is typed rather than where the registration
// happens, so that a malformed credential is reported once, before any node
// is asked to register.
func CollectorCredential(filename string) (user, password string, err error) {
	s, err := SecretFromFileOrEnv(filename, env.CollectorCredentialVar)
	if err != nil {
		return "", "", fmt.Errorf("%w: --credential: %w", ErrFlagInvalid, err)
	}
	if s == "" {
		return "", "", nil
	}
	user, password, err = credential.Parse(s)
	if err != nil {
		return "", "", fmt.Errorf("%w: --credential: %w", ErrFlagInvalid, err)
	}
	return user, password, nil
}

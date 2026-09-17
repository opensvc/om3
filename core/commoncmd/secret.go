package commoncmd

import (
	"fmt"
	"os"
	"strings"
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

// Package hp3par holds what the drivers of an HPE 3PAR array have in
// common: how a command line is built for the array, and how it is run.
//
// The array driver and the disk driver reach the same arrays, and there is one
// way of reaching them. The shapes here are the ones the disk driver has been
// run with against real hardware on recent firmware, which is why they are
// here rather than in each driver.
package hp3par

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/rs/zerolog"

	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/plog"
)

type (
	// Config is how one array is reached.
	//
	// The paths are resolved: a key or a password file named in the
	// configuration as a reference to a datastore is materialised by the
	// caller, which knows what the reference is relative to.
	Config struct {
		// Method is ssh or cli.
		Method string

		// Manager is the array, by name or address.
		Manager string

		// Username is who to log in as, over ssh.
		Username string

		// KeyFile is the ssh private key to log in with.
		KeyFile string

		// CLI is the 3par cli binary, for the cli method.
		CLI string

		// PWFile is the password file the cli authenticates with.
		PWFile string
	}
)

const (
	// MethodSSH runs the command on the array, over ssh.
	MethodSSH = "ssh"

	// MethodCLI runs the command through the 3par cli installed here.
	MethodCLI = "cli"
)

// ErrBuildCommand is raised when the configuration does not say enough to
// reach the array.
var ErrBuildCommand = errors.New("build command")

// Command returns the command line reaching the array, the program first.
func (t Config) Command(cmd string) ([]string, error) {
	switch t.Method {
	case MethodSSH:
		return t.sshCommand(cmd)
	case MethodCLI:
		return t.cliCommand(cmd)
	case "":
		return nil, fmt.Errorf("%w: the method keyword is required: %s or %s", ErrBuildCommand, MethodSSH, MethodCLI)
	default:
		return nil, fmt.Errorf("%w: unknown method %s", ErrBuildCommand, t.Method)
	}
}

// sshCommand returns "ssh [-i key] [user@]manager <command>".
//
// The command is an argument rather than a session written on the standard
// input: the array runs it and closes, which is what a recent firmware
// answers to.
func (t Config) sshCommand(cmd string) ([]string, error) {
	if t.Manager == "" {
		return nil, fmt.Errorf("%w: the manager keyword is required", ErrBuildCommand)
	}
	args := []string{"ssh"}
	if t.KeyFile != "" {
		args = append(args, "-i", t.KeyFile)
	}
	if t.Username != "" {
		args = append(args, t.Username+"@"+t.Manager)
	} else {
		args = append(args, t.Manager)
	}
	return append(args, cmd), nil
}

// cliCommand returns "<cli> -sys <manager> [-pwf <pwf>] <command>".
//
// The password file is named on the command line rather than left in the
// environment, and the array is named by its manager.
func (t Config) cliCommand(cmd string) ([]string, error) {
	if t.Manager == "" {
		return nil, fmt.Errorf("%w: the manager keyword is required", ErrBuildCommand)
	}
	cli := t.CLI
	if cli == "" {
		cli = "cli"
	}
	args := []string{cli, "-sys", t.Manager}
	if t.PWFile != "" {
		args = append(args, "-pwf", t.PWFile)
	}
	return append(args, strings.Fields(cmd)...), nil
}

// Run runs one command on the array and returns what it printed.
func (t Config) Run(ctx context.Context, log *plog.Logger, cmd string) (string, error) {
	args, err := t.Command(cmd)
	if err != nil {
		return "", err
	}
	if len(args) < 2 {
		return "", fmt.Errorf("%w: %s", ErrBuildCommand, cmd)
	}
	c := command.New(
		command.WithContext(ctx),
		command.WithName(args[0]),
		command.WithArgs(args[1:]),
		command.WithLogger(log),
		command.WithCommandLogLevel(zerolog.DebugLevel),
		command.WithBufferedStdout(),
		command.WithStderrLogLevel(zerolog.TraceLevel),
	)
	b, err := c.Output()
	if err != nil {
		return Clean(string(b)), err
	}
	return Clean(string(b)), nil
}

// Clean removes the carriage returns and the trailing blank lines of what the
// array printed.
func Clean(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.TrimRight(s, "\n ")
}

// ParseCSV reads what the array printed as one map per line, named by the
// columns that were asked for.
//
// A line holding fewer values than there are columns is read as far as it
// goes, and a line holding one value is dropped: the cli prints lines the
// columns do not describe.
func ParseCSV(s string, cols []string) []map[string]string {
	l := make([]map[string]string, 0)
	for _, line := range strings.Split(s, "\n") {
		values := strings.Split(strings.TrimSpace(line), ",")
		if len(values) < 2 {
			continue
		}
		m := make(map[string]string, len(cols))
		for i, col := range cols {
			if i >= len(values) {
				break
			}
			m[col] = values[i]
		}
		l = append(l, m)
	}
	return l
}

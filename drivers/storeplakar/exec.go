package storeplakar

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/rs/zerolog"

	"github.com/opensvc/om3/v3/util/capabilities"
	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/funcopt"
)

const (
	// alias is the name stores.yml gives the store, alone in the
	// configuration directory of the store.
	alias = "@store"

	// unresolvedMessage is what plakar says when stores.yml does not name
	// the store: the configuration is written again and the call retried
	// once.
	unresolvedMessage = "could not resolve repository"

	// reasonMax bounds, in bytes, the reason of a failure quoted from the
	// standard output of plakar: a line as long as the error of the
	// command, which may name a path or quote a server.
	reasonMax = 1024
)

var (
	errPlakarNotInstalled = errors.New("plakar is not installed (rescan capabilities after install)")

	// getPath is the capabilities lookup of the plakar binary, replaced by
	// the tests.
	getPath = capabilities.GetPath
)

// commandAt builds one plakar call against the store alias, resolved
// through stores.yml with its passphrase_cmd and the options of its
// credentials, in the environment of the store. The output goes raw to w when
// given, else to the buffer Stdout returns, line by line in the log.
func (t *T) commandAt(ctx context.Context, level zerolog.Level, w io.Writer, verb string, args ...string) (*command.T, error) {
	p := getPath("plakar")
	if p == "" {
		return nil, errPlakarNotInstalled
	}
	d, err := t.dirs()
	if err != nil {
		return nil, err
	}
	argv := []string{
		"-configdir", d.config,
		"-cachedir", d.cache,
		"-datadir", d.data,
		"at", alias, verb,
	}
	argv = append(argv, args...)
	opts := []funcopt.O{
		command.WithName(p),
		command.WithArgs(argv),
		command.WithContext(ctx),
		command.WithProcessGroup(),
		command.WithLogger(t.Log()),
		command.WithCommandLogLevel(level),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
		command.WithBufferedStderr(),
		command.WithEnv(t.env(d)),
	}
	var cmd *command.T
	if w == nil {
		cmd = command.New(append(opts,
			command.WithStdoutLogLevel(zerolog.TraceLevel),
			command.WithBufferedStdout(),
		)...)
	} else {
		cmd = command.New(opts...)
		// Start leaves Stdout alone when nothing asks to log or buffer it.
		cmd.Cmd().Stdout = w
	}
	return cmd, nil
}

// run runs one plakar verb and returns what it printed. A store plakar
// cannot resolve gets its stores.yml written again and one retry. A
// location plakar answers holds no repository is an errNoRepository, and a
// call exiting 0 without running the verb is a failure, see notRunError.
func (t *T) run(ctx context.Context, level zerolog.Level, verb string, args ...string) ([]byte, error) {
	cmd, err := t.commandAt(ctx, level, nil, verb, args...)
	if err != nil {
		return nil, err
	}
	if err := cmd.Run(); err != nil {
		if !strings.Contains(string(cmd.Stderr()), unresolvedMessage) {
			return nil, t.storeError(verb, cmd, err)
		}
		t.Log().Infof("plakar %s: store unresolved, write stores.yml again and retry", verb)
		if _, err := t.configure(true); err != nil {
			return nil, err
		}
		cmd, err = t.commandAt(ctx, level, nil, verb, args...)
		if err != nil {
			return nil, err
		}
		if err := cmd.Run(); err != nil {
			return nil, t.storeError(verb, cmd, err)
		}
	}
	// The repository was opened, its command not run.
	t.setPresent()
	if err := notRunError(verb, cmd); err != nil {
		return nil, err
	}
	return cmd.Stdout(), nil
}

// callError is the cmdError of a call on the store, with the hint to set
// disable_tls when the store is a s3 one reached with TLS whose endpoint
// answers in plain http: plakar then fails on the handshake, with a
// message of the tls library saying nothing of the keyword.
func (t *T) callError(verb string, cmd *command.T, err error) error {
	err = cmdError(verb, cmd, err)
	location, _, lerr := t.location()
	if lerr != nil || scheme(location) != "s3" {
		return err
	}
	if disableTLS, _ := t.disableTLS(); disableTLS {
		return err
	}
	// plakar says a repository not opened on its standard error, and
	// what a command fails on, as a create, on its standard output.
	said := string(cmd.Stderr()) + string(cmd.Stdout())
	for _, message := range plainHTTPMessages {
		if strings.Contains(said, message) {
			return fmt.Errorf("%w: %s", err, plainHTTPHint)
		}
	}
	return err
}

// reason returns why plakar says its command failed, without the name it
// opens the line with, or nothing when it says none.
//
// plakar 1.1.8 prints the error its command returns, whatever the command,
// as one last line of its standard output, "<argv0>: <error>", argv0 being
// the path it was run by and the error made one line, its unprintable
// characters replaced. Its standard error only holds what it says before
// running a command, as a repository not opened, and the warnings and the
// per-file errors of the command.
//
// The rest of the standard output is never quoted: it is the listing of
// the files of a backup, the snapshots of a ls, the content of a cat. Only
// the last line opening with the path plakar was run by is taken, which no
// line of a listing does, cut to reasonMax bytes.
func reason(cmd *command.T) string {
	return reasonIn(cmd, cmd.Stdout(), false)
}

// reasonIn is reason for the output out of the command. With last, only
// the last line of the output is a reason, not a line before it.
func reasonIn(cmd *command.T, out []byte, last bool) string {
	args := cmd.Cmd().Args
	if len(args) == 0 {
		return ""
	}
	prefix := args[0] + ": "
	rest := strings.TrimRight(string(out), "\r\n")
	for rest != "" {
		i := strings.LastIndexByte(rest, '\n')
		line := strings.TrimRight(rest[i+1:], "\r")
		if s, ok := strings.CutPrefix(line, prefix); ok {
			if s = strings.TrimSpace(s); len(s) > reasonMax {
				s = strings.ToValidUTF8(s[:reasonMax], "") + "..."
			}
			return s
		}
		if i < 0 || last {
			break
		}
		rest = rest[:i]
	}
	return ""
}

// notRunError is the error of a plakar call that exited 0 without running
// its command, and nil for a call that ran it.
//
// plakar 1.1.8 rebuilds the state of the repository before it runs a
// command on it, and a rebuild failing, as on a state the key of the
// CONFIG does not open or a store no longer answering, skips the command,
// prints the reason as the one of a failed command and exits with the
// status it never set: 0. A backup taken for done on its exit code would
// have written nothing, a check verified nothing.
//
// The reason line is the only one plakar opens with the path it was run
// by, and it prints it of an error only: no command says one when it
// succeeds, nor its welcome on a cache not ready. The line is looked for
// last, where plakar prints it after everything else, so a line of the
// listing of a backup, as a file named to look like one, is not taken for
// it.
func notRunError(verb string, cmd *command.T) error {
	return notRunErrorIn(verb, cmd, cmd.Stdout())
}

func notRunErrorIn(verb string, cmd *command.T, out []byte) error {
	if s := reasonIn(cmd, out, true); s != "" {
		return fmt.Errorf("plakar %s: plakar exited 0 without running the command: %s", verb, s)
	}
	return nil
}

// cmdError names the verb that failed and keeps the exit error underneath,
// followed by the reason plakar printed on its standard output, then by
// its standard error. Both are said: plakar words most failures on the
// first only, as a backup, a rm or a create failing, the ones met before
// the command runs on the second only, and some on both, as a cat saying
// "errors occurred" of the errors its standard error details.
func cmdError(verb string, cmd *command.T, err error) error {
	err = fmt.Errorf("plakar %s: %w", verb, err)
	if s := reason(cmd); s != "" {
		err = fmt.Errorf("%w: %s", err, s)
	}
	if stderr := strings.TrimSpace(string(cmd.Stderr())); stderr != "" {
		err = fmt.Errorf("%w: %s", err, stderr)
	}
	return err
}

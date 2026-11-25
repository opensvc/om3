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
// cannot resolve gets its stores.yml written again and one retry.
func (t *T) run(ctx context.Context, level zerolog.Level, verb string, args ...string) ([]byte, error) {
	cmd, err := t.commandAt(ctx, level, nil, verb, args...)
	if err != nil {
		return nil, err
	}
	if err := cmd.Run(); err != nil {
		if !strings.Contains(string(cmd.Stderr()), unresolvedMessage) {
			return nil, t.callError(verb, cmd, err)
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
			return nil, t.callError(verb, cmd, err)
		}
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
	stderr := string(cmd.Stderr())
	for _, message := range plainHTTPMessages {
		if strings.Contains(stderr, message) {
			return fmt.Errorf("%w: %s", err, plainHTTPHint)
		}
	}
	return err
}

// cmdError names the verb that failed and keeps the exit error underneath.
func cmdError(verb string, cmd *command.T, err error) error {
	if stderr := strings.TrimSpace(string(cmd.Stderr())); stderr != "" {
		return fmt.Errorf("plakar %s: %w: %s", verb, err, stderr)
	}
	return fmt.Errorf("plakar %s: %w", verb, err)
}

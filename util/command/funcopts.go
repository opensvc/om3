package command

import (
	"bufio"
	"context"
	"time"

	"github.com/rs/zerolog"

	"github.com/opensvc/om3/v3/util/funcopt"
	"github.com/opensvc/om3/v3/util/plog"
)

// WithName sets the process args[0]
func WithName(name string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.name = name
		return nil
	})
}

// WithArgs sets the process args[1:] from a string slice
func WithArgs(args []string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.args = args
		return nil
	})
}

// WithContext sets a custom context.Context for the command execution,
// using context.Background() if none is provided.
func WithContext(ctx context.Context) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		if ctx == nil {
			ctx = context.Background()
		}
		t.ctx = ctx
		return nil
	})
}

// WithVarArgs sets the process args[1:] from a variadic string slice
func WithVarArgs(args ...string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.args = args
		return nil
	})
}

// WithLogger defines the Logger that will receive this pkg logs and process outputs
func WithLogger(l *plog.Logger) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.log = l
		return nil
	})
}

// WithLogger defines the Logger that will receive this pkg logs and process outputs
func WithPrompt(l *bufio.Reader) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.promptReader = l
		return nil
	})
}

// WithTimeout sets the max duration the process is allowed to run. After this duration,
// the process is killed and an error is reported.
func WithTimeout(timeout time.Duration) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.timeout = timeout
		return nil
	})
}

// WithCommandLogLevel show command name and args during Start
//
//	default zerolog.DebugLevel
func WithCommandLogLevel(l zerolog.Level) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.commandLogLevel = l
		return nil
	})
}

// WithIgnoredExitCodes set alternate list of successful exit codes.
//
//	exit codes are checked during Wait().
//	- default successful exit code is 0 when WithIgnoredExitCodes is not used
//	- Ignore all exit codes: WithIgnoredExitCodes()
//	- Accept 0, 1 or 6 exit code: WithIgnoredExitCodes(0, 1, 6)
func WithIgnoredExitCodes(codes ...int) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.okExitCodes = codes
		return nil
	})
}

// WithErrorExitCodeLogLevel sets the level of the log entries for error exit code.
func WithErrorExitCodeLogLevel(l zerolog.Level) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.errorExitCodeLogLevel = l
		return nil
	})
}

// WithLogLevel sets the level of the log entries.
func WithLogLevel(l zerolog.Level) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.logLevel = l
		return nil
	})
}

// WithStdoutLogLevel sets the level of the log entries coming from the process stdout.
// If not set, stdout lines are not logged.
func WithStdoutLogLevel(l zerolog.Level) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.stdoutLogLevel = l
		return nil
	})
}

// WithStderrLogLevel sets the level of the log entries coming from the process stderr
// If not set, stderr lines are not logged.
func WithStderrLogLevel(l zerolog.Level) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.stderrLogLevel = l
		return nil
	})
}

// WithBufferedStdout activates the buffering of the lines emitted by the process on stdout
func WithBufferedStdout() funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.bufferStdout = true
		return nil
	})
}

// WithBufferedStderr activates the buffering of the lines emitted by the process on stderr
func WithBufferedStderr() funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.bufferStderr = true
		return nil
	})
}

func WithUser(user string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.user = user
		return nil
	})
}

func WithGroup(group string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.group = group
		return nil
	})
}

func WithCWD(cwd string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.cwd = cwd
		return nil
	})
}

func WithVarEnv(env ...string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.env = append(t.env, env...)
		return nil
	})
}

func WithEnv(env []string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.env = append(t.env, env...)
		return nil
	})
}

func WithOnStdoutLine(f func(string)) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.onStdoutLine = f
		return nil
	})
}

func WithOnStderrLine(f func(string)) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.onStderrLine = f
		return nil
	})
}

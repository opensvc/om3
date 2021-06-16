package command

import (
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"opensvc.com/opensvc/util/funcopt"
)

// valid ensure T is usable
func (t T) valid() error {
	disabledLog := zerolog.Disabled
	missingLog := func(s string) error { return fmt.Errorf("use funcopt %v without funcopt WithLogger", s) }
	if t.log == nil {
		switch {
		case t.stdoutLogLevel != disabledLog:
			return missingLog("WithStdoutLogLevel")
		case t.stderrLogLevel != disabledLog:
			return missingLog("WithStderrLogLevel")
		case t.commandLogLevel != zerolog.DebugLevel:
			return missingLog("WithCommandLogLevel")
		}
	}
	return nil
}

func WithName(name string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.name = name
		return nil
	})
}

func WithArgs(args []string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.args = args
		return nil
	})
}

func WithVarArgs(args ...string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.args = args
		return nil
	})
}

func WithLogger(l *zerolog.Logger) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.log = l
		return nil
	})
}

func WithTimeout(timeout time.Duration) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.timeout = timeout
		return nil
	})
}

// WithCommandLogLevel show command name and args during Start
//   default zerolog.DebugLevel
func WithCommandLogLevel(l zerolog.Level) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.commandLogLevel = l
		return nil
	})
}

// WithIgnoredExitCodes set alternate list of successful exit codes.
//   exit codes are checked during Wait().
//   - default successful exit code is 0 when WithIgnoredExitCodes is not used
//   - Ignore all exit codes: WithIgnoredExitCodes()
//   - Accept 0, 1 or 6 exit code: WithIgnoredExitCodes(0, 1, 6)
func WithIgnoredExitCodes(codes ...int) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.okExitCodes = codes
		return nil
	})
}

func WithStdoutLogLevel(l zerolog.Level) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.stdoutLogLevel = l
		return nil
	})
}

func WithStderrLogLevel(l zerolog.Level) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.stderrLogLevel = l
		return nil
	})
}

func WithBufferedStdout() funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.bufferStdout = true
		return nil
	})
}

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

func WithEnv(env []string) funcopt.O {
	return funcopt.F(func(i interface{}) error {
		t := i.(*T)
		t.env = env
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

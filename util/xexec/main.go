package xexec

import (
	"github.com/anmitsu/go-shlex"
	"github.com/pkg/errors"
	"os/exec"
	"strings"
	"syscall"
)

// T struct hold attributes that needs to be applied on exec.Cmd by Update func
type T struct {
	Cwd        string
	CmdArgs    []string
	CmdEnv     []string
	Credential *syscall.Credential
}

// Update func set attributes on existing exec.Cmd 'cmd' from T struct settings
func (t T) Update(cmd *exec.Cmd) error {
	if cmd == nil {
		return errors.New("Can't Update nil cmd")
	}
	if t.Cwd != "" {
		cmd.Dir = t.Cwd
	}
	if t.Credential != nil {
		if cmd.SysProcAttr == nil {
			cmd.SysProcAttr = &syscall.SysProcAttr{}
		}
		cmd.SysProcAttr.Credential = t.Credential
	}
	if len(t.CmdEnv) > 0 {
		cmd.Env = append(cmd.Env, t.CmdEnv...)
	}
	return nil
}

// CommandFromString wrapper to exec.Command from a string command 's'
// When string command 's' contains multiple commands,
//   exec.Command("/bin/sh", "-c", s)
// else
//   exec.Command from shlex.Split(s)
func CommandFromString(s string) (*exec.Cmd, error) {
	args, err := commandArgsFromString(s)
	if err != nil {
		return nil, err
	}
	return exec.Command(args[0], args[1:]...), nil
}

func CommandArgsFromString(s string) ([]string, error) {
	return commandArgsFromString(s)
}

func commandArgsFromString(s string) ([]string, error) {
	var needShell bool
	if len(s) == 0 {
		return nil, errors.New("can not create command from empty string")
	}
	switch {
	case strings.Contains(s, "|"):
		needShell = true
	case strings.Contains(s, "&&"):
		needShell = true
	case strings.Contains(s, ";"):
		needShell = true
	}
	if needShell {
		return []string{"/bin/sh", "-c", s}, nil
	}
	sSplit, err := shlex.Split(s, true)
	if err != nil {
		return nil, err
	}
	if len(sSplit) == 0 {
		return nil, errors.New("unexpected empty command args from string")
	}
	return sSplit, nil
}

package rescontainerocibase

import (
	"context"
	"os"
	"os/exec"
	"syscall"
)

type (
	// Credential is the identity the engine commands run as, when it is not
	// the agent's.
	//
	// An engine keeps its containers in a store of the user running it, so
	// every command of a container has to run as the same user: a container
	// run by a user is invisible to an inspect run by another, which reads as
	// the container being down.
	Credential struct {
		UID uint32
		GID uint32

		// Home is the working directory of the commands. The one of the
		// agent is not one a demoted user can necessarily enter, and an
		// engine failing to resolve its working directory fails the
		// command.
		Home string

		// Env is added to the environment of the commands, for the engine to
		// find the store and the runtime directory of the user rather than
		// the agent's.
		Env []string
	}

	// ExecutorCredentialer is an optional interface an ExecutorArgser
	// implements to run the engine commands as another user than the
	// agent.
	//
	// A nil Credential runs them as the agent. An error refuses them, and
	// is what the action reports: an engine run as the wrong user reports
	// nothing useful.
	ExecutorCredentialer interface {
		Credential() (*Credential, error)
	}
)

// Demote makes cmd run as the credential, when there is one.
func (c *Credential) Demote(cmd *exec.Cmd) {
	if c == nil {
		return
	}
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Credential = &syscall.Credential{Uid: c.UID, Gid: c.GID}
	if cmd.Env == nil {
		cmd.Env = os.Environ()
	}
	// The last value of a variable is the one exec passes on, so these
	// override the agent's.
	cmd.Env = append(cmd.Env, c.Env...)
	if c.Home != "" {
		cmd.Dir = c.Home
	}
}

// credential returns the identity the engine commands run as, nil for the
// agent's.
func (e *Executor) credential() (*Credential, error) {
	if i, ok := e.args.(ExecutorCredentialer); ok {
		return i.Credential()
	}
	return nil, nil
}

// command returns the engine command a, run as the credential of the
// container.
func (e *Executor) command(ctx context.Context, a ...string) (*exec.Cmd, error) {
	cred, err := e.credential()
	if err != nil {
		return nil, err
	}
	var cmd *exec.Cmd
	if ctx != nil {
		cmd = exec.CommandContext(ctx, e.bin, a...)
	} else {
		cmd = exec.Command(e.bin, a...)
	}
	cred.Demote(cmd)
	return cmd, nil
}

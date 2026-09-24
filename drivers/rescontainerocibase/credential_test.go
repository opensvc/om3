package rescontainerocibase

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/util/plog"
)

// credArgs is an argser whose only answer is the identity the commands run
// as: it is the only question the executor asks it before exec.
type credArgs struct {
	ExecutorArgser
	cred *Credential
}

func (a credArgs) Credential() (*Credential, error) {
	return a.cred, nil
}

type testLogger struct{}

func (testLogger) Log() *plog.Logger {
	return plog.NewDefaultLogger()
}

// nobody is a credential every node has, with a working directory it can
// enter.
var nobody = &Credential{
	UID:  65534,
	GID:  65534,
	Home: "/",
	Env:  []string{"HOME=/", "XDG_RUNTIME_DIR=/run/user/65534"},
}

// An engine keeps the containers of a user in a store of that user, so an
// executor asked for a credential runs every command as it: a run as the user
// and an inspect as root would see two different stores, and the container
// would read as down.
func TestAnExecutorRunsItsCommandsAsTheCredential(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("running a command as another user needs root")
	}
	e := NewExecutor("/bin/sh", credArgs{cred: nobody}, testLogger{})

	t.Run("a streamed command", func(t *testing.T) {
		logC, err := e.doExecRunLogs(context.Background(), "-c", "id -u; id -g; echo $HOME; echo $XDG_RUNTIME_DIR; pwd")
		require.NoError(t, err)
		var output strings.Builder
		for b := range logC {
			output.Write(b)
		}
		assert.Equal(t, "65534\n65534\n/\n/run/user/65534\n/\n", output.String())
	})

	t.Run("a logged command", func(t *testing.T) {
		err := e.doExecRun(context.Background(), nil, "-c", `test "$(id -u):$(id -g):$HOME:$XDG_RUNTIME_DIR:$(pwd)" = "65534:65534:/:/run/user/65534:/"`)
		assert.NoError(t, err, "the command sees the identity, the environment and the working directory of the credential")
	})

	t.Run("an exec'd command", func(t *testing.T) {
		cmd, err := e.command(context.Background(), "-c", "id -u")
		require.NoError(t, err)
		b, err := cmd.Output()
		require.NoError(t, err)
		assert.Equal(t, "65534\n", string(b))
	})
}

// No credential is the agent's own identity, as before rootless containers.
func TestAnExecutorWithoutCredentialRunsAsTheAgent(t *testing.T) {
	e := NewExecutor("/bin/sh", credArgs{}, testLogger{})
	cmd, err := e.command(context.Background(), "-c", "true")
	require.NoError(t, err)
	assert.Nil(t, cmd.SysProcAttr, "no credential is set on the command")
	assert.Empty(t, cmd.Dir, "the working directory is the agent's")
}

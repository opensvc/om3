//go:build linux

package pg

import (
	"github.com/stretchr/testify/require"
	"os"
	"testing"

	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/funcopt"
)

func TestMain(m *testing.M) {
	systemctl = func(opts ...funcopt.O) *command.T {
		return command.New(command.WithName("true"))
	}
	os.Exit(m.Run())
}

// A cpuset nobody capped yet has no file to write in: the apply delegates it
// first, for the first start of an object to find them.
func TestValueControllers(t *testing.T) {
	require.Empty(t, Config{}.valueControllers())
	require.Empty(t, Config{CPUQuota: "50%", MemLimit: "1g"}.valueControllers())
	require.Equal(t, []string{"cpuset"}, Config{CPUs: "0"}.valueControllers())
	require.Equal(t, []string{"cpuset"}, Config{Mems: "0"}.valueControllers())
	require.Equal(t, []string{"cpuset"}, Config{CPUs: DefaultValue}.valueControllers(), "a reset writes in it too")
}

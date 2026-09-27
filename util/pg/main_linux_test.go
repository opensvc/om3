//go:build linux

package pg

import (
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

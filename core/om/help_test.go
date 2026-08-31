package om

import (
	"testing"

	"github.com/opensvc/om3/v3/core/commoncmd/helptest"
)

func TestHelp(t *testing.T) {
	helptest.Run(t, root)
}

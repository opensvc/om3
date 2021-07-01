package volsignal

import (
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParse(t *testing.T) {
	s := "hup:container#1,container#2 SiGkIll:foo foo:bar foo"
	t.Logf("volume signal kw parser: %s", s)
	m := Parse(s)
	assert.Equal(t, 2, len(m), "2 valid signals parsed")
	assert.Contains(t, m, syscall.SIGHUP, "contains SIGHUP")
	assert.Contains(t, m, syscall.SIGKILL, "contains SIGKILL")
}

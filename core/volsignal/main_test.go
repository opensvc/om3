package volsignal

import (
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParse(t *testing.T) {
	s := "hup:container#1,container#2,container#1 SiGkIll:foo foo:bar foo"
	t.Logf("volume signal kw parser: %s", s)
	m := New(s)
	assert.Equal(t, 2, len(m.data), "2 valid signals parsed")
	assert.Contains(t, m.data, syscall.SIGHUP, "contains SIGHUP")
	assert.Equal(t, 2, len(m.data[syscall.SIGHUP]), "2 deduped rids for HUP")
	assert.Contains(t, m.data, syscall.SIGKILL, "contains SIGKILL")
}

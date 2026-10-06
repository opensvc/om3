package chkvg

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParse(t *testing.T) {
	out := "  root   19193135104B 0B\n  data   1000B 250B\n  empty  0B 0B\n"
	assert.Equal(t, []vg{{"root", 100}, {"data", 75}}, parse([]byte(out)))
}

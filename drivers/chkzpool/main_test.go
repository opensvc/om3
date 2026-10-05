package chkzpool

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParse(t *testing.T) {
	out := "rpool\tONLINE\ntank\tDEGRADED\nold\tSUSPENDED\n\n"
	assert.Equal(t, []pool{{"rpool", 0}, {"tank", 1}, {"old", 6}}, parse([]byte(out)))
}

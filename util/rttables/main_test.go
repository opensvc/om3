package rttables

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type (
	fileInfo map[string]struct {
		major uint32
		minor uint32
	}
)

var (
	cases = map[string]int{
		"local": 255,
		"main":  254,
	}
)

func TestT_Lookup(t *testing.T) {
	for rtname, rtindex := range cases {
		t.Run("Index", func(t *testing.T) {
			i, err := Index(rtname)
			if err != nil {
				require.Nil(t, err)
			}
			assert.Equal(t, i, rtindex)
		})
	}
}

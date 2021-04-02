package xmap

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKeys(t *testing.T) {
	tests := map[string]struct {
		data   interface{}
		output []string
	}{
		"empty map": {
			data:   map[string]string{},
			output: []string{},
		},
		"map with string index": {
			data:   map[string]string{"foo": "bar"},
			output: []string{"foo"},
		},
	}
	for testName, test := range tests {
		t.Logf("%s", testName)
		output := Keys(test.data)
		assert.Equal(t, test.output, output)
	}
}

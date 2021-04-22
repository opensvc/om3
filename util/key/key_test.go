package key

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKey(t *testing.T) {
	tests := []struct {
		s       string
		section string
		option  string
		render  string
		scope   string
	}{
		{
			s:       "topology",
			section: "DEFAULT",
			option:  "topology",
			render:  "topology",
			scope:   "",
		},
		{
			s:       "DEFAULT.topology",
			section: "DEFAULT",
			option:  "topology",
			render:  "topology",
			scope:   "",
		},
		{
			s:       "topology@nodes",
			section: "DEFAULT",
			option:  "topology@nodes",
			render:  "topology@nodes",
			scope:   "nodes",
		},
		{
			s:       "DEFAULT.topology@nodes",
			section: "DEFAULT",
			option:  "topology@nodes",
			render:  "topology@nodes",
			scope:   "nodes",
		},
		{
			s:       "fs#1.dev",
			section: "fs#1",
			option:  "dev",
			render:  "fs#1.dev",
			scope:   "",
		},
		{
			s:       "data.a.b/C.D",
			section: "data",
			option:  "a.b/C.D",
			render:  "data.a.b/C.D",
			scope:   "",
		},
	}
	for _, test := range tests {
		t.Logf("%s", test.s)
		k := Parse(test.s)
		render := k.String()
		assert.Equal(t, test.render, render)
		assert.Equal(t, test.section, k.Section)
		assert.Equal(t, test.option, k.Option)
		assert.Equal(t, test.scope, k.Scope())
	}

}

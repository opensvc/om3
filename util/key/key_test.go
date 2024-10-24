package key

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKey(t *testing.T) {
	cases := []struct {
		s      string
		t      T
		render string
		scope  string
		isZero bool
	}{
		{
			s:      "topology",
			t:      T{"DEFAULT", "topology"},
			render: "topology",
		},
		{
			s:      "DEFAULT.topology",
			t:      T{"DEFAULT", "topology"},
			render: "topology",
		},
		{
			s:      "topology@nodes",
			t:      T{"DEFAULT", "topology@nodes"},
			render: "topology@nodes",
			scope:  "nodes",
		},
		{
			s:      "DEFAULT.topology@nodes",
			t:      T{"DEFAULT", "topology@nodes"},
			render: "topology@nodes",
			scope:  "nodes",
		},
		{
			s:      "fs#1.dev",
			t:      T{"fs#1", "dev"},
			render: "fs#1.dev",
		},
		{
			s:      "data.a.b/C.D",
			t:      T{"data", "a.b/C.D"},
			render: "data.a.b/C.D",
		},
		{
			s:      "container#1",
			t:      T{"container#1", ""},
			render: "container#1",
		},
		{
			s:      ".foo",
			t:      T{Option: "foo"},
			render: ".foo",
		},
		// test cases where Parse must return zero T
		{
			s:      "",
			isZero: true,
		},
		{
			s:      ".",
			isZero: true,
		},
		{
			s:      " ",
			isZero: true,
		},
		{
			s:      "a ",
			isZero: true,
		},
		{
			s:      " b",
			isZero: true,
		},
		{
			s:      " .foo",
			isZero: true,
		},
		{
			s:      " foo.bar",
			isZero: true,
		},
		{
			s:      "foo.bar ",
			isZero: true,
		},
		{
			s:      "\tfoo.bar",
			isZero: true,
		},
	}
	for _, tc := range cases {
		t.Logf("verify after k := Parse(%q) if k == %#v, k.String() == %q, k.Scope() == %q and k.IsZero() == %v",
			tc.s, tc.t, tc.render, tc.scope, tc.isZero)
		k := Parse(tc.s)
		render := k.String()
		assert.Equal(t, tc.render, render, "k.String()")
		assert.Equal(t, tc.t.Section, k.Section, "k.Section")
		assert.Equal(t, tc.t.Option, k.Option, "k.Option")
		assert.Equal(t, tc.scope, k.Scope(), "k.Scope()")
		assert.Equal(t, tc.isZero, k.IsZero(), "k.IsZero()")
	}

}

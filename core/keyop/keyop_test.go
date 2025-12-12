package keyop

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/util/key"
)

func TestDrop(t *testing.T) {
	op1 := T{
		Key:   key.Parse("topology"),
		Op:    Set,
		Value: "failover",
	}
	op2 := T{
		Key:   key.Parse("priority"),
		Op:    Set,
		Value: "50",
	}
	ops := L{op1, op2}

	ops = ops.Drop(key.T{Section: "DEFAULT", Option: "priority"})
	assert.Len(t, ops, 1)
	assert.Equal(t, ops[0], op1)

	ops = ops.Drop(key.T{Section: "DEFAULT", Option: "foo"})
	assert.Len(t, ops, 1)
}

func TestParse(t *testing.T) {
	tests := []struct {
		expr  string
		key   key.T
		op    Op
		val   string
		index int
	}{
		{
			expr: "a=b",
			key:  key.Parse("a"),
			op:   Set,
			val:  "b",
		},
		{
			expr: "foo=bar",
			key:  key.Parse("foo"),
			op:   Set,
			val:  "bar",
		},
		{
			expr: "foo=bar+=666",
			key:  key.Parse("foo"),
			op:   Set,
			val:  "bar+=666",
		},
		{
			expr: "a+=b",
			key:  key.Parse("a"),
			op:   Append,
			val:  "b",
		},
		{
			expr: "ab",
			key:  key.T{},
			op:   Invalid,
			val:  "",
		},
		{
			expr: "ab:",
			key:  key.Parse("ab."),
			op:   Exist,
			val:  "",
		},
		{
			expr: "a.b:",
			key:  key.Parse("a.b"),
			op:   Exist,
			val:  "",
		},
		{
			expr:  "env.abc[0]=bb8",
			key:   key.T{Section: "env", Option: "abc"},
			op:    Insert,
			val:   "bb8",
			index: 0,
		},
		{
			expr:  "env.a[2]=b",
			key:   key.T{Section: "env", Option: "a"},
			op:    Insert,
			val:   "b",
			index: 2,
		},
		{
			expr: "fs.optional=false",
			key:  key.T{Section: "fs", Option: "optional"},
			op:   Set,
			val:  "false",
		},
		{
			expr: "disk#0.vdev=mirror {volume#1.exposed_devs[0]} {volume#2.exposed_devs[0]}",
			key:  key.T{Section: "disk#0", Option: "vdev"},
			op:   Set,
			val:  "mirror {volume#1.exposed_devs[0]} {volume#2.exposed_devs[0]}",
		},
	}
	for _, test := range tests {
		t.Run(test.expr, func(t *testing.T) {
			op := Parse(test.expr)
			t.Run("test key is correct", func(t *testing.T) {
				assert.Equal(t, test.key, op.Key)
			})
			t.Run("test option is correct", func(t *testing.T) {
				assert.Equal(t, test.op, op.Op)
			})
			t.Run("test value is correct", func(t *testing.T) {
				assert.Equal(t, test.val, op.Value)
			})
			if op.Op == Insert {
				t.Run("test index is correct", func(t *testing.T) {
					assert.Equal(t, test.index, op.Index)
				})
			}
		})
	}
}

func TestParseOps(t *testing.T) {
	cases := []struct {
		l        []string
		expected L
	}{
		{
			l: []string{"foo=bar"},
			expected: L{
				T{Key: key.T{Section: "DEFAULT", Option: "foo"}, Op: 1, Value: "bar", Index: 0},
			},
		},
		{
			l: []string{"foo1=bar1", "must_be_dropped", "foo2=bar2"},
			expected: L{
				T{Key: key.T{Section: "DEFAULT", Option: "foo1"}, Op: 1, Value: "bar1", Index: 0},
				T{Key: key.T{Section: "DEFAULT", Option: "foo2"}, Op: 1, Value: "bar2", Index: 0},
			},
		},
		{
			l:        []string{"must_be_dropped"},
			expected: L{},
		},
		{
			l:        []string{""},
			expected: L{},
		},
		{
			l:        []string{},
			expected: L{},
		},
		{
			l:        nil,
			expected: L{},
		},
	}
	for _, tc := range cases {
		t.Logf("ParseOps(%q)", tc.l)
		ops := ParseOps(tc.l)
		assert.Equal(t, tc.expected, ops)
	}
}

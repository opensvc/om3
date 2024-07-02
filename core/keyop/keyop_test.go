package keyop

import (
	"testing"

	"github.com/opensvc/om3/util/key"
	"github.com/stretchr/testify/assert"
)

func TestKeyopsDrop(t *testing.T) {
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

	ops = ops.Drop(key.T{"DEFAULT", "priority"})
	assert.Len(t, ops, 1)
	assert.Equal(t, ops[0], op1)

	ops = ops.Drop(key.T{"DEFAULT", "foo"})
	assert.Len(t, ops, 1)
}

func TestKeyopParse(t *testing.T) {
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

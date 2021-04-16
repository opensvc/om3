package keyop

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"opensvc.com/opensvc/util/key"
)

func TestKeyop(t *testing.T) {
	tests := []struct {
		expr string
		key  key.T
		op   Op
		val  string
	}{
		{
			expr: "a=b",
			key:  key.Parse("a"),
			op:   Set,
			val:  "b",
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
	}
	for _, test := range tests {
		t.Logf("%s", test.expr)
		op := Parse(test.expr)
		assert.Equal(t, test.key, op.Key)
		assert.Equal(t, test.op, op.Op)
		assert.Equal(t, test.val, op.Value)
	}

}

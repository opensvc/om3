package arithmetic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvalExpr(t *testing.T) {
	for _, tc := range []struct {
		expr     string
		expected int64
	}{
		{"1024", 1024},
		{"10g", 10737418240},
		{"10GB", 10000000000},
		{"1 + 2", 3},
		{"10 - 2 - 3", 5},
		{"2 * 3 * 4", 24},
		{"100 / 4", 25},
		{"1 + 2 * 3", 7},
		{"(1 + 2) * 3", 9},
		{"((2))", 2},
		{"-5 + 8", 3},
		{"10g / 2", 5368709120},
		{"10g - 1g", 9663676416},

		// A share is a fraction, which is what makes it useful multiplied.
		{"50% * 10g", 5368709120},
		{"12.5% * 8g", 1073741824},
		{"100% * 10g", 10737418240},

		// A count of bytes is what the caller asked for, so a half of an odd
		// number of them is one of them or the other.
		{"3 / 2", 2},
		{"1 / 2", 1},
		{"1 / 3", 0},

		// The size units are the ones every other size in a configuration is
		// written with.
		{"1ki", 1024},
		{"1kb", 1000},
	} {
		t.Run(tc.expr, func(t *testing.T) {
			v, err := EvalExpr(tc.expr)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, v)
		})
	}
}

func TestEvalExprRefusals(t *testing.T) {
	for _, expr := range []string{
		"",
		"1 +",
		"1 + + ",
		"(1 + 2",
		"1 / 0",
		"1 2",
		"banana",
		"1 + banana",
		"100%FREE",
	} {
		t.Run(expr, func(t *testing.T) {
			_, err := EvalExpr(expr)
			assert.Error(t, err)
		})
	}
}

// Eval computes the expressions a value holds and leaves the rest of it alone.
func TestEval(t *testing.T) {
	for _, tc := range []struct {
		in       string
		expected string
	}{
		{"", ""},
		{"10g", "10g"},
		{"$(1 + 1)", "2"},
		{"$( 50% * 10g )", "5368709120"},
		{"$((1 + 1) * 2)", "4"},
		{"a $(1+1) b $(2+2) c", "a 2 b 4 c"},
		{"no expression here", "no expression here"},
	} {
		t.Run(tc.in, func(t *testing.T) {
			s, err := Eval(tc.in)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, s)
		})
	}

	_, err := Eval("$(1 + 1")
	assert.Error(t, err, "an expression nothing closes")
}

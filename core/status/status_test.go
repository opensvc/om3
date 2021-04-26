package status

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAdd(t *testing.T) {
	tests := []struct {
		s1 T
		s2 T
		as T
	}{
		{
			s1: Up,
			s2: Down,
			as: Warn,
		},
		{
			s1: Up,
			s2: NotApplicable,
			as: Up,
		},
	}
	for _, test := range tests {
		t.Logf("%s & %s", test.s1, test.s2)
		as := test.s1
		as.Add(test.s2)
		assert.Equal(t, test.as, as)
	}

}

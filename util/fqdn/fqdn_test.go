package fqdn

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsValid(t *testing.T) {
	tests := []struct {
		s string
		v bool
	}{
		{
			s: "ca-dev.wuc",
			v: true,
		},
		{
			s: "ca&dev.wuc",
			v: false,
		},
		{
			s: "1ca-dev.wuc",
			v: true,
		},
	}
	for _, test := range tests {
		t.Logf("%s", test.s)
		v := IsValid(test.s)
		assert.Equalf(t, test.v, v, "%s", test.s)
	}
}

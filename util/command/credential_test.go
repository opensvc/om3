package command

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCredential(t *testing.T) {
	cases := []struct {
		user  string
		group string
	}{
		{"WrongUserX", "WrongGroupY"},
		{"WrongUserX", ""},
		{"", "WrongGroupY"},
	}
	for _, tc := range cases {
		name := "user: '" + tc.user + "' group '" + tc.group + "'"
		t.Run("return error for "+name, func(t *testing.T) {
			cred, err := credential(tc.user, tc.group)
			assert.NotNil(t, err)
			assert.Nil(t, cred)
		})
	}
}

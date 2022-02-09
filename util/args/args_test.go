package args

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test(t *testing.T) {
	s := "-f -d /tmp/foo --comment 'bad trip' --comment 'good trap'"
	l1 := []string{"-f", "-d", "/tmp/foo", "--comment", "bad trip", "--comment", "good trap"}
	l2 := []string{"-d", "/tmp/foo", "--comment", "bad trip", "--comment", "good trap"}
	l3 := []string{"--comment", "bad trip", "--comment", "good trap"}
	l4 := []string{}
	args, err := Parse(s)
	t.Run("Parse", func(t *testing.T) {
		assert.NoError(t, err, "")
		assert.Equal(t, l1, args.Get(), "")
	})
	t.Run("DropNotExistingOption", func(t *testing.T) {
		args.DropOption("--foo")
		assert.Equal(t, l1, args.Get(), "")
	})
	t.Run("DropOption", func(t *testing.T) {
		args.DropOption("-f")
		assert.Equal(t, l2, args.Get(), "")
	})
	t.Run("DropOptionAndValue", func(t *testing.T) {
		args.DropOptionAndValue("-d")
		assert.Equal(t, l3, args.Get(), "")
	})
	t.Run("DropMultiOptionAndValue", func(t *testing.T) {
		args.DropOptionAndValue("--comment")
		assert.Equal(t, l4, args.Get(), "")
	})

}

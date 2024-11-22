package args

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test(t *testing.T) {
	s := "--init -o a=b -o b=c --comment 'bad trip' --comment 'good trap' -d /tmp/foo -f"
	l1 := []string{"--init", "-o", "a=b", "-o", "b=c", "--comment", "bad trip", "--comment", "good trap", "-d", "/tmp/foo", "-f"}
	l2 := []string{"--init", "-o", "a=b", "-o", "b=c", "--comment", "bad trip", "--comment", "good trap", "-d", "/tmp/foo"}
	l3 := []string{"--init", "-o", "a=b", "-o", "b=c", "--comment", "bad trip", "--comment", "good trap"}
	l4 := []string{"--init", "-o", "a=b", "-o", "b=c", "--comment", "bad trip"}
	l5 := []string{"--init", "-o", "a=b", "--comment", "bad trip"}
	l6 := []string{"--init", "-o", "a=b", "--comment", "bad trip", "-o", "b=d"}
	l7 := []string{"-o", "a=b", "--comment", "bad trip", "-o", "b=d"}
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
	t.Run("DropOptionAndAnyValue", func(t *testing.T) {
		args.DropOptionAndAnyValue("-d")
		assert.Equal(t, l3, args.Get(), "")
	})
	t.Run("DropOptionAndExactValue", func(t *testing.T) {
		args.DropOptionAndExactValue("--comment", "good trap")
		assert.Equal(t, l4, args.Get(), "")
	})
	t.Run("DropMultiOptionAndMatchingValue", func(t *testing.T) {
		args.DropOptionAndMatchingValue("-o", "^b=.*")
		assert.Equal(t, l5, args.Get(), "")
	})
	t.Run("Append", func(t *testing.T) {
		args.Append("-o", "b=d")
		assert.Equal(t, l6, args.Get(), "")
	})
	t.Run("HasOptionAndMatchingValue", func(t *testing.T) {
		v := args.HasOptionAndMatchingValue("-o", "^a=")
		assert.True(t, v, "")
	})
	t.Run("HasOptionAndMatchingValue", func(t *testing.T) {
		v := args.HasOptionAndMatchingValue("-o", "^a=c")
		assert.False(t, v, "")
	})
	t.Run("DropFirstOption", func(t *testing.T) {
		args.DropOption("--init")
		assert.Equal(t, l7, args.Get(), "")
	})
}

package runfiles

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunFiles(t *testing.T) {
	d := Dir{
		Path: t.TempDir(),
	}

	t.Run("Count", func(t *testing.T) {
		n, err := d.Count()
		assert.NoError(t, err)
		assert.Zero(t, n)
	})

	t.Run("Create", func(t *testing.T) {
		content := "foo"
		err := d.Create([]byte(content))
		assert.NoError(t, err)
		filename := d.filename(os.Getpid())
		b, err := os.ReadFile(filename)
		assert.NoError(t, err)
		assert.Equal(t, string(b), content)
	})

	t.Run("CountAndClean", func(t *testing.T) {
		n, err := d.CountAndClean()
		assert.NoError(t, err)
		assert.Equal(t, 1, n)
	})

	t.Run("CreateStale", func(t *testing.T) {
		content := "foo"
		err := d.create(2, []byte(content))
		assert.NoError(t, err)
	})

	t.Run("CountAndCleanWithStale", func(t *testing.T) {
		n, err := d.CountAndClean()
		assert.NoError(t, err)
		assert.Equal(t, 1, n)
	})

	t.Run("Remove", func(t *testing.T) {
		err := d.Remove()
		assert.NoError(t, err)
	})
}

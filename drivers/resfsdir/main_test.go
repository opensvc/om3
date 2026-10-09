package resfsdir

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDirPath(t *testing.T) {
	ctx := context.Background()
	o := &T{DirPath: "/srv/www"}
	p, err := o.dirPath(ctx, true)
	require.NoError(t, err)
	assert.Equal(t, "/srv/www", p)
	assert.Equal(t, "/srv/www", o.Head())

	// A directory is a resource paths are written in, so one written in
	// another resource could name a directory naming it back.
	o = &T{DirPath: "fs#2:/www"}
	_, err = o.dirPath(ctx, false)
	assert.ErrorContains(t, err, "written volume#<n>:/<path>")
	assert.Empty(t, o.Head())

	_, err = (&T{}).dirPath(ctx, false)
	assert.Error(t, err)
}

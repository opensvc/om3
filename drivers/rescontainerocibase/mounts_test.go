package rescontainerocibase

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAVolumeMountSourceIsResolvedInTheHead(t *testing.T) {
	head := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(head, "html", "css"), 0755))
	require.NoError(t, os.Symlink("html/css", filepath.Join(head, "style")))
	realHead, err := filepath.EvalSymlinks(head)
	require.NoError(t, err)

	source, err := volumeMountSource(head, filepath.Join(head, "html"))
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(realHead, "html"), source)

	source, err = volumeMountSource(head, filepath.Join(head, "style"))
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(realHead, "html", "css"), source, "a link in the head is resolved, and handed resolved")
}

// A link a container plants in the volume must not have the engine mount a
// path of the node in the next container.
func TestAVolumeMountSourceLeadingOutOfTheHeadIsRefused(t *testing.T) {
	base := t.TempDir()
	head := filepath.Join(base, "head")
	require.NoError(t, os.Mkdir(head, 0755))
	require.NoError(t, os.Symlink("/", filepath.Join(head, "html")))

	_, err := volumeMountSource(head, filepath.Join(head, "html"))
	assert.Error(t, err)
	_, err = volumeMountSource(head, filepath.Join(head, "html", "etc"))
	assert.Error(t, err)
	_, err = volumeMountSource(head, filepath.Join(head, "..", "..", "etc"))
	assert.Error(t, err)
}

func TestAMissingVolumeMountSourceIsMadeInTheHead(t *testing.T) {
	base := t.TempDir()
	head := filepath.Join(base, "head")
	outside := filepath.Join(base, "outside")
	require.NoError(t, os.Mkdir(head, 0755))
	require.NoError(t, os.Mkdir(outside, 0755))
	require.NoError(t, os.Symlink(outside, filepath.Join(head, "data")))

	_, err := volumeMountSource(head, filepath.Join(head, "data", "new"))
	assert.Error(t, err)
	assert.NoDirExists(t, filepath.Join(outside, "new"), "the missing source is not made through the link")

	source, err := volumeMountSource(head, filepath.Join(head, "cache"))
	require.NoError(t, err)
	assert.DirExists(t, source)
}

func TestAVolumeWithNoHeadHasNoMountSource(t *testing.T) {
	_, err := volumeMountSource("", "/etc")
	assert.Error(t, err)
}

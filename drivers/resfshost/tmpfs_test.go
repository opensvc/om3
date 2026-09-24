package resfshost

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func ptrSize(n int64) *int64 { return &n }

func ptrMode(m os.FileMode) *os.FileMode { return &m }

// The size and the mode of a tmpfs are keywords, and a mount is given them:
// the size keyword is what a resize records the size it reached in, so the
// next mount keeps the resize.
func TestATmpfsIsMountedWithItsSizeAndModeKeywords(t *testing.T) {
	r := &T{Type: "tmpfs", MountOptions: "size=1m,noexec,mode=755", Size: ptrSize(2097152), Mode: ptrMode(0700)}
	assert.Equal(t, "noexec,size=2097152,mode=700", r.mountOptions(),
		"the options the keywords set are taken out of mnt_opt, which a mount reads last")
	assert.Equal(t, []string{
		"size=1m is ignored: the size keyword sets the size",
		"mode=755 is ignored: the mode keyword sets the mode",
	}, r.overriddenMountOptions(), "and the status says they are ignored")

	r = &T{Type: "tmpfs", MountOptions: "size=1m,mode=755"}
	assert.Equal(t, "size=1m,mode=755", r.mountOptions(), "without the keywords, mnt_opt is the mount options")
	assert.Empty(t, r.overriddenMountOptions())

	r = &T{Type: "ext4", MountOptions: "noatime", Size: ptrSize(1024)}
	assert.Equal(t, "noatime", r.mountOptions(), "only a tmpfs holds a size of its own")
}

// A mode names the special bits a mount option takes in octal: a sticky
// directory everyone may write in is 1777.
func TestOctalModeKeepsTheSpecialBits(t *testing.T) {
	assert.Equal(t, "700", octalMode(0700))
	assert.Equal(t, "1777", octalMode(os.ModeSticky|0777))
	assert.Equal(t, "2750", octalMode(os.ModeSetgid|0750))
}

// The size and mode keywords are the tmpfs driver's alone.
func TestOnlyTheTmpfsDriverHasSizeAndMode(t *testing.T) {
	has := func(fsType, option string) bool {
		for _, kw := range (&T{Type: fsType}).Manifest().Keywords() {
			if kw.Option == option {
				return true
			}
		}
		return false
	}
	assert.True(t, has("tmpfs", "size"))
	assert.True(t, has("tmpfs", "mode"))
	assert.False(t, has("ext4", "size"))
	assert.False(t, has("xfs", "mode"))
}

package datarecv

import (
	"os"
	"testing"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/util/converters"
)

// A file of the install keyword takes the mode the perm keyword sets, from a
// sec as from a cfg, as the configs and secrets keywords do. Without perm, a
// file from a sec is kept to its owner, and the install line has the last
// word either way.
func TestInstallFilePerm(t *testing.T) {
	path, err := naming.ParsePath("ns1/svc/s1")
	if err != nil {
		t.Fatal(err)
	}
	perm := func(m os.FileMode) *os.FileMode { return &m }
	v, err := converters.Shlex.Convert(`/etc/a.conf from ./cfg/c key a.conf
/etc/b.pem from ./sec/s key b.pem
/etc/c.sh from ./cfg/c key c.sh mode 0700`)
	if err != nil {
		t.Fatal(err)
	}
	install := v.([]string)
	for _, tc := range []struct {
		name string
		perm *os.FileMode
		want []os.FileMode
	}{
		{"no perm", nil, []os.FileMode{0o644, 0o600, 0o700}},
		{"perm 0755", perm(0o755), []os.FileMode{0o755, 0o755, 0o700}},
		{"perm 0640", perm(0o640), []os.FileMode{0o640, 0o640, 0o700}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recv := &DataRecv{Install: install, Perm: tc.perm}
			_, files := recv.parseInstall("/head", path)
			if len(files) != len(tc.want) {
				t.Fatalf("got %d files, want %d", len(files), len(tc.want))
			}
			for i, f := range files {
				if got := f.AccessControl.Perm; got != tc.want[i] {
					t.Errorf("%s: mode %s, want %s", f.ToPath, got, tc.want[i])
				}
			}
		})
	}
}

// The mode of an install line is read as chmod reads it: the leading digit
// of a 4 digit mode sets the setuid, setgid and sticky bits, which a shared
// directory needs for its files to keep its group.
func TestInstallModeKeepsTheSpecialBits(t *testing.T) {
	path, err := naming.ParsePath("ns1/svc/s1")
	if err != nil {
		t.Fatal(err)
	}
	v, err := converters.Shlex.Convert(`/srv/shared/ mode 2775
/tmp/spool/ mode 1777
/bin/tool from ./cfg/c key tool mode 4755
/etc/a.conf from ./cfg/c key a.conf mode 0640`)
	if err != nil {
		t.Fatal(err)
	}
	recv := &DataRecv{Install: v.([]string)}
	dirs, files := recv.parseInstall("/head", path)
	if len(dirs) != 2 || len(files) != 2 {
		t.Fatalf("got %d dirs and %d files, want 2 and 2", len(dirs), len(files))
	}
	for _, tc := range []struct {
		name string
		got  os.FileMode
		want os.FileMode
	}{
		{"setgid dir", dirs[0].Perm, os.ModeSetgid | 0o775},
		{"sticky dir", dirs[1].Perm, os.ModeSticky | 0o777},
		{"setuid file", files[0].AccessControl.Perm, os.ModeSetuid | 0o755},
		{"plain file", files[1].AccessControl.Perm, 0o640},
	} {
		if tc.got != tc.want {
			t.Errorf("%s: mode %s, want %s", tc.name, tc.got, tc.want)
		}
	}
}

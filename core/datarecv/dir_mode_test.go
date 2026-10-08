package datarecv

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/rs/zerolog"

	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/util/file"
	"github.com/opensvc/om3/v3/util/plog"
)

type dirModeReceiver struct {
	head   string
	log    *plog.Logger
	status *resource.StatusLog
}

func (r dirModeReceiver) Head() string                     { return r.head }
func (r dirModeReceiver) Log() *plog.Logger                { return r.log }
func (r dirModeReceiver) StatusLog() resource.StatusLogger { return r.status }
func (r dirModeReceiver) GetObject() any                   { return nil }

// A directory mode with a setuid, setgid or sticky flag, such as dirperm=2750,
// is in place when the directory has the flag too: neither the status nor the
// install see a difference, and only a missing flag is reported and fixed. A
// directory the install creates has the flag at once, which mkdir alone drops.
func TestDirModeIncludesSpecialBits(t *testing.T) {
	user, group := strconv.Itoa(os.Getuid()), strconv.Itoa(os.Getgid())
	want := 0o750 | os.ModeSetgid
	for _, tc := range []struct {
		name       string
		current    os.FileMode
		wantChange bool
	}{
		{"flag in place", 0o750 | os.ModeSetgid, false},
		{"flag missing", 0o750, true},
		{"directory absent", 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			head := t.TempDir()
			p := filepath.Join(head, "sock")
			if tc.current != 0 {
				if err := os.Mkdir(p, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(p, tc.current); err != nil {
					t.Fatal(err)
				}
			}
			var logs bytes.Buffer
			receiver := dirModeReceiver{head: head, log: plog.NewLogger(zerolog.New(&logs)), status: resource.NewStatusLog()}
			recv := &DataRecv{to: receiver}

			if tc.current != 0 {
				recv.statusDir("sock", head, want, user, group)
				if warned := receiver.status.Len() != 0; warned != tc.wantChange {
					t.Fatalf("status warnings = %v, want %v", receiver.status.Entries(), tc.wantChange)
				}
			}

			if err := recv.installDir("sock", head, want, user, group); err != nil {
				t.Fatal(err)
			}
			if changed := strings.Contains(logs.String(), "change directory"); changed != tc.wantChange {
				t.Fatalf("install changed permissions = %v, want %v: %s", changed, tc.wantChange, logs.String())
			}
			info, err := os.Stat(p)
			if err != nil {
				t.Fatal(err)
			}
			if got := info.Mode() & file.ModeBits; got != want {
				t.Fatalf("mode after install = %s, want %s", got, want)
			}
		})
	}
}

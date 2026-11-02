package volfile

import (
	"encoding/json"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// setMark writes the mark of an upload on f. A filesystem refusing extended
// attributes leaves the file unmarked, which is no reason to refuse it.
func setMark(f *os.File, m mark) {
	b, err := json.Marshal(m)
	if err != nil {
		return
	}
	_ = unix.Fsetxattr(int(f.Fd()), markAttr, b, 0)
}

// getMark reads the mark of the entry of the name in the directory d, opened
// in the volume: the path goes through d, so no link of the volume leads the
// read elsewhere, and the entry itself is not followed.
func getMark(d *os.File, name string) (mark, bool) {
	var m mark
	buf := make([]byte, 1024)
	n, err := unix.Lgetxattr(fmt.Sprintf("/proc/self/fd/%d/%s", d.Fd(), name), markAttr, buf)
	if err != nil || json.Unmarshal(buf[:n], &m) != nil {
		return mark{}, false
	}
	return m, true
}

package volfile

import (
	"encoding/json"
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

// getMark reads the mark of the file at path, not following a link: a name
// is listed as what it is.
func getMark(path string) (mark, bool) {
	var m mark
	buf := make([]byte, 1024)
	n, err := unix.Lgetxattr(path, markAttr, buf)
	if err != nil || json.Unmarshal(buf[:n], &m) != nil {
		return mark{}, false
	}
	return m, true
}

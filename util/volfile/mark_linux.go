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
	_ = unix.Fsetxattr(int(f.Fd()), UploadAttr, b, 0)
}

// getMark reads the mark of an upload of the entry of the name in the
// directory d, opened in the volume.
func getMark(d *os.File, name string) (mark, bool) {
	var m mark
	return m, readAttr(d, name, UploadAttr, &m)
}

// getInstallMark reads the mark of an install of the entry of the name in the
// directory d, opened in the volume.
func getInstallMark(d *os.File, name string) (InstallMark, bool) {
	var m InstallMark
	return m, readAttr(d, name, InstallAttr, &m)
}

// readAttr decodes the extended attribute of the entry of the name in the
// directory d, opened in the volume: the path goes through d, so no link of
// the volume leads the read elsewhere, and the entry itself is not followed.
func readAttr(d *os.File, name, attr string, v any) bool {
	buf := make([]byte, 4096)
	n, err := unix.Lgetxattr(fmt.Sprintf("/proc/self/fd/%d/%s", d.Fd(), name), attr, buf)
	return err == nil && json.Unmarshal(buf[:n], v) == nil
}

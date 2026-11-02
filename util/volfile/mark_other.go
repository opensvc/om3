//go:build !linux

package volfile

import "os"

func setMark(*os.File, mark) {}

func getMark(string) (mark, bool) { return mark{}, false }

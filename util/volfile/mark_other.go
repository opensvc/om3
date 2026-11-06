//go:build !linux

package volfile

import "os"

func setMark(*os.File, mark) {}

func setInstallMark(*os.File, InstallMark) {}

func getMark(*os.File, string) (mark, bool) { return mark{}, false }

func getInstallMark(*os.File, string) (InstallMark, bool) { return InstallMark{}, false }

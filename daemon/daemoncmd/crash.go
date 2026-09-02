package daemoncmd

import (
	"errors"
	"os"
	"path/filepath"
	"runtime/debug"

	"github.com/opensvc/om3/v3/core/rawconfig"
)

// crashFileBasename is the file the runtime writes its crash report to,
// in addition to stderr. The name is the one the recover in the om main
// used to write, so that whoever knows where to look still finds it.
const crashFileBasename = "om.stack"

// setupCrashReport arranges for a crash of this daemon to be legible
// after the fact.
//
// The runtime prints an unhandled panic, and the fatal errors no
// recover can catch, to stderr, which for a daemon started by the
// service manager is the journal. Two things are missing from that
// report: the goroutines other than the one that crashed, which for a
// daemon is where the story is, and a copy that outlives a journal that
// was rotated or never kept.
//
// It replaces a recover in the main goroutine, which caught neither a
// panic in any of the daemon's other goroutines nor a fatal error, and
// dumped the crashing goroutine alone when it did catch something.
func setupCrashReport() {
	log := logger("crash report: ")
	debug.SetTraceback("all")
	filename := filepath.Join(rawconfig.Paths.Var, crashFileBasename)
	if err := rotateCrashFile(filename); err != nil {
		log.Warnf("rotate %s: %s", filename, err)
	}
	f, err := os.Create(filename)
	if err != nil {
		log.Warnf("create %s: %s", filename, err)
		return
	}
	// SetCrashOutput duplicates the descriptor, so the runtime keeps
	// writing to the file after this one is closed.
	defer func() { _ = f.Close() }()
	if err := debug.SetCrashOutput(f, debug.CrashOptions{}); err != nil {
		log.Warnf("set crash output to %s: %s", filename, err)
		return
	}
	log.Tracef("crash report goes to %s", filename)
}

// rotateCrashFile keeps the report of the previous crash out of the way
// of the next one.
//
// The crash file is truncated when the daemon starts, and a daemon that
// starts is often a daemon that just crashed, restarted by its service
// manager. Without this, the report is erased seconds after it was
// written, by the process that came to replace the one that wrote it.
func rotateCrashFile(filename string) error {
	info, err := os.Stat(filename)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	if info.Size() == 0 {
		// The daemon that had this file did not crash.
		return nil
	}
	return os.Rename(filename, filename+".1")
}

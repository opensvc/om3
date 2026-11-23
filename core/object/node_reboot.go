package object

import (
	"fmt"
	"runtime"

	"github.com/rs/zerolog"

	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/key"
)

// systemRebootArgv and systemShutdownArgv are the commands the operating
// system reboots and powers off with, through its shutdown sequence: the
// init system stops the daemon and the services it runs on the way down.
//
// Not the sysrq reboot of a split node in util/toc, which reboots at once,
// without syncing nor unmounting the disks.
var (
	systemRebootArgv = map[string][]string{
		"linux":   {"reboot"},
		"freebsd": {"reboot"},
		"darwin":  {"shutdown", "-r", "now"},
		"solaris": {"init", "6"},
	}
	systemShutdownArgv = map[string][]string{
		"linux":   {"shutdown", "-h", "now"},
		"freebsd": {"halt"},
		"darwin":  {"shutdown", "-h", "now"},
		"solaris": {"init", "5"},
	}
)

// Reboot reboots the node, after the reboot.pre and reboot.blocking_pre
// triggers.
//
// An error of reboot.pre is logged and ignored, an error of
// reboot.blocking_pre aborts the reboot, as a patching command that failed
// would leave the node half updated.
func (t *Node) Reboot() error {
	if err := t.rebootPreTriggers(); err != nil {
		return err
	}
	t.log.Infof("reboot")
	return t.runSystemCommand(systemRebootArgv)
}

// Shutdown powers the node off.
func (t *Node) Shutdown() error {
	t.log.Infof("shutdown")
	return t.runSystemCommand(systemShutdownArgv)
}

func (t *Node) rebootPreTriggers() error {
	if s := t.mergedConfig.GetString(key.New("reboot", "pre")); s != "" {
		t.log.Infof("execute trigger %s", s)
		if err := t.runTrigger(s); err != nil {
			t.log.Warnf("trigger %s: %s", s, err)
		}
	}
	if s := t.mergedConfig.GetString(key.New("reboot", "blocking_pre")); s != "" {
		t.log.Infof("execute blocking trigger %s", s)
		if err := t.runTrigger(s); err != nil {
			return fmt.Errorf("blocking pre trigger error: abort reboot: %w", err)
		}
	}
	return nil
}

func (t *Node) runTrigger(s string) error {
	argv, err := command.CmdArgsFromString(s)
	if err != nil {
		return err
	}
	return t.runCommand(argv)
}

func (t *Node) runSystemCommand(argvByOS map[string][]string) error {
	argv, ok := argvByOS[runtime.GOOS]
	if !ok {
		return fmt.Errorf("not implemented on %s", runtime.GOOS)
	}
	return t.runCommand(argv)
}

func (t *Node) runCommand(argv []string) error {
	cmd := command.New(
		command.WithName(argv[0]),
		command.WithArgs(argv[1:]),
		command.WithLogger(t.log),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
	)
	return cmd.Run()
}

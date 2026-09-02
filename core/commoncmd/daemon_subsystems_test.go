package commoncmd

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/opensvc/om3/v3/daemon/daemonenv"
)

// TestAuditSubsystemsHoldTheListeners keeps the audit taxonomy and the
// names the listeners answer to from drifting: a listener a client can
// address with an action but cannot name in an audit is the mismatch
// these two lists were merged to remove.
func TestAuditSubsystemsHoldTheListeners(t *testing.T) {
	for _, name := range daemonenv.ListenerNames {
		assert.Containsf(t, AuditSubsystems, name, "%s is a listener the audit cannot name", name)
	}
}

// TestAuditSubsystemsHelpListsThemAll pins that the help a user reads is
// rendered from the list the completion offers, rather than kept beside
// it by hand.
func TestAuditSubsystemsHelpListsThemAll(t *testing.T) {
	help := auditSubsystemsHelp()
	for _, name := range AuditSubsystems {
		assert.Containsf(t, strings.Fields(help), name, "%s is missing from the help", name)
	}
	for _, line := range strings.Split(strings.TrimRight(help, "\n"), "\n") {
		assert.LessOrEqual(t, len(line), 76, "a help line must fit a narrow terminal")
		assert.True(t, strings.HasPrefix(line, "  "), "a help line must be indented")
	}
}

func TestFilterPrefix(t *testing.T) {
	assert.Equal(t, AuditSubsystems, filterPrefix(AuditSubsystems, ""))
	assert.Equal(t, []string{"icfg", "icfg:"}, filterPrefix(AuditSubsystems, "icf"))
	assert.Empty(t, filterPrefix(AuditSubsystems, "nosuch"))
}

// TestListenerNameHelpNamesThemAll pins that the help a user reads names
// every listener, rather than a list kept beside them by hand: the ones a
// start, stop or restart may address, and the one that answers to none of
// the three, whose absence would otherwise read as an oversight.
func TestListenerNameHelpNamesThemAll(t *testing.T) {
	for _, name := range daemonenv.ListenerNames {
		assert.Containsf(t, ListenerNameHelp, name, "%s is missing from the help", name)
	}
	for _, name := range daemonenv.LifecycleListenerNames {
		assert.Containsf(t, daemonenv.ListenerNames, name, "%s is addressable but is no listener", name)
	}
	assert.NotContains(t, daemonenv.LifecycleListenerNames, daemonenv.ListenerNameUX,
		"the unix socket listener lives as long as the daemon does")
	assert.Contains(t, ListenerNameHelp, "lives as long as the daemon",
		"the help must say why the unix socket listener is not a value")
}

func TestForProgram(t *testing.T) {
	withProgram := func(t *testing.T, path string) {
		t.Helper()
		saved := os.Args[0]
		os.Args[0] = path
		t.Cleanup(func() { os.Args[0] = saved })
	}
	const help = `  # start it
  om daemon hb start 1.rx

  the id "om daemon hb status" shows, from the om command`

	t.Run("om reads its own name", func(t *testing.T) {
		withProgram(t, "/usr/bin/om")
		assert.Equal(t, help, ForProgram(help))
	})
	t.Run("ox reads its own name", func(t *testing.T) {
		withProgram(t, "/usr/bin/ox")
		got := ForProgram(help)
		assert.NotContains(t, strings.Fields(got), "om", "an ox help must not tell its reader to type om")
		assert.Contains(t, got, "ox daemon hb start 1.rx")
		assert.Contains(t, got, `"ox daemon hb status"`)

		// A word ending with the command name is not the command name.
		assert.Contains(t, got, "from the ox command")
	})
}

// TestHeartbeatStreamNamesHoldTheHeartbeats pins that the completion of a
// stream action offers the heartbeats too, since naming one addresses both
// of its streams.
func TestHeartbeatStreamNamesHoldTheHeartbeats(t *testing.T) {
	saved := heartbeatSectionNames
	heartbeatSectionNames = func() []string { return []string{"hb#1", "hb#11"} }
	t.Cleanup(func() { heartbeatSectionNames = saved })

	assert.Equal(t, []string{"1", "11"}, HeartbeatNames())
	assert.Equal(t, []string{"1", "1.rx", "1.tx", "11", "11.rx", "11.tx"}, HeartbeatStreamNames())
}

func TestWithout(t *testing.T) {
	assert.Equal(t, []string{"1.rx"}, without([]string{"1.rx", "1.tx"}, []string{"1.tx"}))
	assert.Equal(t, []string{"1.rx", "1.tx"}, without([]string{"1.rx", "1.tx"}, nil))
	assert.Empty(t, without([]string{"1.rx"}, []string{"1.rx"}))
}

package commoncmd

import (
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

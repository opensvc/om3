//go:build linux

package logging

import (
	"github.com/coreos/go-systemd/v22/journal"
)

func journalEnabled() bool {
	return journal.Enabled()
}

package console

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/opensvc/om3/v3/core/console"
)

type (
	// Session is what a console session records of itself for the time
	// it lasts.
	//
	// The record is a file, so it is there for whoever needs it after the
	// daemon that spawned the session is gone: the sessions outlive it.
	Session struct {
		PID       int            `json:"pid"`
		User      string         `json:"user"`
		Remote    string         `json:"remote"`
		Target    console.Target `json:"console"`
		StartedAt time.Time      `json:"started_at"`
	}
)

// ticketLinger is how long the mark of a used ticket is kept: longer than a
// ticket is valid for, clock differences included.
const ticketLinger = 10 * time.Minute

func sessionFile(dir string, pid int) string {
	return filepath.Join(dir, fmt.Sprintf("%d.json", pid))
}

// register records the session, and returns the function removing the
// record.
func register(dir string, s Session) (func(), error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	b, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	p := sessionFile(dir, s.PID)
	if err := os.WriteFile(p, b, 0600); err != nil {
		return nil, err
	}
	return func() { _ = os.Remove(p) }, nil
}

// errTicketUsed is a ticket presented a second time.
var errTicketUsed = errors.New("console ticket already used")

// useTicket marks the ticket used on this node, and fails if it was already.
//
// A ticket opens one session. The mark is a file, as the sessions are
// processes that share nothing else, and the marks of the tickets that
// expired long ago are removed on the way.
func useTicket(dir, id string) error {
	if id == "" || id != filepath.Base(id) {
		return errors.New("invalid console ticket id")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	sweepTickets(dir)
	f, err := os.OpenFile(filepath.Join(dir, id), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if errors.Is(err, os.ErrExist) {
		return errTicketUsed
	}
	if err != nil {
		return err
	}
	return f.Close()
}

func sweepTickets(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	limit := time.Now().Add(-ticketLinger)
	for _, e := range entries {
		if info, err := e.Info(); err == nil && info.ModTime().Before(limit) {
			_ = os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}

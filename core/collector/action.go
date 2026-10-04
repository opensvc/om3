package collector

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/opensvc/om3/v3/core/naming"
)

type (
	// ActionPhase is the phase of an instance action reported to the
	// collector: its begin or its end.
	ActionPhase string

	// Action describes an instance action reported to the collector.
	//
	// The action process writes it at begin, and again at end with End and
	// Status set. The log lines are not carried: the collector speaker reads
	// them from the journal of the node, by exec id and path, when it sends
	// the end.
	Action struct {
		Path      naming.Path `json:"path"`
		Action    string      `json:"action"`
		Argv      []string    `json:"argv"`
		RIDs      string      `json:"rids"`
		Origin    string      `json:"origin"`
		SessionID uuid.UUID   `json:"session_id"`
		ExecID    uuid.UUID   `json:"exec_id"`
		PID       int         `json:"pid"`
		Begin     time.Time   `json:"begin"`
		End       time.Time   `json:"end,omitzero"`
		Status    string      `json:"status,omitempty"`
	}

	// ActionPendingDir is the directory holding the begin and the end of the
	// local instance actions the collector did not acknowledge yet.
	//
	// The files of an action share a key, <exec_id>.<namespace>.<kind>.<name>:
	//
	//	<key>.begin.json   the begin, written by the action process
	//	<key>.end.json     the end, written by the action process
	//	<key>.uuid         the collector uuid of the begin, written by the daemon
	//
	// Each file has a single writer, so the action process and the daemon
	// never race on one. The daemon removes them all.
	ActionPendingDir string

	// ActionPendingKey describes the files found for a key.
	ActionPendingKey struct {
		Key      string
		HasBegin bool
		HasEnd   bool
		HasUUID  bool

		// ModTime is the modification time of the newest file of the key.
		ModTime time.Time
	}
)

const (
	ActionPhaseBegin ActionPhase = "begin"
	ActionPhaseEnd   ActionPhase = "end"

	actionSuffixBegin = ".begin.json"
	actionSuffixEnd   = ".end.json"
	actionSuffixUUID  = ".uuid"
)

// ActionKey returns the key of the pending files of the action of the
// exec id on the object path. The exec id is unique per process, and the
// path tells apart the objects one process acts on.
func ActionKey(execID uuid.UUID, p naming.Path) string {
	return fmt.Sprintf("%s.%s.%s.%s", execID, p.Namespace, p.Kind, p.Name)
}

// Key returns the key of the pending files of the action.
func (t Action) Key() string {
	return ActionKey(t.ExecID, t.Path)
}

// Phase returns the phase the action is at.
func (t Action) Phase() ActionPhase {
	if t.End.IsZero() {
		return ActionPhaseBegin
	}
	return ActionPhaseEnd
}

func (t ActionPhase) suffix() (string, error) {
	switch t {
	case ActionPhaseBegin:
		return actionSuffixBegin, nil
	case ActionPhaseEnd:
		return actionSuffixEnd, nil
	default:
		return "", fmt.Errorf("invalid action phase: %q", t)
	}
}

// Write writes the action file of its current phase. The file is written
// aside then renamed, so a reader never sees it partial.
func (t ActionPendingDir) Write(a Action) error {
	suffix, err := a.Phase().suffix()
	if err != nil {
		return err
	}
	b, err := json.Marshal(a)
	if err != nil {
		return err
	}
	return t.writeFile(a.Key()+suffix, b)
}

// Read returns the action of the key at the phase.
func (t ActionPendingDir) Read(key string, phase ActionPhase) (Action, error) {
	var a Action
	suffix, err := phase.suffix()
	if err != nil {
		return a, err
	}
	b, err := t.readFile(key + suffix)
	if err != nil {
		return a, err
	}
	err = json.Unmarshal(b, &a)
	return a, err
}

// WriteUUID records the collector uuid of the begin of the key.
func (t ActionPendingDir) WriteUUID(key, s string) error {
	return t.writeFile(key+actionSuffixUUID, []byte(s))
}

// ReadUUID returns the collector uuid of the begin of the key, and an empty
// string when the begin was not acknowledged.
func (t ActionPendingDir) ReadUUID(key string) (string, error) {
	b, err := t.readFile(key + actionSuffixUUID)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	} else if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

// RemoveBegin removes the begin file of the key.
func (t ActionPendingDir) RemoveBegin(key string) error {
	return t.remove(key + actionSuffixBegin)
}

// RemoveAll removes all the files of the key.
func (t ActionPendingDir) RemoveAll(key string) error {
	return errors.Join(
		t.remove(key+actionSuffixBegin),
		t.remove(key+actionSuffixEnd),
		t.remove(key+actionSuffixUUID),
	)
}

// List returns the keys found in the directory, with the files they have.
// A missing directory has no key.
func (t ActionPendingDir) List() ([]ActionPendingKey, error) {
	entries, err := os.ReadDir(string(t))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	m := make(map[string]*ActionPendingKey)
	keys := make([]string, 0)
	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			continue
		}
		name := entry.Name()
		var key string
		var set func(*ActionPendingKey)
		switch {
		case strings.HasSuffix(name, actionSuffixBegin):
			key = strings.TrimSuffix(name, actionSuffixBegin)
			set = func(k *ActionPendingKey) { k.HasBegin = true }
		case strings.HasSuffix(name, actionSuffixEnd):
			key = strings.TrimSuffix(name, actionSuffixEnd)
			set = func(k *ActionPendingKey) { k.HasEnd = true }
		case strings.HasSuffix(name, actionSuffixUUID):
			key = strings.TrimSuffix(name, actionSuffixUUID)
			set = func(k *ActionPendingKey) { k.HasUUID = true }
		default:
			// temporary files of a write in progress, or unrelated
			continue
		}
		info, err := entry.Info()
		if err != nil {
			// removed since ReadDir
			continue
		}
		k, ok := m[key]
		if !ok {
			k = &ActionPendingKey{Key: key}
			m[key] = k
			keys = append(keys, key)
		}
		set(k)
		if info.ModTime().After(k.ModTime) {
			k.ModTime = info.ModTime()
		}
	}
	l := make([]ActionPendingKey, len(keys))
	for i, key := range keys {
		l[i] = *m[key]
	}
	return l, nil
}

// root opens the directory as the root of the pending file operations, so
// a file name, made of a key, can not name a file out of it.
func (t ActionPendingDir) root() (*os.Root, error) {
	return os.OpenRoot(string(t))
}

func (t ActionPendingDir) readFile(name string) ([]byte, error) {
	root, err := t.root()
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()
	return root.ReadFile(name)
}

func (t ActionPendingDir) writeFile(name string, b []byte) error {
	if err := os.MkdirAll(string(t), 0700); err != nil {
		return err
	}
	root, err := t.root()
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	tmp := "." + name + "." + rand.Text()
	f, err := root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	if _, err := f.Write(b); err != nil {
		_ = f.Close()
		_ = root.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = root.Remove(tmp)
		return err
	}
	if err := root.Rename(tmp, name); err != nil {
		_ = root.Remove(tmp)
		return err
	}
	return nil
}

func (t ActionPendingDir) remove(name string) error {
	root, err := t.root()
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	if err := root.Remove(name); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

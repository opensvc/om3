package flock

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"time"

	bflock "github.com/gofrs/flock"
)

type (
	// T wraps flock and dumps a json data in the lock file
	// hinting about wath holds the lock.
	T struct {
		Base *bflock.Flock
	}

	meta struct {
		PID       int    `json:"pid"`
		Intent    string `json:"intent"`
		SessionId string `json:"session_id"`
	}
)

func New(p string) *T {
	return &T{
		Base: bflock.New(p),
	}
}

//
// Lock acquires an exclusive file lock on the file and write a json
// formatted structure hinting who holds the lock and with what
// intention.
//
func (t *T) Lock(timeout time.Duration, intent string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	locked, err := t.Base.TryLockContext(ctx, 500*time.Millisecond)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return locked, errors.New("lock timeout exceeded")
		}
		return locked, err
	}
	if err := writeMeta(t.Base.Path(), intent); err != nil {
		return locked, err
	}
	return locked, nil
}

func writeMeta(p string, intent string) error {
	m := meta{
		PID:       os.Getpid(),
		Intent:    intent,
		SessionId: os.Getenv("OSVC_SESSION_ID"),
	}
	f, err := os.Create(p)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(f)
	return enc.Encode(m)
}

func (t *T) Unlock() {
	os.Remove(t.Base.Path())
	t.Base.Unlock()
}

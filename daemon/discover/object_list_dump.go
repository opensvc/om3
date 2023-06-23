package discover

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"
)

type (
	// objectList janitors the list.objects file used by the
	// shell autocompletion.
	objectList struct {
		sync.RWMutex

		// File is where the object list is dumped
		File string

		// Delay limits the rate of the dump file rewrites
		Delay time.Duration

		// m is a the reference data of the
		// <var>/list.objects file content.
		// On ObjectStatusUpdated, add the object path in the map.
		// On ObjectStatusDeleted, del the object path from the map.
		// If the top level map changes, dump to file.
		m map[string]any

		// q is used for dump request coalescing and
		// rate limiting
		q chan bool

		// report write loop errors to the users (for logging, ...)
		ErrC chan error

		// report information to the users (for logging, ...)
		InfoC chan string

		ctx context.Context
	}
)

func newObjectList(ctx context.Context, file string) *objectList {
	t := objectList{
		File:  file,
		Delay: time.Second,
		m:     make(map[string]any),
		q:     make(chan bool, 2),
		ErrC:  make(chan error, 2),
		InfoC: make(chan string, 2),
		ctx:   ctx,
	}
	return &t
}

func (t *objectList) Loop() {
	for {
		select {
		case <-t.ctx.Done():
			return
		case <-t.q:
			if err := t.write(); err != nil {
				select {
				case <-t.ctx.Done():
					return
				case t.ErrC <- err:
				default:
				}
			}
			time.Sleep(time.Second)
		}
	}
}

func (t *objectList) requestWrite() bool {
	select {
	case <-t.ctx.Done():
		return false
	case t.q <- true:
		return true
	default:
		return false
	}
}

func (t *objectList) Add(l ...string) {
	t.Lock()
	defer t.Unlock()
	var changed bool
	for _, s := range l {
		if _, ok := t.m[s]; ok {
			continue
		}
		t.m[s] = nil
		changed = true
	}
	if changed {
		t.requestWrite()
	}
}

func (t *objectList) Del(l ...string) {
	t.Lock()
	defer t.Unlock()
	var changed bool
	for _, s := range l {
		if _, ok := t.m[s]; !ok {
			continue
		}
		delete(t.m, s)
		changed = true
	}
	if changed {
		t.requestWrite()
	}
}

func (t *objectList) write() error {
	t.RLock()
	defer t.RUnlock()
	f, err := os.OpenFile(t.File, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.ModePerm)
	if err != nil {
		return err
	}
	defer f.Close()
	for p, _ := range t.m {
		if _, err := fmt.Fprintf(f, "%s\n", p); err != nil {
			return err
		}
	}
	select {
	case <-t.ctx.Done():
		return t.ctx.Err()
	case t.InfoC <- fmt.Sprintf("%s dumped", t.File):
	default:
	}
	return nil
}

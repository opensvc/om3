package discover

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/core/rawconfig"
)

type (
	// objectList janitors the list.objects file used by the
	// shell autocompletion.
	objectList struct {
		// File is where the object list is dumped
		File string

		// Delay limits the rate of the dump file rewrites
		Delay time.Duration

		// m is a the reference data of the
		// <var>/list.objects file content.
		// On ObjectStatusUpdated, add the object path in the map.
		// On ObjectStatusDeleted, del the object path from the map.
		// If the top level map changes, dump to file.
		m map[path.T]any

		// q is used for dump request coalescing and
		// rate limiting
		q chan bool

		// report write loop errors to the users (for logging, ...)
		ErrC chan error

		// report information to the users (for logging, ...)
		InfoC chan string
	}
)

func newObjectList() *objectList {
	t := objectList{
		File:  filepath.Join(rawconfig.Paths.Var, "list.objects"),
		Delay: time.Second,
		m:     make(map[path.T]any),
		q:     make(chan bool, 2),
		ErrC:  make(chan error, 2),
		InfoC: make(chan string, 2),
	}
	return &t
}

func (t *objectList) Loop() {
	for {
		select {
		case <-t.q:
			if err := t.write(); err != nil {
				select {
				case t.ErrC <- err:
				}
			}
			time.Sleep(time.Second)
		}
	}
}

func (t *objectList) requestWrite() bool {
	select {
	case t.q <- true:
		return true
	default:
		return false
	}
}

func (t *objectList) Add(p path.T) {
	if _, ok := t.m[p]; ok {
		return
	}
	t.m[p] = nil
	t.requestWrite()
}

func (t *objectList) Del(p path.T) {
	if _, ok := t.m[p]; !ok {
		return
	}
	delete(t.m, p)
	t.requestWrite()
}

func (t *objectList) write() error {
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
	case t.InfoC <- fmt.Sprintf("%s dumped", t.File):
	}
	return nil
}

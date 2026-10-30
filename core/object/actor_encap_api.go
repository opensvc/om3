package object

import (
	"bufio"
	"context"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/encapapi"
	"github.com/opensvc/om3/v3/core/resource"
)

// encapAPIPingTimeout bounds the check that a guest daemon answers. It runs
// on the hypervisor of the guest: what does not answer at once is not there.
// A guest with a vsock device and no driver for it does not refuse: a connect
// to it runs out this time, and every operation on the guest pays it.
const encapAPIPingTimeout = 500 * time.Millisecond

// errEncapNoConfig says the encap instance has no config yet.
var errEncapNoConfig = errors.New("no encap instance config")

// encapAPI returns the driver of the encap instance through the api of the
// guest daemon, nil when the container offers none or the daemon does not
// answer, for the caller to use EncapCmd.
//
// This is the only place the two ways are chosen between, and nothing has been
// asked of the guest yet: a way failing once the guest accepted a request is
// an error, never a reason to ask again the other way, which would run an
// action twice.
func (t *actor) encapAPI(ctx context.Context, encapContainer resource.Encaper) *encapapi.T {
	i, ok := encapContainer.(resource.EncapAPIer)
	if !ok {
		return nil
	}
	url, err := i.EncapAPIURL(ctx)
	if err != nil {
		t.log.Tracef("%s: encap api url: %s", t.path, err)
		return nil
	}
	if url == "" {
		return nil
	}
	c, err := client.New(client.WithURL(url), client.WithTimeout(0))
	if err != nil {
		t.log.Tracef("%s: encap api client: %s", t.path, err)
		return nil
	}
	e := encapapi.New(c, encapContainer.GetHostname(), t.path)
	pingCtx, cancel := context.WithTimeout(ctx, encapAPIPingTimeout)
	defer cancel()
	if err := e.Ping(pingCtx); err != nil {
		t.log.Debugf("%s: the guest daemon does not answer at %s: %s", t.path, url, err)
		return nil
	}
	return e
}

// relayEncapLog logs the lines of r, as the encap instance logged them, at
// the level they say. The first field is a time and the second a level.
func (t *actor) relayEncapLog(r io.Reader, rid string) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		l := strings.Fields(line)
		if len(l) > 2 {
			switch {
			case strings.Contains(l[1], "DBG"):
				t.log.Tracef("%s: %s", rid, line)
			case strings.Contains(l[1], "INF"):
				t.log.Infof("%s: %s", rid, line)
			case strings.Contains(l[1], "WRN"):
				t.log.Warnf("%s: %s", rid, line)
			case strings.Contains(l[1], "ERR"):
				t.log.Errorf("%s: %s", rid, line)
			}
		} else {
			t.log.Errorf("%s: %s", rid, line)
		}
	}
	if err := scanner.Err(); err != nil && err != io.EOF {
		t.log.Errorf("error reading stderr: %v", err)
	}
}

// doEncapAPI runs the action on the encap instance through the api of the
// guest daemon, with the config of the object installed first if missing.
func (t *actor) doEncapAPI(ctx context.Context, e *encapapi.T, encapContainer resource.Encaper, action, rid string, opts encapapi.ActionOptions) error {
	has, err := e.HasConfig(ctx)
	if err != nil {
		return err
	}
	if !has {
		if err := t.installEncapConfig(ctx, encapContainer, t.path.ConfigFile()); err != nil {
			return err
		}
	}
	reader, writer := io.Pipe()
	done := make(chan struct{})
	go func() {
		defer close(done)
		t.relayEncapLog(reader, rid)
	}()
	err = e.Action(ctx, action, opts, writer)
	_ = writer.Close()
	<-done
	return err
}

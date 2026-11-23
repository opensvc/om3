package oxcmd

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/v3/core/clusterdump"
	"github.com/opensvc/om3/v3/core/event"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/status"
	"github.com/opensvc/om3/v3/core/tmplrender"
	"github.com/opensvc/om3/v3/daemon/msgbus"
)

func TestTemplateOutputWrite(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "out.conf")
	out, err := newTemplateOutput(p, "", "", "")
	require.NoError(t, err)

	changed, err := out.write([]byte("a"))
	require.NoError(t, err)
	assert.True(t, changed)
	fi, err := os.Stat(p)
	require.NoError(t, err)
	assert.Equal(t, fs.FileMode(0o600), fi.Mode().Perm(), "a new file is 0600")

	changed, err = out.write([]byte("a"))
	require.NoError(t, err)
	assert.False(t, changed, "the same content is not written again")

	// The mode of the file replaced is kept.
	require.NoError(t, os.Chmod(p, 0o640))
	changed, err = out.write([]byte("b"))
	require.NoError(t, err)
	assert.True(t, changed)
	fi, _ = os.Stat(p)
	assert.Equal(t, fs.FileMode(0o640), fi.Mode().Perm())

	// No temporary file is left.
	entries, _ := os.ReadDir(dir)
	assert.Len(t, entries, 1)
}

func TestTemplateOutputMode(t *testing.T) {
	p := filepath.Join(t.TempDir(), "out.conf")
	out, err := newTemplateOutput(p, "0644", "", "")
	require.NoError(t, err)
	assert.True(t, out.isGroupOrWorldReadable())
	_, err = out.write([]byte("a"))
	require.NoError(t, err)
	fi, _ := os.Stat(p)
	assert.Equal(t, fs.FileMode(0o644), fi.Mode().Perm())

	_, err = newTemplateOutput(p, "999", "", "")
	require.Error(t, err)
}

func TestTemplateOutputRefusesASymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	require.NoError(t, os.WriteFile(target, []byte("x"), 0o600))
	link := filepath.Join(dir, "link")
	require.NoError(t, os.Symlink(target, link))
	out, err := newTemplateOutput(link, "", "", "")
	require.NoError(t, err)
	_, err = out.write([]byte("y"))
	require.ErrorContains(t, err, "symbolic link")
	b, _ := os.ReadFile(target)
	assert.Equal(t, "x", string(b), "the target is untouched")
}

func writeScript(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(p, []byte("#!/bin/sh\n"+body+"\n"), 0o755))
	return p
}

func TestRunReload(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "log")
	ok := writeScript(t, dir, "ok", `echo "$OX_TEMPLATE_OUT" >> `+log)
	require.NoError(t, runReload(context.Background(), ok, "/etc/x.conf", time.Second))
	b, _ := os.ReadFile(log)
	assert.Equal(t, "/etc/x.conf\n", string(b))

	fail := writeScript(t, dir, "fail", "exit 2")
	err := runReload(context.Background(), fail, "x", time.Second)
	var rerr *reloadError
	require.True(t, errors.As(err, &rerr))
	assert.Equal(t, 3, rerr.ExitCode())

	slow := writeScript(t, dir, "slow", "sleep 10")
	begin := time.Now()
	err = runReload(context.Background(), slow, "x", 200*time.Millisecond)
	require.ErrorContains(t, err, "killed after")
	assert.Less(t, time.Since(begin), 5*time.Second)
}

func TestCheckReloadCommand(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "notexec")
	require.NoError(t, os.WriteFile(p, []byte("x"), 0o644))
	require.ErrorContains(t, checkReloadCommand(p), "not an executable")
	require.Error(t, checkReloadCommand(filepath.Join(dir, "missing")))
	require.NoError(t, checkReloadCommand(""))
}

// fakeStream is an events stream a test feeds.
type fakeStream struct {
	evC      chan *event.Event
	closed   chan struct{}
	once     sync.Once
	servedBy string
}

func (f *fakeStream) Read() (*event.Event, error) {
	select {
	case ev := <-f.evC:
		return ev, nil
	case <-f.closed:
		return nil, errors.New("closed")
	}
}

func (f *fakeStream) Close() error {
	f.once.Do(func() { close(f.closed) })
	return nil
}

func (f *fakeStream) GetServedBy() string {
	return f.servedBy
}

// fakeStreams opens fake streams, and records the subscriptions asked.
type fakeStreams struct {
	mu      sync.Mutex
	streams []*fakeStream
	subs    []tmplrender.Subscription
	id      uint64
}

func (f *fakeStreams) open(_ context.Context, sub tmplrender.Subscription) (event.ReadCloser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s := &fakeStream{evC: make(chan *event.Event, 100), closed: make(chan struct{}), servedBy: "n1"}
	f.streams = append(f.streams, s)
	f.subs = append(f.subs, sub)
	return s, nil
}

func (f *fakeStreams) last() (*fakeStream, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.streams) == 0 {
		return nil, 0
	}
	return f.streams[len(f.streams)-1], len(f.streams)
}

// send sends an event of msg on the last stream, with the next id.
func (f *fakeStreams) send(t *testing.T, kind string, msg any) {
	t.Helper()
	b, err := json.Marshal(msg)
	require.NoError(t, err)
	s, _ := f.last()
	f.mu.Lock()
	f.id++
	id := f.id
	f.mu.Unlock()
	s.evC <- &event.Event{Kind: kind, ID: id, At: time.Now(), Data: b}
}

func objectStatusUpdated(path string, avail status.T, at time.Time) *msgbus.ObjectStatusUpdated {
	p, _ := naming.ParsePath(path)
	return &msgbus.ObjectStatusUpdated{
		Path:  p,
		Node:  "n1",
		Value: object.Status{UpdatedAt: at, ActorStatus: &object.ActorStatus{Avail: avail}},
	}
}

type watchTest struct {
	t       *testing.T
	out     string
	reloads string
	streams *fakeStreams
	cancel  context.CancelFunc
	done    chan error
	hupC    chan os.Signal
}

// startWatch watches the template text, with the object prod/svc/web up in
// the first cluster data.
func startWatch(t *testing.T, text string, opts ...func(*CmdTemplate)) *watchTest {
	settleQuiet = 50 * time.Millisecond
	dir := t.TempDir()
	in := filepath.Join(dir, "in.tpl")
	require.NoError(t, os.WriteFile(in, []byte(text), 0o600))
	wt := &watchTest{
		t:       t,
		out:     filepath.Join(dir, "out"),
		reloads: filepath.Join(dir, "reloads"),
		streams: &fakeStreams{},
		done:    make(chan error, 1),
		hupC:    make(chan os.Signal, 1),
	}
	cmd := &CmdTemplate{
		In:            in,
		Out:           wt.out,
		Watch:         true,
		ReloadCommand: writeScript(t, dir, "reload", "echo >> "+wt.reloads),
		Delay:         50 * time.Millisecond,
		MaxDelay:      time.Second,
		ReloadTimeout: time.Second,
	}
	for _, opt := range opts {
		opt(cmd)
	}
	r, out, err := cmd.prepare()
	require.NoError(t, err)
	w := &templateWatcher{cmd: cmd, r: r, out: out, openStream: wt.streams.open}
	data := clusterdump.NewData("n1")
	data.Cluster.Object["prod/svc/web"] = objectStatusUpdated("prod/svc/web", status.Up, time.Now()).Value
	data.Cluster.Object["prod/svc/web2"] = objectStatusUpdated("prod/svc/web2", status.Up, time.Now()).Value
	ctx, cancel := context.WithCancel(context.Background())
	wt.cancel = cancel
	go func() { wt.done <- w.run(ctx, data, wt.hupC) }()
	t.Cleanup(wt.stop)
	return wt
}

func (wt *watchTest) stop() {
	wt.cancel()
	select {
	case <-wt.done:
		wt.done <- nil
	case <-time.After(5 * time.Second):
		wt.t.Error("the watch did not stop")
	}
}

func (wt *watchTest) content() string {
	b, _ := os.ReadFile(wt.out)
	return string(b)
}

func (wt *watchTest) reloadCount() int {
	b, _ := os.ReadFile(wt.reloads)
	return strings.Count(string(b), "\n")
}

func (wt *watchTest) eventually(f func() bool, msg string) {
	wt.t.Helper()
	require.Eventually(wt.t, f, 5*time.Second, 10*time.Millisecond, msg)
}

func TestWatchRendersOnceForABurst(t *testing.T) {
	wt := startWatch(t, `{{ avail "prod/svc/web" }}`)
	wt.eventually(func() bool { return wt.content() == "up" && wt.reloadCount() == 1 }, "first render")
	wt.eventually(func() bool { _, n := wt.streams.last(); return n == 1 }, "stream opened")
	assert.Equal(t, "prod/svc/web", wt.streams.subs[0].Selector)

	// The replay of the current data, then a refresh changing nothing
	// read: no write, no reload.
	now := time.Now()
	wt.streams.send(t, "ObjectStatusUpdated", objectStatusUpdated("prod/svc/web", status.Up, now))
	wt.streams.send(t, "ObjectStatusUpdated", objectStatusUpdated("prod/svc/web", status.Up, now.Add(time.Second)))
	time.Sleep(300 * time.Millisecond)
	assert.Equal(t, 1, wt.reloadCount())

	// A burst renders once, the last value.
	wt.streams.send(t, "ObjectStatusUpdated", objectStatusUpdated("prod/svc/web", status.Down, now))
	wt.streams.send(t, "ObjectStatusUpdated", objectStatusUpdated("prod/svc/web", status.Up, now))
	wt.streams.send(t, "ObjectStatusUpdated", objectStatusUpdated("prod/svc/web", status.Down, now))
	wt.eventually(func() bool { return wt.content() == "down" }, "render of the burst")
	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, 2, wt.reloadCount())
}

func TestWatchKeepsTheFileOnAStrictReadFailure(t *testing.T) {
	wt := startWatch(t, `{{ avail "prod/svc/web" }}`)
	wt.eventually(func() bool { return wt.content() == "up" }, "first render")
	wt.eventually(func() bool { _, n := wt.streams.last(); return n == 1 }, "stream opened")
	wt.streams.send(t, "ObjectStatusUpdated", objectStatusUpdated("prod/svc/web", status.Up, time.Now()))
	time.Sleep(200 * time.Millisecond)

	p, _ := naming.ParsePath("prod/svc/web")
	wt.streams.send(t, "ObjectStatusDeleted", &msgbus.ObjectStatusDeleted{Path: p, Node: "n1"})
	time.Sleep(300 * time.Millisecond)
	assert.Equal(t, "up", wt.content(), "the last good file is kept")

	// The object back, the template renders again.
	wt.streams.send(t, "ObjectStatusUpdated", objectStatusUpdated("prod/svc/web", status.Down, time.Now()))
	wt.eventually(func() bool { return wt.content() == "down" }, "render once the object is back")
}

func TestWatchReconnects(t *testing.T) {
	wt := startWatch(t, `{{ avail "prod/svc/web" }}`)
	wt.eventually(func() bool { _, n := wt.streams.last(); return n == 1 }, "stream opened")
	s, _ := wt.streams.last()
	_ = s.Close()
	wt.eventually(func() bool { _, n := wt.streams.last(); return n == 2 }, "stream opened again")

	// The new stream replays into new cluster data.
	wt.streams.send(t, "ObjectStatusUpdated", objectStatusUpdated("prod/svc/web", status.Down, time.Now()))
	wt.eventually(func() bool { return wt.content() == "down" }, "render from the new stream")
}

func TestWatchFollowsWhatTheTemplateReads(t *testing.T) {
	wt := startWatch(t, `{{ if eq (avail "prod/svc/web") "up" }}up{{ else }}{{ availOr "prod/svc/db" "none" }}{{ end }}`)
	wt.eventually(func() bool { _, n := wt.streams.last(); return n == 1 }, "stream opened")
	assert.Equal(t, "prod/svc/web", wt.streams.subs[0].Selector)

	// The web svc down, the template reads the db svc too: the stream is
	// opened again, asking its events.
	wt.streams.send(t, "ObjectStatusUpdated", objectStatusUpdated("prod/svc/web", status.Down, time.Now()))
	wt.eventually(func() bool { _, n := wt.streams.last(); return n == 2 }, "stream opened again")
	assert.Equal(t, "prod/svc/db,prod/svc/web", wt.streams.subs[1].Selector)
}

func TestWatchReadsTheTemplateAgainOnSIGHUP(t *testing.T) {
	wt := startWatch(t, `{{ avail "prod/svc/web" }}`)
	wt.eventually(func() bool { return wt.content() == "up" }, "first render")
	wt.eventually(func() bool { _, n := wt.streams.last(); return n == 1 }, "stream opened")
	wt.streams.send(t, "ObjectStatusUpdated", objectStatusUpdated("prod/svc/web", status.Up, time.Now()))
	time.Sleep(200 * time.Millisecond)
	cmdIn := filepath.Join(filepath.Dir(wt.out), "in.tpl")
	require.NoError(t, os.WriteFile(cmdIn, []byte(`web is {{ avail "prod/svc/web" }}`), 0o600))
	wt.hupC <- os.Interrupt
	wt.eventually(func() bool { return wt.content() == "web is up" }, "render of the new template")
}

func TestPrepare(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "in")
	require.NoError(t, os.WriteFile(in, []byte(`{{ .a }}`), 0o600))
	_, _, err := (&CmdTemplate{In: in, Out: "-", Watch: true}).prepare()
	require.Error(t, err)
	_, _, err = (&CmdTemplate{In: in, Out: "x", Vars: []string{"novalue"}}).prepare()
	require.ErrorContains(t, err, "key=value")
	r, _, err := (&CmdTemplate{In: in, Out: "-", Vars: []string{"a=1=2"}}).prepare()
	require.NoError(t, err)
	assert.Equal(t, "1=2", r.Data["a"])

	bad := filepath.Join(dir, "bad")
	require.NoError(t, os.WriteFile(bad, []byte(`{{ exec "ls" }}`), 0o600))
	_, _, err = (&CmdTemplate{In: bad, Out: "-"}).prepare()
	require.ErrorContains(t, err, "not defined")
}

func TestWatchKeepsItsSubscriptionOnAFailedRender(t *testing.T) {
	wt := startWatch(t, `{{ avail "prod/svc/web" }} {{ avail "prod/svc/web2" }}`)
	wt.eventually(func() bool { return wt.content() == "up up" }, "first render")
	wt.eventually(func() bool { _, n := wt.streams.last(); return n == 1 }, "stream opened")
	wt.streams.send(t, "ObjectStatusUpdated", objectStatusUpdated("prod/svc/web", status.Up, time.Now()))
	wt.streams.send(t, "ObjectStatusUpdated", objectStatusUpdated("prod/svc/web2", status.Up, time.Now()))
	time.Sleep(200 * time.Millisecond)

	// The render fails at its first read: the second is still followed,
	// the stream is not opened again without it.
	p, _ := naming.ParsePath("prod/svc/web")
	wt.streams.send(t, "ObjectStatusDeleted", &msgbus.ObjectStatusDeleted{Path: p, Node: "n1"})
	time.Sleep(300 * time.Millisecond)
	_, n := wt.streams.last()
	assert.Equal(t, 1, n)

	wt.streams.send(t, "ObjectStatusUpdated", objectStatusUpdated("prod/svc/web2", status.Down, time.Now()))
	wt.streams.send(t, "ObjectStatusUpdated", objectStatusUpdated("prod/svc/web", status.Up, time.Now()))
	wt.eventually(func() bool { return wt.content() == "up down" }, "render with both")
}

func TestEventConcerns(t *testing.T) {
	read := tmplrender.Read{Kinds: []string{"ObjectStatusUpdated"}, Selection: "prod/svc/web*,prod/cfg/*+!prod/cfg/x"}
	assert.True(t, eventConcerns(pendingEvent{kind: "ObjectStatusUpdated", path: "prod/svc/web1"}, read))
	assert.True(t, eventConcerns(pendingEvent{kind: "ObjectStatusUpdated", path: "prod/cfg/db"}, read))
	assert.False(t, eventConcerns(pendingEvent{kind: "ObjectStatusUpdated", path: "prod/svc/db"}, read))
	assert.False(t, eventConcerns(pendingEvent{kind: "InstanceStatusUpdated", path: "prod/svc/web1"}, read))

	// A read of node data, or of every object, is concerned by any event
	// of its kinds.
	all := tmplrender.Read{Kinds: []string{"NodeConfigUpdated"}}
	assert.True(t, eventConcerns(pendingEvent{kind: "NodeConfigUpdated", node: "n1"}, all))
}

func TestWatchDebug(t *testing.T) {
	r, w, err := os.Pipe()
	require.NoError(t, err)
	stderr := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = stderr }()

	wt := startWatch(t, `{{ avail "prod/svc/web" }}`, func(c *CmdTemplate) { c.Debug = true })
	wt.eventually(func() bool { _, n := wt.streams.last(); return n == 1 }, "stream opened")
	wt.streams.send(t, "ObjectStatusUpdated", objectStatusUpdated("prod/svc/web", status.Up, time.Now()))
	time.Sleep(200 * time.Millisecond)
	wt.streams.send(t, "ObjectStatusUpdated", objectStatusUpdated("prod/svc/web", status.Up, time.Now()))
	time.Sleep(200 * time.Millisecond)
	wt.streams.send(t, "ObjectStatusUpdated", objectStatusUpdated("prod/svc/web", status.Down, time.Now()))
	wt.eventually(func() bool { return wt.content() == "down" }, "render")
	wt.stop()
	_ = w.Close()
	os.Stderr = stderr
	b, _ := io.ReadAll(r)
	log := string(b)
	for _, s := range []string{
		"debug: render: first render, from the cluster status",
		"debug: events stream: subscribed to ObjectStatusDeleted,ObjectStatusUpdated, selector \"prod/svc/web\", served by n1",
		"debug: events stream: replay settled, 1 events applied",
		"debug: no render: 1 events applied, no read changed",
		"debug: render: reads changed: avail prod/svc/web",
		"debug: render: caused by event 3 ObjectStatusUpdated prod/svc/web@n1",
	} {
		assert.Contains(t, log, s)
	}
}

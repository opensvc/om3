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
	replayDone bool

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

func (f *fakeStream) ReplayDoneAdvertised() bool {
	return f.replayDone
}

// fakeStreams opens fake streams, and records the subscriptions asked.
type fakeStreams struct {
	// noServedBy opens streams not telling the node serving them.
	noServedBy bool

	// replayDone opens streams ending their replay with a ReplayDone
	// event.
	replayDone bool

	mu      sync.Mutex
	streams []*fakeStream
	subs    []tmplrender.Subscription
	id      uint64
}

func (f *fakeStreams) open(_ context.Context, sub tmplrender.Subscription) (event.ReadCloser, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s := &fakeStream{evC: make(chan *event.Event, 100), closed: make(chan struct{}), servedBy: "n1"}
	if f.noServedBy {
		s.servedBy = ""
	}
	s.replayDone = f.replayDone
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
	// fetcher fetches the values of the templates, none when nil.
	fetcher tmplrender.Fetcher

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
	wt.run(cmd)
	return wt
}

// run runs the watch of cmd, with the objects prod/svc/web and web2 up in the
// first cluster data.
func (wt *watchTest) run(cmd *CmdTemplate) {
	t := wt.t
	jobs, err := cmd.prepare()
	require.NoError(t, err)
	for _, job := range jobs {
		job.r.Fetcher = wt.fetcher
	}
	w := &templateWatcher{cmd: cmd, jobs: jobs, fetcher: wt.fetcher, openStream: wt.streams.open}
	data := clusterdump.NewData("n1")
	data.Cluster.Object["prod/svc/web"] = objectStatusUpdated("prod/svc/web", status.Up, time.Now()).Value
	data.Cluster.Object["prod/svc/web2"] = objectStatusUpdated("prod/svc/web2", status.Up, time.Now()).Value
	ctx, cancel := context.WithCancel(context.Background())
	wt.cancel = cancel
	go func() { wt.done <- w.run(ctx, data, wt.hupC) }()
	t.Cleanup(wt.stop)
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

	// The render reading db was made from cluster data not following db:
	// it is not written, the file is written once the new stream replayed.
	assert.Equal(t, "up", wt.content(), "no file written from cluster data missing what the render read")
	wt.streams.send(t, "ObjectStatusUpdated", objectStatusUpdated("prod/svc/web", status.Down, time.Now()))
	wt.streams.send(t, "ObjectStatusUpdated", objectStatusUpdated("prod/svc/db", status.Up, time.Now()))
	wt.eventually(func() bool { return wt.content() == "up" && wt.reloadCount() == 1 }, "render after the replay")
	wt.streams.send(t, "ObjectStatusUpdated", objectStatusUpdated("prod/svc/db", status.Down, time.Now()))
	wt.eventually(func() bool { return wt.content() == "down" }, "render of the db status")
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
	_, err := (&CmdTemplate{In: in, Out: "-", Watch: true}).prepare()
	require.Error(t, err)
	_, err = (&CmdTemplate{In: in, Out: "x", Vars: []string{"novalue"}}).prepare()
	require.ErrorContains(t, err, "key=value")
	jobs, err := (&CmdTemplate{In: in, Out: "-", Vars: []string{"a=1=2"}}).prepare()
	require.NoError(t, err)
	assert.Equal(t, "1=2", jobs[0].r.Data["a"])
	_, err = (&CmdTemplate{}).prepare()
	require.ErrorContains(t, err, "--config")

	bad := filepath.Join(dir, "bad")
	require.NoError(t, os.WriteFile(bad, []byte(`{{ exec "ls" }}`), 0o600))
	_, err = (&CmdTemplate{In: bad, Out: "-"}).prepare()
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

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(p, []byte(content), 0o600))
	return p
}

func TestPrepareConfig(t *testing.T) {
	dir := t.TempDir()
	a := writeFile(t, dir, "a.tpl", `{{ .env }}-{{ .site }}`)
	b := writeFile(t, dir, "b.tpl", `b`)
	reload := writeScript(t, dir, "reload", "true")
	cfg := writeFile(t, dir, "templates.yaml", `
vars:
  env: dev
  site: paris
templates:
  - in: `+a+`
    out: `+filepath.Join(dir, "a")+`
    mode: 0644
    reload_command: `+reload+`
  - in: `+b+`
    out: `+filepath.Join(dir, "b")+`
    reload_command: `+reload+`
    reload_timeout: 5s
`)
	jobs, err := (&CmdTemplate{Config: cfg, Vars: []string{"env=prod"}, ReloadTimeout: time.Minute}).prepare()
	require.NoError(t, err)
	require.Len(t, jobs, 2)
	assert.Equal(t, map[string]any{"env": "prod", "site": "paris"}, jobs[0].r.Data, "a --var replaces a var of the config")
	assert.Equal(t, fs.FileMode(0o644), *jobs[0].output.mode)
	assert.Equal(t, time.Minute, jobs[0].reloadTimeout, "the --reload-timeout is the default")
	assert.Equal(t, 5*time.Second, jobs[1].reloadTimeout)

	_, err = (&CmdTemplate{Config: cfg, In: a}).prepare()
	require.ErrorContains(t, err, "--config names the templates")

	for name, content := range map[string]string{
		"unknown key": "templates:\n  - in: " + a + "\n    out: x\n    oops: 1\n",
		"no out":      "templates:\n  - in: " + a + "\n",
		"stdout":      "templates:\n  - in: " + a + "\n    out: \"-\"\n",
		"same out":    "templates:\n  - in: " + a + "\n    out: x\n  - in: " + b + "\n    out: ./x\n",
		"empty":       "",
		"bad timeout": "templates:\n  - in: " + a + "\n    out: x\n    reload_timeout: soon\n",
	} {
		p := writeFile(t, dir, "bad.yaml", content)
		_, err := (&CmdTemplate{Config: p}).prepare()
		assert.Error(t, err, name)
	}
}

func TestReloadsOf(t *testing.T) {
	jobs := []*templateJob{
		{out: "a", reloadCommand: "/r1", reloadTimeout: time.Second},
		{out: "b"},
		{out: "c", reloadCommand: "/r2", reloadTimeout: time.Second},
		{out: "d", reloadCommand: "/r1", reloadTimeout: time.Minute},
	}
	l := reloadsOf(jobs)
	require.Len(t, l, 2)
	assert.Equal(t, &templateReload{command: "/r1", timeout: time.Minute, outs: []string{"a", "d"}}, l[0])
	assert.Equal(t, &templateReload{command: "/r2", timeout: time.Second, outs: []string{"c"}}, l[1])
}

// Two templates sharing a reload command: the command runs once for the
// files written together, a template failing does not hold back the other.
func TestWatchConfig(t *testing.T) {
	settleQuiet = 50 * time.Millisecond
	dir := t.TempDir()
	wt := &watchTest{
		t:       t,
		out:     filepath.Join(dir, "web"),
		reloads: filepath.Join(dir, "reloads"),
		streams: &fakeStreams{},
		done:    make(chan error, 1),
		hupC:    make(chan os.Signal, 1),
	}
	web := writeFile(t, dir, "web.tpl", `{{ avail "prod/svc/web" }}`)
	both := writeFile(t, dir, "both.tpl", `{{ avail "prod/svc/web" }} {{ avail "prod/svc/web2" }}`)
	reload := writeScript(t, dir, "reload", `echo "$OX_TEMPLATE_OUT" >> `+wt.reloads)
	outWeb, outBoth := filepath.Join(dir, "web"), filepath.Join(dir, "both")
	cfg := writeFile(t, dir, "templates.yaml", `
templates:
  - in: `+web+`
    out: `+outWeb+`
    reload_command: `+reload+`
  - in: `+both+`
    out: `+outBoth+`
    reload_command: `+reload+`
`)
	wt.run(&CmdTemplate{Config: cfg, Watch: true, Delay: 50 * time.Millisecond, MaxDelay: time.Second, ReloadTimeout: time.Second})
	read := func(p string) string { b, _ := os.ReadFile(p); return string(b) }
	reloads := func() []string {
		return strings.Fields(strings.ReplaceAll(read(wt.reloads), outBoth, "both"))
	}

	// The first renders write both files, and reload them once.
	wt.eventually(func() bool { return read(outWeb) == "up" && read(outBoth) == "up up" }, "first renders")
	wt.eventually(func() bool { return strings.Count(read(wt.reloads), "\n") == 1 }, "one reload")
	assert.Equal(t, outWeb+" "+outBoth+"\n", read(wt.reloads))

	wt.eventually(func() bool { _, n := wt.streams.last(); return n == 1 }, "stream opened")
	assert.Equal(t, "prod/svc/web,prod/svc/web2", wt.streams.subs[0].Selector, "one stream for both templates")
	wt.streams.send(t, "ObjectStatusUpdated", objectStatusUpdated("prod/svc/web", status.Up, time.Now()))
	wt.streams.send(t, "ObjectStatusUpdated", objectStatusUpdated("prod/svc/web2", status.Up, time.Now()))
	time.Sleep(200 * time.Millisecond)

	// web2 changes: only the second file is written and reloaded.
	wt.streams.send(t, "ObjectStatusUpdated", objectStatusUpdated("prod/svc/web2", status.Down, time.Now()))
	wt.eventually(func() bool { return read(outBoth) == "up down" }, "render of the second template")
	wt.eventually(func() bool { return strings.Count(read(wt.reloads), "\n") == 2 }, "second reload")
	assert.Equal(t, "both", reloads()[len(reloads())-1])

	// web2 deleted: the second template fails, its file is kept, and the
	// first renders on.
	p, _ := naming.ParsePath("prod/svc/web2")
	wt.streams.send(t, "ObjectStatusDeleted", &msgbus.ObjectStatusDeleted{Path: p, Node: "n1"})
	wt.streams.send(t, "ObjectStatusUpdated", objectStatusUpdated("prod/svc/web", status.Down, time.Now()))
	wt.eventually(func() bool { return read(outWeb) == "down" }, "render of the first template")
	assert.Equal(t, "up down", read(outBoth), "the file of the failed template is kept")
}

// SIGHUP reads the config file again: a template added is rendered, a var
// changed applies, and an invalid file keeps the former templates.
func TestWatchConfigSIGHUP(t *testing.T) {
	settleQuiet = 50 * time.Millisecond
	dir := t.TempDir()
	wt := &watchTest{
		t:       t,
		reloads: filepath.Join(dir, "reloads"),
		streams: &fakeStreams{},
		done:    make(chan error, 1),
		hupC:    make(chan os.Signal, 1),
	}
	web := writeFile(t, dir, "web.tpl", `{{ .env }} {{ avail "prod/svc/web" }}`)
	web2 := writeFile(t, dir, "web2.tpl", `{{ .env }} {{ avail "prod/svc/web2" }}`)
	outWeb, outWeb2 := filepath.Join(dir, "web"), filepath.Join(dir, "web2")
	cfg := filepath.Join(dir, "templates.yaml")
	writeConfig := func(env string, templates ...[2]string) {
		s := "vars:\n  env: " + env + "\ntemplates:\n"
		for _, tpl := range templates {
			s += "  - in: " + tpl[0] + "\n    out: " + tpl[1] + "\n"
		}
		require.NoError(t, os.WriteFile(cfg, []byte(s), 0o600))
	}
	read := func(p string) string { b, _ := os.ReadFile(p); return string(b) }

	writeConfig("dev", [2]string{web, outWeb})
	wt.run(&CmdTemplate{Config: cfg, Watch: true, Delay: 50 * time.Millisecond, MaxDelay: time.Second, ReloadTimeout: time.Second})
	wt.eventually(func() bool { return read(outWeb) == "dev up" }, "first render")
	wt.eventually(func() bool { _, n := wt.streams.last(); return n == 1 }, "stream opened")
	assert.Equal(t, "prod/svc/web", wt.streams.subs[0].Selector)
	wt.streams.send(t, "ObjectStatusUpdated", objectStatusUpdated("prod/svc/web", status.Up, time.Now()))
	time.Sleep(200 * time.Millisecond)

	// A template added, a var changed: the stream is opened again for the
	// data the new template reads, and both files are rendered from it.
	writeConfig("prod", [2]string{web, outWeb}, [2]string{web2, outWeb2})
	wt.hupC <- os.Interrupt
	wt.eventually(func() bool { _, n := wt.streams.last(); return n == 2 }, "stream opened again")
	assert.Equal(t, "prod/svc/web,prod/svc/web2", wt.streams.subs[1].Selector)
	wt.streams.send(t, "ObjectStatusUpdated", objectStatusUpdated("prod/svc/web", status.Up, time.Now()))
	wt.streams.send(t, "ObjectStatusUpdated", objectStatusUpdated("prod/svc/web2", status.Down, time.Now()))
	wt.eventually(func() bool { return read(outWeb) == "prod up" && read(outWeb2) == "prod down" }, "renders of the new config")

	// An invalid file keeps the former templates.
	require.NoError(t, os.WriteFile(cfg, []byte("templates:\n  - in: "+web+"\n    oops: 1\n"), 0o600))
	wt.hupC <- os.Interrupt
	time.Sleep(200 * time.Millisecond)
	wt.streams.send(t, "ObjectStatusUpdated", objectStatusUpdated("prod/svc/web2", status.Up, time.Now()))
	wt.eventually(func() bool { return read(outWeb2) == "prod up" }, "the former templates still render")
}

// An output holding the content rendered already gets the mode asked still:
// a secret left readable by others is not left so.
func TestTemplateOutputHardensAnUnchangedFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "out.conf")
	require.NoError(t, os.WriteFile(p, []byte("secret"), 0o644))
	out, err := newTemplateOutput(p, "0600", "", "")
	require.NoError(t, err)
	changed, err := out.write([]byte("secret"))
	require.NoError(t, err)
	assert.False(t, changed)
	fi, _ := os.Stat(p)
	assert.Equal(t, fs.FileMode(0o600), fi.Mode().Perm())

	// With no mode asked, the mode of the file is kept.
	require.NoError(t, os.Chmod(p, 0o640))
	out, _ = newTemplateOutput(p, "", "", "")
	_, err = out.write([]byte("secret"))
	require.NoError(t, err)
	fi, _ = os.Stat(p)
	assert.Equal(t, fs.FileMode(0o640), fi.Mode().Perm())
}

// A stream not telling the node serving it, as a daemon proxying the events
// of the --node asked, is served by that node: the object statuses it serves
// are followed.
func TestWatchServedByFallsBackToNode(t *testing.T) {
	settleQuiet = 50 * time.Millisecond
	dir := t.TempDir()
	in := writeFile(t, dir, "in.tpl", `{{ avail "prod/svc/web" }}`)
	wt := &watchTest{t: t, out: filepath.Join(dir, "out"), streams: &fakeStreams{noServedBy: true}, done: make(chan error, 1), hupC: make(chan os.Signal, 1)}
	wt.run(&CmdTemplate{In: in, Out: wt.out, Watch: true, Node: "n1", Delay: 50 * time.Millisecond, MaxDelay: time.Second})
	wt.eventually(func() bool { return wt.content() == "up" }, "first render")
	wt.eventually(func() bool { _, n := wt.streams.last(); return n == 1 }, "stream opened")
	wt.streams.send(t, "ObjectStatusUpdated", objectStatusUpdated("prod/svc/web", status.Up, time.Now()))
	time.Sleep(200 * time.Millisecond)
	wt.streams.send(t, "ObjectStatusUpdated", objectStatusUpdated("prod/svc/web", status.Down, time.Now()))
	wt.eventually(func() bool { return wt.content() == "down" }, "the status of the node asked is followed")
}

// A file that could not be written is written again later, with no other
// change of what the template read.
func TestWatchRetriesAFailedWrite(t *testing.T) {
	writeRetry = 100 * time.Millisecond
	wt := startWatch(t, `{{ avail "prod/svc/web" }}`)
	wt.eventually(func() bool { return wt.content() == "up" }, "first render")
	wt.eventually(func() bool { _, n := wt.streams.last(); return n == 1 }, "stream opened")
	wt.streams.send(t, "ObjectStatusUpdated", objectStatusUpdated("prod/svc/web", status.Up, time.Now()))
	time.Sleep(200 * time.Millisecond)

	// The directory of the file not writable, the write fails.
	dir := filepath.Dir(wt.out)
	require.NoError(t, os.Chmod(dir, 0o500))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	if os.Geteuid() == 0 {
		// root writes in a directory not writable: make the write
		// fail on a directory in the way of the temporary file name
		// instead.
		require.NoError(t, os.Chmod(dir, 0o700))
		require.NoError(t, os.Remove(wt.out))
		require.NoError(t, os.Mkdir(wt.out, 0o700))
		t.Cleanup(func() { _ = os.Remove(wt.out) })
	}
	wt.streams.send(t, "ObjectStatusUpdated", objectStatusUpdated("prod/svc/web", status.Down, time.Now()))
	time.Sleep(300 * time.Millisecond)
	assert.NotEqual(t, "down", wt.content())

	// The file writable again, it is written with no other event.
	if os.Geteuid() == 0 {
		require.NoError(t, os.Remove(wt.out))
	} else {
		require.NoError(t, os.Chmod(dir, 0o700))
	}
	wt.eventually(func() bool { return wt.content() == "down" }, "the write retried")
}

// A reload command shared by templates, one asking no timeout, runs with none.
func TestReloadsOfWithNoTimeout(t *testing.T) {
	for _, jobs := range [][]*templateJob{
		{{out: "a", reloadCommand: "/r", reloadTimeout: time.Minute}, {out: "b", reloadCommand: "/r"}},
		{{out: "a", reloadCommand: "/r"}, {out: "b", reloadCommand: "/r", reloadTimeout: time.Minute}},
	} {
		l := reloadsOf(jobs)
		require.Len(t, l, 1)
		assert.Equal(t, time.Duration(0), l[0].timeout)
	}
}

// The output swapped for a symbolic link after its check is not followed: the
// file it points to keeps its mode.
func TestTemplateOutputHardenDoesNotFollowASymlink(t *testing.T) {
	dir := t.TempDir()
	target := writeFile(t, dir, "target", "secret")
	require.NoError(t, os.Chmod(target, 0o644))
	link := filepath.Join(dir, "out")
	require.NoError(t, os.Symlink(target, link))
	out, err := newTemplateOutput(link, "0600", "", "")
	require.NoError(t, err)
	_, err = out.hardenIfSame([]byte("secret"))
	require.ErrorContains(t, err, "symbolic link")
	fi, _ := os.Stat(target)
	assert.Equal(t, fs.FileMode(0o644), fi.Mode().Perm())
}

// flakyFetcher answers a keyword with an api error the calls of fail, counted
// from 1, the value v otherwise.
type flakyFetcher struct {
	mu    sync.Mutex
	calls int
	fail  map[int]bool
	v     string
}

func (f *flakyFetcher) DataKey(context.Context, naming.Path, string) ([]byte, error) {
	return nil, tmplrender.ErrNotFound
}

func (f *flakyFetcher) DataKeyNames(context.Context, naming.Path) ([]string, error) {
	return nil, tmplrender.ErrNotFound
}

func (f *flakyFetcher) ConfigValue(context.Context, naming.Path, string, string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.fail[f.calls] {
		return "", transient(errors.New("503 Service Unavailable"))
	}
	return f.v, nil
}

func (f *flakyFetcher) ResourceInfo(context.Context, naming.Path, string, string, string) (string, error) {
	return "", tmplrender.ErrNotFound
}

// A render failing on an api error is rendered again with no other event, and
// its retry failing again is retried too.
func TestWatchRetriesARenderFailingOnTheAPI(t *testing.T) {
	writeRetry = 100 * time.Millisecond
	settleQuiet = 50 * time.Millisecond
	dir := t.TempDir()
	in := writeFile(t, dir, "in.tpl", `{{ avail "prod/svc/web" }} {{ config "prod/svc/web" "env.port" }}`)
	f := &flakyFetcher{fail: map[int]bool{3: true, 4: true}, v: "8080"}
	wt := &watchTest{fetcher: f, t: t, out: filepath.Join(dir, "out"), streams: &fakeStreams{}, done: make(chan error, 1), hupC: make(chan os.Signal, 1)}
	wt.run(&CmdTemplate{In: in, Out: wt.out, Watch: true, Delay: 50 * time.Millisecond, MaxDelay: time.Second})
	wt.eventually(func() bool { return wt.content() == "up 8080" }, "first render")
	wt.eventually(func() bool { _, n := wt.streams.last(); return n == 1 }, "stream opened")
	wt.streams.send(t, "ObjectStatusUpdated", objectStatusUpdated("prod/svc/web", status.Up, time.Now()))
	time.Sleep(200 * time.Millisecond) // the forced render after the replay: call 2

	// The web svc down: its render fails on the api (call 3), its retry
	// too (call 4), the next retry writes it, with no other event.
	wt.streams.send(t, "ObjectStatusUpdated", objectStatusUpdated("prod/svc/web", status.Down, time.Now()))
	wt.eventually(func() bool { return wt.content() == "down 8080" }, "the render retried")
	f.mu.Lock()
	defer f.mu.Unlock()
	assert.GreaterOrEqual(t, f.calls, 5)
}

// A template read again on SIGHUP no longer reading cluster data is rendered
// and written at once: no events stream is to replay data for it.
func TestWatchSIGHUPToATemplateReadingNoClusterData(t *testing.T) {
	wt := startWatch(t, `{{ avail "prod/svc/web" }}`)
	wt.eventually(func() bool { return wt.content() == "up" && wt.reloadCount() == 1 }, "first render")
	wt.eventually(func() bool { _, n := wt.streams.last(); return n == 1 }, "stream opened")
	wt.streams.send(t, "ObjectStatusUpdated", objectStatusUpdated("prod/svc/web", status.Up, time.Now()))
	time.Sleep(200 * time.Millisecond)

	in := filepath.Join(filepath.Dir(wt.out), "in.tpl")
	require.NoError(t, os.WriteFile(in, []byte(`static`), 0o600))
	wt.hupC <- os.Interrupt
	wt.eventually(func() bool { return wt.content() == "static" && wt.reloadCount() == 2 }, "render of the static template")
}

// An output the process can write but not read is replaced.
func TestTemplateOutputReplacesAWriteOnlyFile(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a write-only file")
	}
	p := filepath.Join(t.TempDir(), "out.conf")
	require.NoError(t, os.WriteFile(p, []byte("old"), 0o200))
	out, err := newTemplateOutput(p, "0600", "", "")
	require.NoError(t, err)
	changed, err := out.write([]byte("new"))
	require.NoError(t, err)
	assert.True(t, changed)
	b, err := os.ReadFile(p)
	require.NoError(t, err)
	assert.Equal(t, "new", string(b))
}

// sendReplayDone sends the ReplayDone event ending the replay on the last
// stream, with the next id.
func (f *fakeStreams) sendReplayDone(t *testing.T) {
	t.Helper()
	s, _ := f.last()
	f.mu.Lock()
	f.id++
	id := f.id
	f.mu.Unlock()
	s.evC <- &event.Event{Kind: event.KindReplayDone, ID: id, At: time.Now(), Data: []byte(`{"count":1}`)}
}

func startReplayDoneWatch(t *testing.T, text string) *watchTest {
	t.Helper()
	dir := t.TempDir()
	in := writeFile(t, dir, "in.tpl", text)
	wt := &watchTest{
		t:       t,
		out:     filepath.Join(dir, "out"),
		reloads: filepath.Join(dir, "reloads"),
		streams: &fakeStreams{replayDone: true},
		done:    make(chan error, 1),
		hupC:    make(chan os.Signal, 1),
	}
	wt.run(&CmdTemplate{
		In:            in,
		Out:           wt.out,
		Watch:         true,
		ReloadCommand: writeScript(t, dir, "reload", "echo >> "+wt.reloads),
		Delay:         50 * time.Millisecond,
		MaxDelay:      100 * time.Millisecond,
		ReloadTimeout: time.Second,
	})
	return wt
}

// A stream ending its replay with a ReplayDone event is rendered at it: not
// before, whatever the time the replay takes, not even past --max-delay, and
// at once, with no quiet period.
func TestWatchRendersAtTheReplayDone(t *testing.T) {
	settleQuiet = time.Hour
	t.Cleanup(func() { settleQuiet = 50 * time.Millisecond })
	wt := startReplayDoneWatch(t, `{{ avail "prod/svc/web" }}`)
	wt.eventually(func() bool { return wt.content() == "up" }, "first render")
	wt.eventually(func() bool { _, n := wt.streams.last(); return n == 1 }, "stream opened")

	// A replay slower than --max-delay: no render from the partial data.
	wt.streams.send(t, "ObjectStatusUpdated", objectStatusUpdated("prod/svc/web", status.Down, time.Now()))
	time.Sleep(300 * time.Millisecond)
	assert.Equal(t, "up", wt.content(), "no render before the end of the replay")

	wt.streams.sendReplayDone(t)
	wt.eventually(func() bool { return wt.content() == "down" }, "render at the end of the replay")

	// The live events after it are rendered as before.
	wt.streams.send(t, "ObjectStatusUpdated", objectStatusUpdated("prod/svc/web", status.Up, time.Now()))
	wt.eventually(func() bool { return wt.content() == "up" }, "render of a live event")
}

// A stream advertising the ReplayDone event not sending it is opened again.
func TestWatchReopensWithNoReplayDone(t *testing.T) {
	replayDoneTimeout = 200 * time.Millisecond
	t.Cleanup(func() { replayDoneTimeout = 30 * time.Second })
	wt := startReplayDoneWatch(t, `{{ avail "prod/svc/web" }}`)
	wt.eventually(func() bool { _, n := wt.streams.last(); return n == 1 }, "stream opened")
	wt.eventually(func() bool { _, n := wt.streams.last(); return n >= 2 }, "stream opened again")

	// The new stream ending its replay renders.
	wt.streams.send(t, "ObjectStatusUpdated", objectStatusUpdated("prod/svc/web", status.Down, time.Now()))
	wt.streams.sendReplayDone(t)
	wt.eventually(func() bool { return wt.content() == "down" }, "render at the end of the replay")
}

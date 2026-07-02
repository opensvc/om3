// Package mntmon is responsible for monitoring filesystem mount/umount/remount events
// and publishing them as pubsub events.
//
// It watches /proc/self/mountinfo for changes via the kernel's POLLPRI/POLLERR wakeup mechanism.
package mntmon

import (
	"bufio"
	"context"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/opensvc/om3/v3/daemon/msgbus"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/plog"
	"github.com/opensvc/om3/v3/util/pubsub"
	"golang.org/x/sys/unix"
)

// mountEntry is a minimal parse of a /proc/*/mountinfo line, keyed by
// mount ID so remounts of the same path are still distinguishable.
type mountEntry struct {
	id, mountPoint, fsType, source, options string
}

func parseMountinfo(path string) (map[string]mountEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	entries := make(map[string]mountEntry)
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 10 {
			continue
		}
		// Fields: id parent major:minor root mountpoint opts ... - fstype source ...
		sepIdx := -1
		for i, fld := range fields {
			if fld == "-" {
				sepIdx = i
				break
			}
		}
		if sepIdx == -1 || sepIdx+2 >= len(fields) {
			continue
		}
		// fields[5] = per-mount options (e.g. "rw,relatime")
		// fields[sepIdx+3], if present = super options (e.g. "rw,errors=remount-ro")
		superOpts := ""
		if sepIdx+3 < len(fields) {
			superOpts = fields[sepIdx+3]
		}
		e := mountEntry{
			id:         fields[0],
			mountPoint: fields[4],
			fsType:     fields[sepIdx+1],
			source:     fields[sepIdx+2],
			options:    fields[5] + "|" + superOpts,
		}
		entries[e.id] = e
	}
	return entries, sc.Err()
}

type (
	// Manager monitors mount/umount events and publishes them to the message bus
	Manager struct {
		drainDuration time.Duration

		ctx    context.Context
		cancel context.CancelFunc
		log    *plog.Logger

		publisher pubsub.Publisher
		databus   interface{}
		sub       *pubsub.Subscription
		subQS     pubsub.QueueSizer

		localhost      string
		labelLocalhost pubsub.Label

		wg sync.WaitGroup
	}
)

// NewManager creates a new mount monitor manager
func NewManager(drainDuration time.Duration, subQS pubsub.QueueSizer) *Manager {
	localhost := hostname.Hostname()
	return &Manager{
		drainDuration:  drainDuration,
		log:            plog.NewDefaultLogger().Attr("pkg", "daemon/mntmon").WithPrefix("daemon: mntmon: "),
		localhost:      localhost,
		labelLocalhost: pubsub.Label{"node", localhost},
		subQS:          subQS,
	}
}

// Start launches the mntmon worker goroutine
func (t *Manager) Start(parent context.Context) error {
	t.log.Infof("starting")
	t.ctx, t.cancel = context.WithCancel(parent)
	t.publisher = pubsub.PubFromContext(t.ctx)

	// Start pubsub subscriptions for audit and other control messages
	t.startSubscriptions()

	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		defer t.log.Infof("mount monitor done")
		t.watchMounts()
	}()

	t.log.Infof("started")
	return nil
}

// Stop stops the mntmon manager
func (t *Manager) Stop() error {
	t.log.Infof("stopping")
	defer t.log.Infof("stopped")
	t.cancel()
	if t.sub != nil {
		if err := t.sub.Stop(); err != nil {
			t.log.Warnf("subscription stop: %s", err)
		}
	}
	t.wg.Wait()
	return nil
}

// startSubscriptions starts the pubsub subscriptions for control messages like AuditStart/AuditStop
func (t *Manager) startSubscriptions() {
	sub := pubsub.SubFromContext(t.ctx, "daemon.mntmon", t.subQS)

	sub.AddFilter(&msgbus.AuditStart{})
	sub.AddFilter(&msgbus.AuditStop{})

	sub.Start()
	t.sub = sub

	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		for {
			select {
			case <-t.ctx.Done():
				return
			case ev := <-sub.C:
				switch c := ev.(type) {
				case *msgbus.AuditStart:
					t.log.HandleAuditStart(c.Q, c.Subsystems, "mntmon")
				case *msgbus.AuditStop:
					t.log.HandleAuditStop(c.Q, c.Subsystems, "mntmon")
				}
			}
		}
	}()
}

// watchMounts watches /proc/self/mountinfo for changes and emits msgbus events
func (t *Manager) watchMounts() {
	t.log.Infof("starting mount monitor")

	mountinfoPath := "/proc/self/mountinfo"

	fd, err := unix.Open(mountinfoPath, unix.O_RDONLY, 0)
	if err != nil {
		t.log.Errorf("open %s: %s", mountinfoPath, err)
		return
	}
	defer unix.Close(fd)

	prev, err := parseMountinfo(mountinfoPath)
	if err != nil {
		t.log.Errorf("parse mountinfo: %s", err)
		return
	}

	pfd := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLERR | unix.POLLPRI}}

	// Use a finite poll timeout to allow checking context cancellation
	const pollTimeout = 300 * time.Millisecond

	for {
		select {
		case <-t.ctx.Done():
			t.log.Infof("context done, stopping mount monitor")
			return
		default:
		}

		// Poll with finite timeout
		_, err := unix.Poll(pfd, int(pollTimeout.Milliseconds()))
		if err != nil {
			if err == unix.EINTR {
				continue
			}
			t.log.Errorf("poll: %s", err)
			return
		}

		// Check context after poll returns (either due to event or timeout)
		select {
		case <-t.ctx.Done():
			t.log.Infof("context done, stopping mount monitor")
			return
		default:
		}

		cur, err := parseMountinfo(mountinfoPath)
		if err != nil {
			t.log.Errorf("parse mountinfo: %s", err)
			continue
		}

		// Publish mount and remount events
		for id, e := range cur {
			old, existed := prev[id]
			switch {
			case !existed:
				msg := &msgbus.FSMounted{
					Node:       t.localhost,
					MountPoint: e.mountPoint,
					FSType:     e.fsType,
					Source:     e.source,
					Options:    e.options,
				}
				t.log.Infof("filesystem mounted: %s (type=%s, source=%s)", e.mountPoint, e.fsType, e.source)
				t.publisher.Pub(msg, t.labelLocalhost)
			case old.options != e.options:
				msg := &msgbus.FSRemounted{
					Node:       t.localhost,
					MountPoint: e.mountPoint,
					FSType:     e.fsType,
					Source:     e.source,
					Options:    e.options,
				}
				t.log.Infof("filesystem remounted: %s (type=%s, source=%s)", e.mountPoint, e.fsType, e.source)
				t.publisher.Pub(msg, t.labelLocalhost)
			}
		}

		// Publish umount events
		for id, e := range prev {
			if _, ok := cur[id]; !ok {
				msg := &msgbus.FSUmounted{
					Node:       t.localhost,
					MountPoint: e.mountPoint,
					FSType:     e.fsType,
					Source:     e.source,
					Options:    e.options,
				}
				t.log.Infof("filesystem unmounted: %s (type=%s, source=%s)", e.mountPoint, e.fsType, e.source)
				t.publisher.Pub(msg, t.labelLocalhost)
			}
		}

		prev = cur
	}
}

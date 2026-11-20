package discover

import (
	"context"
	"sync"

	"github.com/opensvc/om3/v3/core/naming"
)

type (
	// keyInstaller refreshes the keys a datastore installs in the volumes
	// of the local instances, when its configuration is fetched from a peer.
	//
	// The node writing a key refreshes its own volumes, in the process
	// writing it. The other nodes receive the new configuration through the
	// daemon, and nothing else would install the new content, or send the
	// signals the install lines declare: a haproxy reloading on a change of
	// its configuration key would keep the old one on every node but the
	// one the key was changed on.
	//
	// The refreshes wait in a queue, and at most node.max_parallel of them
	// are submitted at once to the runner, whose slots they share with the
	// actions of the orchestrations: many datastores fetched at once, as
	// when hundreds of secrets are created, do not start as many install
	// commands, which would starve the daemon of cpu, nor queue as many
	// actions in the runner ahead of the orchestrations.
	//
	// A refresh asked for a datastore waiting for its run, in the queue or
	// in the runner, is the waiting one: it reads the newest configuration
	// when it runs. A refresh asked while one runs is queued again once it
	// ends, at the tail, so a datastore changing in a loop does not hold a
	// slot while others wait.
	//
	// A refresh submitted decides first whether a local instance may
	// install the keys, and does not run when none may. A refresh asked
	// meanwhile may be for an instance that just started using the
	// datastore, which the decision may not have seen: one submitted and
	// ending without running is queued again when asked meanwhile, to
	// decide again.
	keyInstaller struct {
		ctx context.Context

		// maxParallel returns the most refreshes submitted at once.
		maxParallel func() int

		// refresh refreshes the keys of p, calling starting when its run
		// starts, not at all when it does not run.
		refresh func(ctx context.Context, p naming.Path, starting func())

		mu sync.Mutex

		// queue are the datastores waiting to be submitted, in order.
		queue []naming.Path

		// waiting are the datastores in the queue, or submitted and not
		// started yet.
		waiting map[naming.Path]bool

		// running are the datastores whose refresh started.
		running map[naming.Path]bool

		// submittedWaiting are the datastores submitted and not started yet.
		submittedWaiting map[naming.Path]bool

		// again are the datastores submitted, or running, asked a refresh
		// again.
		again map[naming.Path]bool

		// submitted is the number of refreshes submitted and not ended.
		submitted int
	}
)

func newKeyInstaller(ctx context.Context, maxParallel func() int, refresh func(context.Context, naming.Path, func())) *keyInstaller {
	return &keyInstaller{
		ctx:              ctx,
		maxParallel:      maxParallel,
		refresh:          refresh,
		waiting:          make(map[naming.Path]bool),
		submittedWaiting: make(map[naming.Path]bool),
		running:          make(map[naming.Path]bool),
		again:            make(map[naming.Path]bool),
	}
}

// request asks a refresh of the keys of the datastore p.
func (ki *keyInstaller) request(p naming.Path) {
	ki.mu.Lock()
	defer ki.mu.Unlock()
	switch {
	case ki.submittedWaiting[p]:
		ki.again[p] = true
		return
	case ki.waiting[p]:
		return
	case ki.running[p]:
		ki.again[p] = true
		return
	}
	ki.push(p)
	ki.dispatch()
}

func (ki *keyInstaller) push(p naming.Path) {
	ki.waiting[p] = true
	ki.queue = append(ki.queue, p)
}

// dispatch submits the queued refreshes while less than max parallel are.
// It is called with mu held.
func (ki *keyInstaller) dispatch() {
	if ki.ctx.Err() != nil {
		for _, p := range ki.queue {
			delete(ki.waiting, p)
		}
		ki.queue = nil
		return
	}
	if len(ki.queue) == 0 {
		return
	}
	maxParallel := ki.maxParallel()
	for len(ki.queue) > 0 && ki.submitted < maxParallel {
		p := ki.queue[0]
		ki.queue[0] = naming.Path{}
		ki.queue = ki.queue[1:]
		ki.submitted++
		ki.submittedWaiting[p] = true
		go ki.do(p)
	}
}

func (ki *keyInstaller) do(p naming.Path) {
	ki.refresh(ki.ctx, p, func() {
		ki.mu.Lock()
		defer ki.mu.Unlock()
		delete(ki.waiting, p)
		delete(ki.submittedWaiting, p)
		// The run reads the newest configuration: a refresh asked
		// before it started is this one.
		delete(ki.again, p)
		ki.running[p] = true
	})
	ki.mu.Lock()
	defer ki.mu.Unlock()
	ki.submitted--
	// A refresh skipped never started: it was still waiting.
	delete(ki.waiting, p)
	delete(ki.submittedWaiting, p)
	delete(ki.running, p)
	if ki.again[p] {
		delete(ki.again, p)
		ki.push(p)
	}
	ki.dispatch()
}

package object

import (
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/util/proc"
)

// interruptSyncsGrace is how long the syncs signaled to end have to end,
// before they are killed.
const interruptSyncsGrace = 10 * time.Second

// interruptSyncs ends the syncs running on the local instance, for an action
// that must neither wait for them nor leave them running, as the shutdown of a
// drained node: a sync left running keeps sending to the peers while one of
// them takes over, and writes into the data of the node now running the
// object.
//
// A sync is the om process of its run file and every process it started, as
// the rsync and ssh it runs, which would outlive it: they are all signaled.
func (t *actor) interruptSyncs() {
	var (
		rids []string
		pids []int
	)
	for _, r := range listResources(t) {
		if r.ID().DriverGroup() != driver.GroupSync {
			continue
		}
		i, ok := r.(resource.Runninger)
		if !ok {
			continue
		}
		l, err := i.Running()
		if err != nil {
			t.log.Warnf("%s: running syncs: %s", r.RID(), err)
		}
		for _, info := range l {
			if info.PID == 0 || info.PID == os.Getpid() {
				continue
			}
			rids = append(rids, r.RID())
			pids = append(pids, info.PID)
		}
	}
	if len(pids) == 0 {
		return
	}
	t.log.Infof("interrupt the syncs running: %s", strings.Join(rids, ","))
	var tree []int
	for _, pid := range pids {
		tree = append(tree, proc.Tree(pid)...)
	}
	signal := func(sig syscall.Signal) (alive int) {
		for _, pid := range tree {
			if err := syscall.Kill(pid, sig); err == nil {
				alive++
			}
		}
		return
	}
	signal(syscall.SIGTERM)
	deadline := time.Now().Add(interruptSyncsGrace)
	for time.Now().Before(deadline) {
		time.Sleep(200 * time.Millisecond)
		if signal(0) == 0 {
			return
		}
	}
	if n := signal(syscall.SIGKILL); n > 0 {
		t.log.Warnf("killed %d processes of the syncs still running after %s", n, interruptSyncsGrace)
	}
}

package instance

import (
	"fmt"
	"slices"
	"strings"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/resourceid"
	"github.com/opensvc/om3/v3/core/status"
)

// ReferenceStatus is the status of a reference resource of an instance, one
// of the resources telling the active instance of an object.
type ReferenceStatus struct {
	RID      string
	Status   status.T
	Optional bool
}

// IsReferenceResource reports whether a resource of the group and driver name
// tells the active instance: the ones holding what the data lives on or is
// reached by, that is every resource but the app, sync and task ones, and the
// disk.scsireserv and disk.drbd ones, which are up on the passive nodes too.
func IsReferenceResource(group driver.Group, name string) bool {
	switch group {
	case driver.GroupIP, driver.GroupVolume, driver.GroupFS, driver.GroupShare, driver.GroupContainer:
		return true
	case driver.GroupDisk:
		switch name {
		case "drbd", "scsireserv":
			return false
		}
		return true
	default:
		return false
	}
}

// IsReplicationSource decides from the status of the reference resources of
// an instance whether it is the one the data is replicated from.
//
// As v2 did, their aggregated availability must be up, or not applicable with
// their overall status up. An instance with no reference resource gives no
// way to tell the active one from the others, so none is. force makes an
// instance whose reference resources are neither down nor not applicable the
// source anyway, forced saying so, and reason why it would not be otherwise.
func IsReplicationSource(refs []ReferenceStatus, force bool) (ok bool, reason string, forced bool) {
	avail, overall := status.Undef, status.Undef
	var notUp []string
	for _, r := range refs {
		overall.Add(r.Status)
		if !r.Optional {
			avail.Add(r.Status)
		}
		if r.Status != status.Up {
			notUp = append(notUp, fmt.Sprintf("%s:%s", r.RID, r.Status))
		}
	}
	if avail == status.StandbyUpWithUp {
		avail = status.Up
	}
	if overall == status.StandbyUpWithUp {
		overall = status.Up
	}
	switch {
	case avail == status.Up:
		return true, "", false
	case avail.Is(status.Undef, status.NotApplicable) && overall == status.Up:
		return true, "", false
	case overall == status.Undef:
		return false, "no ip, volume, fs, share, disk or container resource tells the active instance", false
	}
	reason = fmt.Sprintf("reference resources %s/%s: %s", avail, overall, strings.Join(notUp, ","))
	if force && !avail.Is(status.Down, status.NotApplicable, status.Undef) && !overall.Is(status.Down, status.NotApplicable, status.Undef) {
		return true, reason, true
	}
	return false, reason, false
}

// ReplicationSource says whether the instance is the one the data is
// replicated from, reading its reference resources from its status, and why
// not when it is not.
func (t Status) ReplicationSource() (bool, string) {
	refs := make([]ReferenceStatus, 0, len(t.Resources))
	for rid, r := range t.Resources {
		if r.IsDisabled {
			continue
		}
		id, err := resourceid.Parse(rid)
		if err != nil {
			continue
		}
		_, name, _ := strings.Cut(r.Type, ".")
		if !IsReferenceResource(id.DriverGroup(), name) {
			continue
		}
		refs = append(refs, ReferenceStatus{RID: rid, Status: r.Status, Optional: r.IsOptional})
	}
	slices.SortFunc(refs, func(a, b ReferenceStatus) int { return strings.Compare(a.RID, b.RID) })
	ok, reason, _ := IsReplicationSource(refs, false)
	return ok, reason
}

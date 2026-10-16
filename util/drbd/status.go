package drbd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rs/zerolog"

	"github.com/opensvc/om3/v3/util/command"
)

const (
	// DiskStateUpToDate is the disk state of a replica holding the latest
	// data, the only one a promotion accepts.
	DiskStateUpToDate = "UpToDate"
)

type (
	// Status is the state of a resource as drbdsetup status reports it, the
	// part a start reads: the disk of each local volume, and for each peer
	// the connection and how far behind its resync is.
	Status struct {
		Name        string             `json:"name"`
		Role        string             `json:"role"`
		Devices     []StatusDevice     `json:"devices"`
		Connections []StatusConnection `json:"connections"`
	}

	StatusDevice struct {
		Volume    int    `json:"volume"`
		DiskState string `json:"disk-state"`
	}

	StatusConnection struct {
		PeerNodeID      int                    `json:"peer-node-id"`
		Name            string                 `json:"name"`
		ConnectionState string                 `json:"connection-state"`
		PeerDevices     []StatusConnPeerDevice `json:"peer_devices"`
	}

	StatusConnPeerDevice struct {
		Volume           int     `json:"volume"`
		ReplicationState string  `json:"replication-state"`
		PeerDiskState    string  `json:"peer-disk-state"`
		OutOfSync        uint64  `json:"out-of-sync"`
		PercentInSync    float64 `json:"percent-in-sync"`
	}
)

// ParseStatus reads the output of drbdsetup status --json --statistics for a
// resource.
func ParseStatus(b []byte) (Status, error) {
	var l []Status
	if err := json.Unmarshal(b, &l); err != nil {
		return Status{}, err
	}
	if len(l) == 0 {
		return Status{}, fmt.Errorf("no drbd resource found")
	}
	return l[0], nil
}

// Status returns the state of the resource, its disks, its connections and
// their resync.
func (t *T) Status(ctx context.Context) (Status, error) {
	cmd := command.New(
		command.WithName(drbdsetup),
		command.WithVarArgs("status", "--json", "--statistics", t.res),
		command.WithBufferedStdout(),
		command.WithContext(ctx),
		command.WithLogger(t.log),
		command.WithCommandLogLevel(zerolog.TraceLevel),
	)
	b, err := cmd.Output()
	if err != nil {
		return Status{}, err
	}
	return ParseStatus(b)
}

// IsUpToDate says whether every local volume holds the latest data, which a
// promotion needs.
func (t Status) IsUpToDate() bool {
	if len(t.Devices) == 0 {
		return false
	}
	for _, d := range t.Devices {
		if d.DiskState != DiskStateUpToDate {
			return false
		}
	}
	return true
}

// DiskStates returns the disk states of the local volumes, joined.
func (t Status) DiskStates() string {
	l := make([]string, len(t.Devices))
	for i, d := range t.Devices {
		l[i] = d.DiskState
	}
	return strings.Join(l, "/")
}

// HasConnected says whether a peer is connected, which a resync needs.
func (t Status) HasConnected() bool {
	for _, c := range t.Connections {
		if c.ConnectionState == ConnStateConnected {
			return true
		}
	}
	return false
}

// HasUpToDatePeer says whether a connected peer holds the latest data, the
// source a resync of the local disk needs.
func (t Status) HasUpToDatePeer() bool {
	for _, c := range t.Connections {
		if c.ConnectionState != ConnStateConnected {
			continue
		}
		for _, pd := range c.PeerDevices {
			if pd.PeerDiskState == DiskStateUpToDate {
				return true
			}
		}
	}
	return false
}

// OutOfSync returns the data, in KiB, still to resync from the peers, the
// largest a peer reports for each volume.
func (t Status) OutOfSync() uint64 {
	perVolume := make(map[int]uint64)
	for _, c := range t.Connections {
		for _, pd := range c.PeerDevices {
			if pd.OutOfSync > perVolume[pd.Volume] {
				perVolume[pd.Volume] = pd.OutOfSync
			}
		}
	}
	var n uint64
	for _, v := range perVolume {
		n += v
	}
	return n
}

// Peers says, for each peer, its connection, its replication and its disk:
// "n2 Connected SyncTarget UpToDate".
func (t Status) Peers() string {
	l := make([]string, 0, len(t.Connections))
	for _, c := range t.Connections {
		s := fmt.Sprintf("%s %s", c.Name, c.ConnectionState)
		for _, pd := range c.PeerDevices {
			s += fmt.Sprintf(" %s %s", pd.ReplicationState, pd.PeerDiskState)
		}
		l = append(l, s)
	}
	return strings.Join(l, ", ")
}

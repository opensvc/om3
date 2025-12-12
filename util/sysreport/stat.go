package sysreport

import (
	"encoding/json"
	"io"
	"sort"

	"github.com/opensvc/om3/v3/util/timestamp"
	"github.com/opensvc/om3/v3/util/xmap"
)

type (
	// Stat contains the metadata of a tracked file
	Stat struct {
		Path       string `json:"fpath"`    // /opt/opensvc/etc/vcluster.conf
		RealPath   string `json:"realpath"` // /opt/opensvc/etc/vcluster.conf
		Dev        Dev    `json:"dev"`      // 66306
		UID        uint32 `json:"uid"`      // 0
		GID        uint32 `json:"gid"`      // 0
		ModeOctStr string `json:"mode"`     // "0o100600",
		Mode       Mode
		Size       int64
		CTime      timestamp.T `json:"ctime"` // 1640331980
		MTime      timestamp.T `json:"mtime"` // 1640331980
		Nlink      Nlink       `json:"nlink"` // 1
	}

	// Stats is the list of the metadata of all tracked files.
	// The serialized Stats is tracked as a special file, so tracked files metadata changes are also tracked.
	Stats []Stat

	statsMap map[string]Stat
)

// List returns the StatsMap data as a list of Stat sorted by Stat.Path,
// which is also the StatsMap key
func (t statsMap) List() Stats {
	l := make(Stats, len(t))
	keys := xmap.Keys(t)
	sort.Strings(keys)
	for i, key := range keys {
		l[i] = t[key]
	}
	return l
}

func (t *statsMap) Load(r io.Reader) error {
	var stats Stats
	if err := stats.Load(r); err != nil {
		return err
	}
	*t = stats.Map()
	return nil
}

func (t statsMap) Write(w io.Writer) error {
	return t.List().Write(w)
}

func (t Stats) Map() statsMap {
	m := make(statsMap)
	for _, stat := range t {
		m[stat.Path] = stat
	}
	return m
}

func (t *Stats) Load(r io.Reader) error {
	dec := json.NewDecoder(r)
	return dec.Decode(t)
}

func (t Stats) Write(w io.Writer) error {
	enc := json.NewEncoder(w)
	return enc.Encode(t)
}

func (t Stat) IsEqual(o Stat) bool {
	switch {
	case t.Path != o.Path:
		return false
	case t.RealPath != o.RealPath:
		return false
	case t.Dev != o.Dev:
		return false
	case t.UID != o.UID:
		return false
	case t.GID != o.GID:
		return false
	case t.ModeOctStr != o.ModeOctStr:
		return false
	// b2.1 did not store Size, avoid false negative
	//case t.Size != o.Size:
	//	return false
	case t.CTime != o.CTime:
		return false
	case t.MTime != o.MTime:
		return false
	case t.Nlink != o.Nlink:
		return false
	default:
		return true
	}
}

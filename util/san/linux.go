// +build linux

package san

import (
	"os"
	"path/filepath"
	"strings"

	"opensvc.com/opensvc/util/file"
)

func Paths() ([]Path, error) {
	return []Path{}, nil
}

func HostBusAdapters() ([]HostBusAdapter, error) {
	l := make([]HostBusAdapter, 0)
	if more, err := FCHostBusAdapters(); err == nil {
		l = append(l, more...)
	} else {
		return l, err
	}
	if more, err := ISCSIHostBusAdapters(); err == nil {
		l = append(l, more...)
	} else {
		return l, err
	}
	return l, nil
}

func ISCSIHostBusAdapters() ([]HostBusAdapter, error) {
	l := make([]HostBusAdapter, 0)
	hba := HostBusAdapter{
		Type: ISCSI,
	}
	p := "/etc/iscsi/initiatorname.iscsi"
	b, err := file.ReadAll(p)
	if err != nil {
		return l, err
	}
	s := string(b)
	w := strings.Split(s, "=")
	if len(w) < 2 {
		return l, err
	}
	hba.ID = strings.TrimRight(w[1], "\n\r")
	l = append(l, hba)
	return l, nil
}

func FCHostBusAdapters() ([]HostBusAdapter, error) {
	l := make([]HostBusAdapter, 0)
	matches, err := filepath.Glob("/sys/class/fc_host/host*/port_name")
	if err != nil {
		return l, err
	}
	for _, m := range matches {
		hba := HostBusAdapter{}
		hostLink := filepath.Dir(m)
		hostLinkTarget, err := os.Readlink(hostLink)
		if err != nil {
			return l, err
		}
		if strings.Contains(hostLinkTarget, "/eth") {
			hba.Type = FCOE
		} else {
			hba.Type = FC
		}
		if b, err := file.ReadAll(m); err != nil {
			return l, err
		} else {
			id := string(b)
			id = strings.TrimRight(id, "\n\r")
			hba.ID = id
		}
		l = append(l, hba)
	}
	return l, nil
}

//go:build linux

package san

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"opensvc.com/opensvc/util/command"
)

func isPortPresent(tp string) bool {
	p := filepath.Dir(tp) + "/port_state"
	buff, err := os.ReadFile(p)
	if err != nil {
		return false
	}
	return !strings.Contains(string(buff), "Not Present")
}

func GetPaths() ([]Path, error) {
	l := make([]Path, 0)
	if paths, err := GetFCPaths(); err == nil {
		l = append(l, paths...)
	} else {
		return l, err
	}
	if paths, err := GetISCSIPaths(); err == nil {
		l = append(l, paths...)
	} else {
		return l, err
	}
	return l, nil
}

func GetFCPaths() ([]Path, error) {
	l := make([]Path, 0)
	hbas, err := GetHostBusAdapters()
	if err != nil {
		return l, err
	}
	for _, hba := range hbas {
		if hba.Type != "fc" {
			continue
		}
		tps := make([]string, 0)
		p := fmt.Sprintf("/sys/class/fc_transport/target%s:*/port_name", hba.Host)
		if some, err := filepath.Glob(p); err == nil {
			tps = append(tps, some...)
		}
		p = fmt.Sprintf("/sys/class/fc_remote_ports/rport-%s:*/port_name", hba.Host)
		if some, err := filepath.Glob(p); err == nil {
			tps = append(tps, some...)
		}
		for _, tp := range tps {
			b, err := os.ReadFile(tp)
			if err != nil {
				continue
			}
			id := string(b)
			id = strings.TrimSpace(id)
			id = strings.Replace(id, "0x", "", 1)
			if !isPortPresent(tp) {
				continue
			}
			l = append(l, Path{
				HostBusAdapter: hba,
				TargetPort: TargetPort{
					ID: id,
				},
			})
		}
	}
	return l, nil
}

func GetISCSIPaths() ([]Path, error) {
	l := make([]Path, 0)
	hbas, err := GetISCSIHostBusAdapters()
	if err != nil {
		return l, err
	}
	if len(hbas) == 0 {
		return l, nil
	}
	hba := hbas[0]
	buff, err := iscsiadmSession()
	if err != nil {
		return l, err
	}
	for _, line := range strings.Split(buff, "\n") {
		v := strings.Fields(line)
		for i := len(v); i > 0; i -= 1 {
			id := v[i]
			if !strings.HasPrefix(id, "iqn.") {
				continue
			}
			l = append(l, Path{
				HostBusAdapter: hba,
				TargetPort: TargetPort{
					ID: id,
				},
			})
		}
	}
	return l, nil
}

func iscsiadmSession() (string, error) {
	cmd := command.New(
		command.WithName("iscsiadm"),
		command.WithVarArgs("-m", "session"),
	)
	b, err := cmd.Output()
	return string(b), err
}

func GetHostBusAdapters() ([]HostBusAdapter, error) {
	l := make([]HostBusAdapter, 0)
	if more, err := GetFCHostBusAdapters(); err == nil {
		l = append(l, more...)
	} else {
		return l, err
	}
	if more, err := GetISCSIHostBusAdapters(); err == nil {
		l = append(l, more...)
	} else {
		return l, err
	}
	return l, nil
}

func GetISCSIHostBusAdapters() ([]HostBusAdapter, error) {
	l := make([]HostBusAdapter, 0)
	hba := HostBusAdapter{
		Type: ISCSI,
	}
	p := "/etc/iscsi/initiatorname.iscsi"
	b, err := os.ReadFile(p)
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

func GetFCHostBusAdapters() ([]HostBusAdapter, error) {
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
		if b, err := os.ReadFile(m); err != nil {
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

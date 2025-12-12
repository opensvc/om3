//go:build linux

package san

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/opensvc/om3/v3/util/command"
)

func GetPaths() (Paths, error) {
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

func isPortPresent(d string) bool {
	p := d + "/port_state"
	buff, err := os.ReadFile(p)
	if err != nil {
		return false
	}
	return !strings.Contains(string(buff), "Not Present")
}

func readWWPN(d string) (string, error) {
	p := d + "/port_name"
	b, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	id := string(b)
	id = strings.TrimSpace(id)
	id = strings.Replace(id, "0x", "", 1)
	return id, nil
}

func GetFCPaths() ([]Path, error) {
	l := make([]Path, 0)
	matches := make([]string, 0)
	if some, err := filepath.Glob("/sys/class/fc_transport/target*"); err == nil {
		matches = append(matches, some...)
	}
	if some, err := filepath.Glob("/sys/class/fc_remote_ports/rport-*"); err == nil {
		matches = append(matches, some...)
	}
	for _, d := range matches {
		wwpn, err := readWWPN(d)
		if err != nil {
			continue
		}
		if wwpn == "0" {
			continue
		}
		if !isPortPresent(d) {
			continue
		}
		hbtl := d
		hbtl = strings.TrimPrefix(hbtl, "/sys/class/fc_transport/target")
		hbtl = strings.TrimPrefix(hbtl, "/sys/class/fc_remote_ports/rport-")
		host := "host" + hbtl[0:strings.Index(hbtl, ":")]
		if err != nil {
			continue
		}
		initiator, err := GetFCInitiator("/sys/class/fc_host/" + host)
		if err != nil {
			continue
		}
		l = append(l, Path{
			Initiator: initiator,
			Target: Target{
				Type: FC,
				Name: wwpn,
			},
		})
	}
	return l, nil
}

func GetISCSIPaths() (Paths, error) {
	l := make(Paths, 0)
	initiators, err := GetISCSIInitiators()
	if err != nil {
		return l, err
	}
	if len(initiators) == 0 {
		return l, nil
	}
	initiator := initiators[0]
	buff, err := iscsiadmSession()
	if err != nil {
		return l, err
	}
	for _, line := range strings.Split(buff, "\n") {
		v := strings.Fields(line)
		for i := len(v) - 1; i >= 0; i-- {
			name := v[i]
			if !strings.HasPrefix(name, "iqn.") {
				continue
			}
			l = append(l, Path{
				Initiator: initiator,
				Target: Target{
					Type: ISCSI,
					Name: name,
				},
			})
		}
	}
	return l, nil
}

// iscsiadmSession return the output of the iscsiadm session listing command.
// The 21 exitcode is ignored because it means "no record found".
func iscsiadmSession() (string, error) {
	cmd := command.New(
		command.WithName("iscsiadm"),
		command.WithVarArgs("-m", "session"),
		command.WithBufferedStdout(),
		command.WithIgnoredExitCodes(0, 21),
	)
	b, err := cmd.Output()
	return string(b), err
}

func GetInitiators() ([]Initiator, error) {
	l := make([]Initiator, 0)
	if more, err := GetFCInitiators(); err == nil {
		l = append(l, more...)
	} else {
		return l, err
	}
	if more, err := GetISCSIInitiators(); err == nil {
		l = append(l, more...)
	} else {
		return l, err
	}
	return l, nil
}

func GetISCSIInitiators() ([]Initiator, error) {
	l := make([]Initiator, 0)
	initiator := Initiator{
		Type: ISCSI,
	}
	p := "/etc/iscsi/initiatorname.iscsi"
	b, err := os.ReadFile(p)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return l, nil
	case err != nil:
		return l, err
	}
	s := string(b)
	w := strings.Split(s, "=")
	if len(w) < 2 {
		return l, err
	}
	initiator.Name = strings.TrimRight(w[1], "\n\r")
	l = append(l, initiator)
	return l, nil
}

func GetFCInitiator(hostLink string) (Initiator, error) {
	initiator := Initiator{}
	host := filepath.Base(hostLink)
	initiator.Name = host
	hostLinkTarget, err := os.Readlink(hostLink)
	if err != nil {
		return initiator, err
	}
	if strings.Contains(hostLinkTarget, "/eth") {
		initiator.Type = FCOE
	} else {
		initiator.Type = FC
	}
	if b, err := os.ReadFile(hostLink + "/port_name"); err != nil {
		return initiator, err
	} else {
		id := string(b)
		id = strings.TrimRight(id, "\n\r")
		initiator.Name = id
	}
	return initiator, nil
}

func GetFCInitiators() ([]Initiator, error) {
	l := make([]Initiator, 0)
	matches, err := filepath.Glob("/sys/class/fc_host/host*")
	if err != nil {
		return l, err
	}
	for _, m := range matches {
		if initiator, err := GetFCInitiator(m); err != nil {
			return l, err
		} else {
			l = append(l, initiator)
		}
	}
	return l, nil
}

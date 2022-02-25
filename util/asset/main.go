package asset

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	ErrNotImpl = fmt.Errorf("not implemented")
	ErrIgnore  = fmt.Errorf("ignore")
)

type (
	T struct{}

	Device struct {
		Path        string `json:"path"`
		Description string `json:"description"`
		Class       string `json:"class"`
		Driver      string `json:"driver"`
		Type        string `json:"type"`
	}

	Group struct {
		ID   int    `json:"gid"`
		Name string `json:"groupname"`
	}

	User struct {
		ID   int    `json:"uid"`
		Name string `json:"username"`
	}

	LAN struct {
		Address        string `json:"addr"`
		FlagDeprecated bool   `json:"flag_deprecated"`
		Intf           string `json:"intf"`
		Mask           string `json:"mask"`
		Type           string `json:"type"`
	}
)

func TZ() (string, error) {
	now := time.Now()
	return now.Format("-07:00"), nil
}

func GetLANS() (map[string][]LAN, error) {
	m := make(map[string][]LAN)
	intfs, err := net.Interfaces()
	if err != nil {
		return m, err
	}
	for _, intf := range intfs {
		addrs, err := intf.Addrs()
		if err != nil {
			continue
		}
		mcastAddrs, err := intf.MulticastAddrs()
		if err == nil {
			addrs = append(addrs, mcastAddrs...)
		}
		for _, addr := range addrs {
			e := LAN{}
			e.Intf = intf.Name
			l := strings.Split(addr.String(), "/")
			switch len(l) {
			case 1:
				// mcast
				e.Address = l[0]
			case 2:
				// ucast
				e.Address = l[0]
				e.Mask = l[1]
			default:
				continue
			}
			if strings.Contains(e.Address, ":") {
				e.Type = "ipv6"
			} else {
				e.Type = "ipv4"
			}
			hwAddr := intf.HardwareAddr.String()
			if _, ok := m[hwAddr]; !ok {
				m[hwAddr] = make([]LAN, 0)
			}
			m[hwAddr] = append(m[hwAddr], e)
		}
	}
	return m, nil
}

func ConnectTo() (string, error) {
	// TODO: port gcloud address detection ?
	return "", ErrIgnore
}

func Users() ([]User, error) {
	l := make([]User, 0)
	data, err := parseColumned("/etc/passwd")
	if err != nil {
		return l, err
	}
	for _, lineSlice := range data {
		if len(lineSlice) < 3 {
			continue
		}
		uid, err := strconv.Atoi(lineSlice[2])
		if err != nil {
			continue
		}
		l = append(l, User{
			Name: lineSlice[0],
			ID:   uid,
		})
	}
	return l, nil
}

func Groups() ([]Group, error) {
	l := make([]Group, 0)
	data, err := parseColumned("/etc/group")
	if err != nil {
		return l, err
	}
	for _, lineSlice := range data {
		if len(lineSlice) < 3 {
			continue
		}
		gid, err := strconv.Atoi(lineSlice[2])
		if err != nil {
			continue
		}
		l = append(l, Group{
			Name: lineSlice[0],
			ID:   gid,
		})
	}
	return l, nil
}

func parseColumned(p string) ([][]string, error) {
	l := make([][]string, 0)
	file, err := os.Open(p)
	if err != nil {
		return l, err
	}
	defer file.Close()
	reader := bufio.NewReader(file)
	for {
		line, err := reader.ReadString('\n')

		// skip all line starting with #
		if equal := strings.Index(line, "#"); equal < 0 {
			lineSlice := strings.FieldsFunc(line, func(divide rune) bool {
				return divide == ':'
			})
			l = append(l, lineSlice)
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return l, err
		}
	}
	return l, nil
}

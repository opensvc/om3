package disks

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/device"
	"github.com/opensvc/om3/v3/util/file"
)

type (
	// Dev is a block device.
	Dev struct {
		Name   string `json:"name"`
		WWN    string `json:"wwn"`
		Number string `json:"number"`
		Path   string `json:"path"`
		Size   uint64 `json:"size"`
		Vendor string `json:"vendor"`
		Model  string `json:"model"`
		Type   string `json:"type"`
	}

	// Devices is a deduplicating map of Dev, indexed by name.
	Devices map[string]Dev
)

var (
	backtick = "`"
	relRE    = regexp.MustCompile(`([\- |` + backtick + `]*)(\w.*)`)
	pairsRE  = regexp.MustCompile(`([A-Z:\-]+)=(?:"(.*?)")`)

	// caches
	_devices Devices
)

func load() error {
	if err := loadDevs(); err != nil {
		return err
	}
	if err := loadRelations(); err != nil {
		return err
	}
	return nil
}

func factor(t string) int {
	var factor int
	switch t {
	case "raid1":
		factor = 2
	default:
		factor = 1
	}
	return factor
}

func loadDevs() error {
	_devices = make(Devices)
	parse := func(line string) {
		d := Dev{}
		for _, pair := range pairsRE.FindAllStringSubmatch(line, -1) {
			key := pair[1]
			val := pair[2]
			switch key {
			case "PATH":
				d.Path = val
			case "NAME":
				d.Name = val
			case "WWN":
				d.WWN = strings.Replace(val, "0x", "", 1)
			case "SIZE":
				if i, err := strconv.ParseUint(val, 10, 64); err == nil {
					d.Size = i
				}
			case "VENDOR":
				d.Vendor = val
			case "MODEL":
				d.Model = val
			case "TYPE":
				d.Type = val
			case "MAJ:MIN":
				d.Number = val
			}
		}
		_devices[d.Name] = d
	}
	cmd := command.New(
		command.WithName("lsblk"),
		command.WithVarArgs("-o", "PATH,NAME,WWN,SIZE,VENDOR,MODEL,TYPE,MAJ:MIN", "-b", "-e7", "--pairs"),
		command.WithOnStdoutLine(parse),
	)
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func (d Dev) Parents() []Dev {
	devs := make([]Dev, 0)
	for parent, pm := range _relations {
		for child := range pm {
			if child != d.Name {
				continue
			}
			if dev, ok := _devices[parent]; ok {
				devs = append(devs, dev)
			}
		}
	}
	return devs
}

func (d Dev) Children() []Dev {
	devs := make([]Dev, 0)
	pm, ok := _relations[d.Name]
	if !ok {
		return devs
	}
	for child := range pm {
		if dev, ok := _devices[child]; ok {
			devs = append(devs, dev)
		}
	}
	return devs
}

func (d Dev) HasChildren() bool {
	return len(d.Children()) > 0
}

func (d Dev) HasParents() bool {
	return len(d.Parents()) > 0
}

func (d Dev) IsMpathPath() bool {
	for _, dev := range d.Children() {
		if dev.Type == "mpath" {
			return true
		}
	}
	return false
}

func (d Dev) IsMpath() bool {
	return d.Type == "mpath"
}

func GetDevices() (Devices, error) {
	if _devices != nil {
		return _devices, nil
	}
	if err := load(); err != nil {
		return nil, err
	}
	return _devices, nil
}

func getDeviceFromPath(s string) (Dev, error) {
	if v, err := file.ExistsAndSymlink(s); err != nil {
		return Dev{}, err
	} else if v {
		if rp, err := os.Readlink(s); err == nil {
			s = rp
		} else {
			return Dev{}, err
		}
	}
	number, err := device.New(s).MajorMinorStr()
	if err != nil {
		return Dev{}, err
	}
	devices, err := GetDevices()
	if err != nil {
		return Dev{}, err
	}
	for _, d := range devices {
		if d.Number == number {
			return d, nil
		}
	}
	return Dev{}, fmt.Errorf("path:%s number:%s not found in parsed devices", s, number)
}

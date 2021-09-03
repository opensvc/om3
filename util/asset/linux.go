// +build: linux

package asset

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jaypipes/pcidb"
	"github.com/pkg/errors"
	"opensvc.com/opensvc/util/file"
)

func (t T) Get(s string) (interface{}, error) {
	switch s {
	case "bios_version":
		return si.BIOS.Version, nil
	case "cpu_model":
		return si.CPU.Model, nil
	case "cpu_freq":
		return si.CPU.Speed, nil
	case "cpu_threads":
		return si.CPU.Threads, nil
	case "cpu_cores":
		return si.CPU.Cores, nil
	case "cpu_dies":
		return si.CPU.Cpus, nil
	case "os_vendor":
		return si.OS.Vendor, nil
	case "os_release":
		return si.OS.Release, nil
	case "os_kernel":
		return si.Kernel.Release, nil
	case "os_arch":
		return si.OS.Architecture, nil
	case "os_name":
		return si.OS.Name, nil
	case "serial":
		return si.Product.Serial, nil
	case "sp_version":
		return "", ErrNotImpl
	case "enclosure":
		return "", ErrNotImpl
	case "tz":
		return TZ()
	case "manufacturer":
		return si.Product.Vendor, nil
	case "model":
		return si.Product.Name, nil
	case "mem_banks":
		return 0, ErrNotImpl
	case "mem_slots":
		return 0, ErrNotImpl
	case "mem_bytes":
		return si.Memory.Size, nil
	case "fqdn":
		return os.Hostname()
	case "last_boot":
		return LastBoot()
	case "boot_id":
		return BootID()
	case "connect_to":
		return ConnectTo()
	default:
		return nil, fmt.Errorf("unknown asset key: %s", s)
	}
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

func getPCIDevice(address string, db *pcidb.PCIDB) *Device {
	var (
		b                                                []byte
		err                                              error
		s, revision, deviceID, vendorID, product, vendor string
	)
	p := "/sys/bus/pci/devices/" + address
	dev := &Device{Type: "pci"}

	// path
	if i := strings.Index(address, ":"); i >= 0 && len(address) > i+1 {
		dev.Path = address[i+1:]
	}

	// revision
	b, err = file.ReadAll(p + "/revision")
	if err == nil {
		s := string(b)
		s = strings.TrimRight(s, "\n\r")
		s = strings.Replace(s, "0x", "", 1)
		revision = s
	}

	// driver
	s, err = os.Readlink(p + "/driver")
	if err == nil {
		s = filepath.Base(s)
		dev.Driver = s
	}

	// vendor
	b, err = file.ReadAll(p + "/vendor")
	if err == nil {
		s := string(b)
		s = strings.TrimRight(s, "\n\r")
		s = strings.Replace(s, "0x", "", 1)
		vendorID = s
		if v, ok := db.Vendors[s]; ok {
			vendor = v.Name
		}
	}

	// product
	b, err = file.ReadAll(p + "/device")
	if err == nil {
		s := string(b)
		s = strings.TrimRight(s, "\n\r")
		s = strings.Replace(s, "0x", "", 1)
		deviceID = s
		if v, ok := db.Products[vendorID+deviceID]; ok {
			product = v.Name
		}
	}

	// class
	b, err = file.ReadAll(p + "/class")
	if err == nil {
		s := string(b)
		s = s[2:4]
		if v, ok := db.Classes[s]; ok {
			dev.Class = v.Name
		}
	}

	dev.Description = fmt.Sprintf("%s %s (rev %s)", vendor, product, revision)
	return dev
}

func Hardware() ([]Device, error) {
	all := make([]Device, 0)

	devs, _ := hardwarePCIDevices()
	all = append(all, devs...)

	mems, _ := hardwareMemDevices()
	all = append(all, mems...)

	return all, nil
}

func hardwareMemDevices() ([]Device, error) {
	devs := make([]Device, 0)
	return devs, nil
}

func hardwarePCIDevices() ([]Device, error) {
	devs := make([]Device, 0)
	db, err := pcidb.New(pcidb.WithDisableNetworkFetch())
	if err != nil {
		return devs, err
	}
	links, err := ioutil.ReadDir("/sys/bus/pci/devices")
	if err != nil {
		return devs, err
	}
	for _, link := range links {
		addr := link.Name()
		dev := getPCIDevice(addr, db)
		if dev == nil {
			continue
		}
		devs = append(devs, *dev)
	}
	return devs, nil
}

func LastBoot() (string, error) {
	p := "/proc/uptime"
	b, err := file.ReadAll(p)
	if err != nil {
		return "", errors.Wrapf(err, "unable to get last boot time from /proc/uptime")
	}
	l := strings.Fields(string(b))
	secs, err := strconv.ParseFloat(l[0], 64)
	if err != nil {
		return "", errors.Wrapf(err, "unable to get last boot time from /proc/uptime")
	}
	now := time.Now()
	last := now.Add(time.Duration(-int(secs * float64(time.Second))))
	return last.Format(time.RFC3339), nil
}

func BootID() (string, error) {
	p := "/proc/sys/kernel/random/boot_id"
	b, err := file.ReadAll(p)
	if err == nil {
		s := string(b)
		s = strings.TrimRight(s, "\n\r")
		return s, nil
	}
	p = "/proc/stat"
	file, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer file.Close()
	s := bufio.NewScanner(file)
	for s.Scan() {
		lineFields := strings.Fields(s.Text())
		if len(lineFields) == 2 && lineFields[0] == "btime" {
			return lineFields[1], nil
		}
	}
	return "", fmt.Errorf("unable to format a boot id from /proc/sys/kernel/random/boot_id nor /proc/stat")
}

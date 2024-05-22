//go:build linux

package asset

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	dosmbios "github.com/digitalocean/go-smbios/smbios"
	"github.com/jaypipes/pcidb"
	"github.com/talos-systems/go-smbios/smbios"
	"github.com/zcalusic/sysinfo"

	"github.com/opensvc/om3/util/bootid"
)

var (
	si          sysinfo.SysInfo
	smb         *smbios.SMBIOS
	initialized bool
)

func New() *T {
	t := T{}
	if !initialized {
		si.GetSysInfo()
	}
	return &t
}

func SMBIOS() (*smbios.SMBIOS, error) {
	if smb != nil {
		return smb, nil
	}
	smb, err := smbios.New()
	return smb, err
}

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
		return osName()
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
		return memBanks()
	case "mem_slots":
		return memSlots()
	case "mem_bytes":
		return si.Memory.Size, nil
	case "fqdn":
		return os.Hostname()
	case "last_boot":
		return LastBoot()
	case "boot_id":
		return bootid.Scan()
	case "connect_to":
		return ConnectTo()
	default:
		return nil, fmt.Errorf("unknown asset key: %s", s)
	}
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
	b, err = os.ReadFile(p + "/revision")
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
	b, err = os.ReadFile(p + "/vendor")
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
	b, err = os.ReadFile(p + "/device")
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
	b, err = os.ReadFile(p + "/class")
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

	devs, err := hardwarePCIDevices()
	if err != nil {
		return all, err
	}
	all = append(all, devs...)

	mems, _ := hardwareMemDevices()
	if err != nil {
		return all, err
	}
	all = append(all, mems...)

	return all, nil
}

func memSlots() (int, error) {
	smb, err := SMBIOS()
	if err != nil {
		return 0, fmt.Errorf("parse smbios: %w", err)
	}
	n := 0
	for _, s := range smb.Structures {
		if s.Header.Type != 17 {
			continue
		}
		n++
	}
	return n, nil
}

func memBanks() (int, error) {
	smb, err := SMBIOS()
	if err != nil {
		return 0, fmt.Errorf("parse smbios: %w", err)
	}
	n := 0
	for _, s := range smb.Structures {
		if s.Header.Type != 17 {
			continue
		}
		if fmtSize(s) == "" {
			continue
		}
		n++
	}
	return n, nil
}

func osName() (string, error) {
	return runtime.GOOS, nil
}

// pkg Size() is buggy wrt to extended support ...
// define a size formatter here.
func fmtSize(s *dosmbios.Structure) string {
	size := int(binary.LittleEndian.Uint16(s.Formatted[8:10]))
	if size == 0 {
		return ""
	}

	// An extended uint32 DIMM size field appears if 0x7fff is present in size.
	if size == 0x7fff {
		size = int(binary.LittleEndian.Uint32(s.Formatted[24:28]))
	}

	// Size units depend on MSB.  Little endian MSB for uint16 is in second byte.
	// 0 means megabytes, 1 means kilobytes.
	unit := "KB"
	if s.Formatted[9]&0x80 == 0 {
		unit = "MB"
	}
	return fmt.Sprintf("%d %s", size, unit)
}

func hardwareMemDevices() ([]Device, error) {
	devs := make([]Device, 0)
	smb, err := SMBIOS()
	if err != nil {
		return devs, fmt.Errorf("parse smbios: %w", err)
	}

	for _, s := range smb.Structures {
		if s.Header.Type != 17 {
			continue
		}
		mdev := smbios.MemoryDeviceStructure{Structure: s}
		path := fmt.Sprintf("%s %s", mdev.Locator(), mdev.BankLocator())
		clas := fmt.Sprintf("%s %s %s %s", fmtSize(s), mdev.MemoryType(), mdev.TypeDetail(), mdev.Speed())
		desc := fmt.Sprintf("%s %s", mdev.Manufacturer(), mdev.PartNumber())
		devs = append(devs, Device{
			Path:        path,
			Description: desc,
			Class:       clas,
			Type:        "mem",
		})
	}
	return devs, nil
}

func hardwarePCIDevices() ([]Device, error) {
	devs := make([]Device, 0)
	db, err := pcidb.New(pcidb.WithDisableNetworkFetch())
	if err != nil {
		return devs, err
	}
	links, err := os.ReadDir("/sys/bus/pci/devices")
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
	b, err := os.ReadFile(p)
	if err != nil {
		return "", fmt.Errorf("unable to get last boot time from /proc/uptime: %w", err)
	}
	l := strings.Fields(string(b))
	secs, err := strconv.ParseFloat(l[0], 64)
	if err != nil {
		return "", fmt.Errorf("unable to get last boot time from /proc/uptime: %w", err)
	}
	now := time.Now()
	last := now.Add(time.Duration(-int(secs * float64(time.Second))))
	return last.Format(time.RFC3339), nil
}

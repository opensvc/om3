//go:build linux

package scsi

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/opensvc/fcntllock"
	"github.com/opensvc/flock"

	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/util/capabilities"
	"github.com/opensvc/om3/v3/util/xsession"
)

func (t *PersistentReservationHandle) setup() error {
	if t.persistentReservationDriver != nil {
		return nil
	}
	if capabilities.Has(MpathPersistCapability) {
		t.persistentReservationDriver = MpathPersistDriver{
			Log: t.Log,
		}
	} else if capabilities.Has(SGPersistCapability) {
		t.persistentReservationDriver = SGPersistDriver{
			Log: t.Log,
		}
	} else {
		return ErrNotSupported
	}
	return nil
}

func doWithLock(timeout time.Duration, name, intent string, f func() error) error {
	p := filepath.Join(rawconfig.Paths.Lock, strings.Join([]string{"scsi", name}, "."))
	lock := flock.New(p, xsession.ID.String(), fcntllock.New)
	err := lock.Lock(timeout, intent)
	if err != nil {
		return err
	}
	defer func() { _ = lock.UnLock() }()
	return f()
}

func ListHostDirs() ([]string, error) {
	dirs, err := filepath.Glob("/sys/class/scsi_host/host*")
	if err != nil {
		return []string{}, err
	}
	return dirs, nil
}

func LockedScanAll(timeout time.Duration) error {
	return doWithLock(timeout, "scan", "all", func() error {
		return ScanAll()
	})
}

func ScanAll() error {
	return ScanAllBusTargetLun("-", "-", "-")
}

func ScanAllBusTargetLun(b, t, l string) error {
	hosts, err := ListHostDirs()
	if err != nil {
		return err
	}
	for _, h := range hosts {
		if e := ScanHostDirBusTargetLun(h, b, t, l); err != nil {
			err = errors.Join(err, e)
		}
	}
	return err
}

func ScanHostDirBusTargetLun(h, b, t, l string) error {
	if t == "" {
		t = "-"
	}
	if b == "" {
		b = "-"
	}
	if l == "" {
		l = "-"
	}
	filename := fmt.Sprintf("%s/scan", h)
	s := fmt.Sprintf("%s %s %s", b, t, l)
	return os.WriteFile(filename, []byte(s), os.ModePerm)
}

func ScanHostBusTargetLun(h, b, t, l string) error {
	return ScanHostDirBusTargetLun("/sys/class/scsi_host/host"+h, b, t, l)
}

// hbaNums resolves an HBA identifier (WWN or iSCSI initiator name) to host numbers
// Returns a slice of host numbers since an iSCSI initiator can be associated with multiple hosts
func hbaNums(hba string) ([]string, error) {
	if hba == "" {
		return nil, nil
	}

	var hostNums []string

	// Check if it's an iSCSI initiator name
	if strings.HasPrefix(hba, "iqn") {
		matches, err := filepath.Glob("/sys/class/scsi_host/host*/device/session*/iscsi_session/session*/initiatorname")
		if err != nil {
			return nil, err
		}

		for _, path := range matches {
			content, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			if strings.TrimSpace(string(content)) == hba {
				hostPath := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(path)))))
				hostNum := strings.TrimPrefix(filepath.Base(hostPath), "host")
				// Avoid duplicates
				if !slices.Contains(hostNums, hostNum) {
					hostNums = append(hostNums, hostNum)
				}
			}
		}
	}

	// Check if it's a FC port name
	matches, err := filepath.Glob("/sys/class/fc_host/host*/port_name")
	if err != nil {
		return nil, err
	}

	for _, path := range matches {
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		portName := strings.TrimSpace(string(content))
		if portName == hba || "0x"+portName == hba {
			hostPath := filepath.Dir(path)
			hostNum := strings.TrimPrefix(filepath.Base(hostPath), "host")
			// Avoid duplicates
			if !slices.Contains(hostNums, hostNum) {
				hostNums = append(hostNums, hostNum)
			}
		}
	}

	if len(hostNums) == 0 {
		return nil, fmt.Errorf("HBA %s not found", hba)
	}

	return hostNums, nil
}

// targetNum resolves a target identifier (WWN or iSCSI target name) to a target number
func targetNum(hostNum, target string) (string, error) {
	if target == "" {
		return "", nil
	}

	// Check if it's an iSCSI target name
	if strings.HasPrefix(target, "iqn") {
		pattern := "/sys/class/scsi_host/host" + hostNum + "/device/session*/iscsi_session/session*/targetname"
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return "", err
		}

		for _, path := range matches {
			content, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			if strings.TrimSpace(string(content)) == target {
				sessionPath := filepath.Dir(filepath.Dir(filepath.Dir(path)))
				targetPaths, err := filepath.Glob(sessionPath + "/target*:*:*")
				if err != nil || len(targetPaths) == 0 {
					continue
				}
				base := filepath.Base(targetPaths[0])
				return base[strings.LastIndex(base, ":")+1:], nil
			}
		}
	}

	// Check if it's a FC target port name
	pattern := "/sys/class/fc_transport/target" + hostNum + ":*:*"
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", err
	}

	for _, path := range matches {
		portNamePath := filepath.Join(path, "port_name")
		content, err := os.ReadFile(portNamePath)
		if err != nil {
			continue
		}
		portName := strings.TrimSpace(string(content))
		if portName == target || "0x"+portName == target {
			return filepath.Base(path)[strings.LastIndex(path, ":")+1:], nil
		}
	}

	return "", fmt.Errorf("target %s not found on host %s", target, hostNum)
}

// ScanSCSIHosts scans SCSI hosts for new devices
// hba and target are port names in the SCSI transport (e.g., WWN or iSCSI names)
// If hba and target are empty, all hosts will be scanned
// If only hba is provided, all targets on that host(s) will be scanned
// If hba and target are provided, specific target will be scanned on each host
// lun specifies the logical unit number to scan
func ScanSCSIHosts(hba, target, lun string) error {
	if hba == "" && target == "" {
		// Scan all hosts if no specific HBA or target is provided
		return ScanAll()
	}

	// Resolve HBA to host numbers (can return multiple for iSCSI)
	hostNums, err := hbaNums(hba)
	if err != nil {
		return fmt.Errorf("failed to resolve HBA: %w", err)
	}

	if len(hostNums) == 0 {
		// No HBA specified, scan all hosts
		return ScanAll()
	}

	// Loop over all host numbers (important for iSCSI with multiple hosts)
	var scanErrors []error
	for _, hostNum := range hostNums {
		targetNum, err := targetNum(hostNum, target)
		if err != nil {
			continue
		}
		if targetNum == "" {
			// No target specified, scan all targets on the host
			if err := ScanAllBusTargetLun(hostNum, "-", "-"); err != nil {
				scanErrors = append(scanErrors, fmt.Errorf("host %s: %w", hostNum, err))
			}
		} else {
			// Scan specific host:bus:target:lun
			// Note: We need to determine the bus number, for now we use "-" to scan all buses
			if err := ScanHostBusTargetLun(hostNum, "-", targetNum, lun); err != nil {
				scanErrors = append(scanErrors, fmt.Errorf("host %s: %w", hostNum, err))
			}
		}
	}

	if len(scanErrors) > 0 {
		return fmt.Errorf("scan completed with errors: %v", scanErrors)
	}

	return nil
}

//go:build !linux

package scsi

import (
	"fmt"
)

func ScanSCSIHosts(hba, target, lun string) error {
	return fmt.Errorf("dummy implementation: %s %s %s", hba, target, lun)
}

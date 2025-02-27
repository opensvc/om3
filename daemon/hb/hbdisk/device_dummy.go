//go:build !linux

package hbdisk

// openDevice opens the specified char device, validates it, and
// initializes file handling for raw mode operations.
func (t *device) openDevice(newDev string) error {
	if err := t.ensureCharDevice(newDev); err != nil {
		return err
	}
	t.mode = modeRaw
	if t.file, err = os.OpenFile(t.path, os.O_RDWR, openDevicePermission); err != nil {
		return fmt.Errorf("%s open char device: %w", t.path, err)
	}
}

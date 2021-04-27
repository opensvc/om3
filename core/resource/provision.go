package resource

import (
	"path/filepath"
	"time"

	"opensvc.com/opensvc/core/provisioned"
	"opensvc.com/opensvc/util/file"
	"opensvc.com/opensvc/util/timestamp"
)

// VarDir is the full path of the directory where the resource can write its private variable data.
func (t T) VarDir() string {
	return filepath.Join(t.object.VarDir(), t.RID())
}

// provisionedFile is the full path to the provisioned state cache file.
func provisionedFile(t Driver) string {
	return filepath.Join(t.VarDir(), "provisioned")
}

// provisionedFileModTime returns the provisioned state cache file modification time.
func provisionedFileModTime(t Driver) time.Time {
	return file.ModTime(provisionedFile(t))
}

// provisionedTimestamp returns the provisioned state cache file modification time.
func provisionedTimestamp(t Driver) timestamp.T {
	return timestamp.New(provisionedFileModTime(t))
}

//
// getProvisionStatus returns the resource provisioned state from the on-disk cache and its
// state change time.
//
func getProvisionStatus(t Driver) ProvisionStatus {
	var (
		data ProvisionStatus
	)
	if state, err := Provisioned(t); err != nil {
		t.StatusLog().Error("provisioned: %s", err)
	} else {
		data.State = state
	}
	data.Mtime = provisionedTimestamp(t)
	return data
}

func Provision(t Driver) error {
	return t.Provision()
}

func Unprovision(t Driver) error {
	return t.Unprovision()
}

func Provisioned(t Driver) (provisioned.T, error) {
	return t.Provisioned()
}

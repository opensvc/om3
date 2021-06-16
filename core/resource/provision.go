package resource

import (
	"path/filepath"
	"time"

	"opensvc.com/opensvc/core/provisioned"
	"opensvc.com/opensvc/util/file"
	"opensvc.com/opensvc/util/timestamp"
)

type (
	UnprovisionLeaderer interface {
		UnprovisionLeader() error
	}
	ProvisionLeaderer interface {
		ProvisionLeader() error
	}
	UnprovisionLeadeder interface {
		UnprovisionLeaded() error
	}
	ProvisionLeadeder interface {
		ProvisionLeaded() error
	}
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

func Provision(t Driver, leader bool) error {
	if !t.IsStandby() && !leader && t.IsShared() {
		return ProvisionLeaded(t)
	}
	return ProvisionLeader(t)
}

func ProvisionLeader(t Driver) error {
	if i, ok := t.(ProvisionLeaderer); ok {
		return i.ProvisionLeader()
	}
	return nil
}

func ProvisionLeaded(t Driver) error {
	if i, ok := t.(ProvisionLeadeder); ok {
		return i.ProvisionLeaded()
	}
	return nil
}

func Unprovision(t Driver, leader bool) error {
	if err := t.Stop(); err != nil {
		return err
	}
	if leader || t.IsStandby() {
		return UnprovisionLeader(t)
	} else {
		return UnprovisionLeaded(t)
	}
}

func UnprovisionLeader(t Driver) error {
	if i, ok := t.(UnprovisionLeaderer); ok {
		return i.UnprovisionLeader()
	}
	return nil
}

func UnprovisionLeaded(t Driver) error {
	if i, ok := t.(UnprovisionLeadeder); ok {
		return i.UnprovisionLeaded()
	}
	return nil
}

func Provisioned(t Driver) (provisioned.T, error) {
	return t.Provisioned()
}

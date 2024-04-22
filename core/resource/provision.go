package resource

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/opensvc/om3/core/provisioned"
	"github.com/opensvc/om3/core/trigger"
	"github.com/opensvc/om3/util/file"
)

type (
	UnprovisionLeaderer interface {
		UnprovisionLeader(context.Context) error
	}
	ProvisionLeaderer interface {
		ProvisionLeader(context.Context) error
	}
	UnprovisionLeadeder interface {
		UnprovisionLeaded(context.Context) error
	}
	ProvisionLeadeder interface {
		ProvisionLeaded(context.Context) error
	}
	ProvisionStarter interface {
		ProvisionStart(context.Context) error
	}
	UnprovisionStoper interface {
		UnprovisionStop(context.Context) error
	}
)

// VarDir is the full path of the directory where the resource can write its private variable data.
func (t T) VarDir() string {
	return filepath.Join(t.object.(ObjectDriver).VarDir(), t.RID())
}

// provisionedFile is the full path to the provisioned state cache file.
func provisionedFile(t Driver) string {
	return filepath.Join(t.VarDir(), "provisioned")
}

// provisionedFileModTime returns the provisioned state cache file modification time.
func provisionedFileModTime(t Driver) time.Time {
	return file.ModTime(provisionedFile(t))
}

// getProvisionStatus returns the resource provisioned state from the on-disk cache and its
// state change time.
func getProvisionStatus(t Driver) ProvisionStatus {
	var (
		data ProvisionStatus
	)
	if state, err := Provisioned(t); err != nil {
		t.StatusLog().Error("provisioned: %s", err)
	} else {
		data.State = state
	}
	data.Mtime = provisionedFileModTime(t)
	return data
}

// Provision handles triggers around provision() and resource dependencies
func Provision(ctx context.Context, r Driver, leader bool) error {
	defer EvalStatus(ctx, r)
	if r.IsDisabled() {
		return nil
	}
	Setenv(r)
	if r.IsProvisionDisabled() {
		r.Log().Infof("provision is disabled")
		return nil
	}
	if err := checkRequires(ctx, r); err != nil {
		return fmt.Errorf("provision requires: %w", err)
	}
	if err := r.Trigger(ctx, trigger.Block, trigger.Pre, trigger.Provision); err != nil {
		return fmt.Errorf("pre provision trigger: %w", err)
	}
	if err := r.Trigger(ctx, trigger.NoBlock, trigger.Pre, trigger.Provision); err != nil {
		r.Log().Warnf("trigger: %s (exitcode %d)", err, exitCode(err))
	}
	r.Progress(ctx, "▶ provision")
	if err := provision(ctx, r, leader); err != nil {
		return fmt.Errorf("provision: %w", err)
	}
	if err := SCSIPersistentReservationStart(ctx, r); err != nil {
		return err
	}
	if err := r.Trigger(ctx, trigger.Block, trigger.Post, trigger.Provision); err != nil {
		return fmt.Errorf("post provision trigger: %w", err)
	}
	if err := r.Trigger(ctx, trigger.NoBlock, trigger.Post, trigger.Provision); err != nil {
		r.Log().Warnf("trigger: %s (exitcode %d)", err, exitCode(err))
	}
	return nil
}

// Unprovision handles triggers around unprovision() and resource dependencies
func Unprovision(ctx context.Context, r Driver, leader bool) error {
	defer EvalStatus(ctx, r)
	if r.IsDisabled() {
		return nil
	}
	Setenv(r)
	if r.IsUnprovisionDisabled() {
		if err := unprovisionStop(ctx, r); err != nil {
			return err
		}
		r.Log().Infof("unprovision is disabled")
		return nil
	}
	if err := checkRequires(ctx, r); err != nil {
		return fmt.Errorf("unprovision requires: %w", err)
	}
	if err := r.Trigger(ctx, trigger.Block, trigger.Pre, trigger.Unprovision); err != nil {
		return fmt.Errorf("pre unprovision trigger: %w", err)
	}
	if err := r.Trigger(ctx, trigger.NoBlock, trigger.Pre, trigger.Unprovision); err != nil {
		r.Log().Warnf("trigger: %s (exitcode %d)", err, exitCode(err))
	}
	if err := SCSIPersistentReservationStop(ctx, r); err != nil {
		return err
	}
	r.Progress(ctx, "▶ unprovision")
	if err := unprovision(ctx, r, leader); err != nil {
		return fmt.Errorf("unprovision: %w", err)
	}
	if err := r.Trigger(ctx, trigger.Block, trigger.Post, trigger.Unprovision); err != nil {
		return fmt.Errorf("post unprovision trigger: %w", err)
	}
	if err := r.Trigger(ctx, trigger.NoBlock, trigger.Post, trigger.Unprovision); err != nil {
		r.Log().Warnf("trigger: %s (exitcode %d)", err, exitCode(err))
	}
	return nil
}

func provision(ctx context.Context, t Driver, leader bool) error {
	if err := provisionLeaderOrLeaded(ctx, t, leader); err != nil {
		return err
	}
	if err := setProvisionedValue(true, t); err != nil {
		return err
	}
	if err := startLeader(ctx, t, leader); err != nil {
		return err
	}
	return nil
}

func isLeaded(t Driver, leader bool) bool {
	return !t.IsStandby() && !leader && t.IsShared()
}

func startLeader(ctx context.Context, t Driver, leader bool) error {
	if isLeaded(t, leader) {
		return nil
	}
	switch o := t.(type) {
	case ProvisionStarter:
		return o.ProvisionStart(ctx)
	case starter:
		return o.Start(ctx)
	default:
		return nil
	}
}

func provisionLeaderOrLeaded(ctx context.Context, t Driver, leader bool) error {
	if leader {
		return provisionLeader(ctx, t)
	} else {
		return provisionLeaded(ctx, t)
	}
}

func provisionLeader(ctx context.Context, t Driver) error {
	if i, ok := t.(ProvisionLeaderer); ok {
		return i.ProvisionLeader(ctx)
	}
	return nil
}

func provisionLeaded(ctx context.Context, t Driver) error {
	if i, ok := t.(ProvisionLeadeder); ok {
		// The driver cared to implement a ProvisionLeaded function,
		// let it decide what to do with standby resources.
		return i.ProvisionLeaded(ctx)
	} else if !t.IsShared() {
		// The driver did not declare a special behaviour on leaded.
		// Non-shared resources must be provisioned too, use the leader method.
		return provisionLeader(ctx, t)
	}
	return nil
}

func unprovision(ctx context.Context, t Driver, leader bool) error {
	if err := unprovisionStop(ctx, t); err != nil {
		return err
	}
	if err := unprovisionLeaderOrLeaded(ctx, t, leader); err != nil {
		return err
	}
	if err := setProvisionedValue(false, t); err != nil {
		return err
	}
	return nil
}

func unprovisionStop(ctx context.Context, t Driver) error {
	switch o := t.(type) {
	case UnprovisionStoper:
		return o.UnprovisionStop(ctx)
	case stopper:
		return o.Stop(ctx)
	default:
		return nil
	}
}

func unprovisionLeaderOrLeaded(ctx context.Context, t Driver, leader bool) error {
	if leader {
		return unprovisionLeader(ctx, t)
	} else {
		return unprovisionLeaded(ctx, t)
	}
}

func unprovisionLeader(ctx context.Context, t Driver) error {
	if i, ok := t.(UnprovisionLeaderer); ok {
		return i.UnprovisionLeader(ctx)
	}
	return nil
}

func unprovisionLeaded(ctx context.Context, t Driver) error {
	if i, ok := t.(UnprovisionLeadeder); ok {
		return i.UnprovisionLeaded(ctx)
	} else if t.IsStandby() && !t.IsShared() {
		return unprovisionLeader(ctx, t)
	}
	return nil
}

func Provisioned(t Driver) (provisioned.T, error) {
	if t.IsDisabled() {
		return provisioned.NotApplicable, nil
	}
	if !hasAnyProvInterface(t) {
		return provisioned.NotApplicable, nil
	}
	if v, err := getProvisionedValue(t); err == nil {
		return provisioned.FromBool(v), nil
	}
	if v, err := t.Provisioned(); err == nil {
		provBool := v.IsOneOf(provisioned.True)
		err = setProvisionedValue(provBool, t)
		return provisioned.FromBool(provBool), err
	} else {
		return v, nil
	}
}

func hasAnyProvInterface(r Driver) bool {
	if _, ok := r.(ProvisionLeaderer); ok {
		return true
	}
	if _, ok := r.(ProvisionLeadeder); ok {
		return true
	}
	if _, ok := r.(UnprovisionLeaderer); ok {
		return true
	}
	if _, ok := r.(UnprovisionLeadeder); ok {
		return true
	}
	return false
}

func getProvisionedValue(r Driver) (bool, error) {
	var v bool
	p := provisionedFile(r)
	f, err := os.Open(p)
	if err != nil {
		return false, err
	}
	defer f.Close()
	enc := json.NewDecoder(f)
	if err := enc.Decode(&v); err != nil {
		return false, err
	}
	return v, nil
}

func setProvisionedValue(v bool, r Driver) error {
	p := provisionedFile(r)
	d := filepath.Dir(p)
	if _, err := os.Stat(d); errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(d, os.ModePerm); err != nil {
			return err
		}
	}
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	if err := enc.Encode(v); err != nil {
		return err
	}
	return nil
}

// SetProvisioned creates a flag file in the resource var dir to remember that the provision is done.
func SetProvisioned(ctx context.Context, r Driver) error {
	if err := setProvisionedValue(true, r); err != nil {
		return err
	}
	r.Log().Infof("set provisioned")
	return nil
}

// SetUnprovisioned removes the flag file in the resource var dir to forget that the provision is done.
func SetUnprovisioned(ctx context.Context, r Driver) error {
	if err := setProvisionedValue(false, r); err != nil {
		return err
	}
	r.Log().Infof("set unprovisioned")
	return nil
}

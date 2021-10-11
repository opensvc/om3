package object

import (
	"time"

	"opensvc.com/opensvc/core/resourceselector"
)

type (
	// OptsGlobal contains options accepted by all actions
	OptsGlobal struct {
		Color          string `flag:"color"`
		Format         string `flag:"format"`
		Server         string `flag:"server"`
		Local          bool   `flag:"local"`
		NodeSelector   string `flag:"node"`
		ObjectSelector string `flag:"object"`
		DryRun         bool   `flag:"dry-run"`
	}

	OptsResourceSelector struct {
		resourceselector.Options
	}

	// OptsLocking contains options accepted by all actions using an action lock
	OptsLocking struct {
		Disable bool          `flag:"nolock"`
		Timeout time.Duration `flag:"waitlock"`
	}

	// OptsAsync contains options accepted by all actions having an orchestration
	OptsAsync struct {
		Watch bool          `flag:"watch"`
		Wait  bool          `flag:"wait"`
		Time  time.Duration `flag:"time"`
	}

	// OptDisableRollback contains the disable-rollback option
	OptDisableRollback struct {
		DisableRollback bool `flag:"disable-rollback"`
	}

	// OptForce contains the force option
	OptForce struct {
		Force bool `flag:"force"`
	}

	// OptConfirm contains the confirm option
	OptConfirm struct {
		Confirm bool `flag:"confirm"`
	}

	// OpTo sets a barrier when iterating over a resource lister
	OptTo struct {
		To     string `flag:"to"`
		UpTo   string `flag:"upto"`   // Deprecated
		DownTo string `flag:"downto"` // Deprecated
	}

	//
	// OptLeader is used by the provision and unprovision action to trigger
	// allocation of shared resources on the leader node only.
	//
	// This option is usually set by the daemon, who is responsible for the
	// leader detection.
	//
	OptLeader struct {
		Leader bool `flag:"leader"`
	}

	OptsCreate struct {
		Global OptsGlobal
		OptsAsync
		OptsLocking
		OptsResourceSelector
		OptTo
		OptForce
		Template    string   `flag:"template"`
		Config      string   `flag:"config"`
		Keywords    []string `flag:"kwops"`
		Env         string   `flag:"env"`
		Interactive bool     `flag:"interactive"`
		Provision   bool     `flag:"provision"`
		Restore     bool     `flag:"restore"`
		Namespace   string   `flag:"createnamespace"`
	}
)

func (t OptDisableRollback) IsRollbackDisabled() bool {
	return t.DisableRollback
}
func (t OptConfirm) IsConfirm() bool {
	return t.Confirm
}
func (t OptForce) IsForce() bool {
	return t.Force
}
func (t OptTo) ToStr() string {
	return t.To
}
func (t OptLeader) IsLeader() bool {
	return t.Leader
}
func (t OptsGlobal) IsDryRun() bool {
	return t.DryRun
}

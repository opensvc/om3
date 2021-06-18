package object

import "time"

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

	// OptsResourceSelector contains options accepted by all actions manipulating resources
	OptsResourceSelector struct {
		ID     string `flag:"rid"`
		Subset string `flag:"subsets"`
		Tag    string `flag:"tags"`
		To     string `flag:"to"`
		UpTo   string `flag:"upto"`   // Deprecated
		DownTo string `flag:"downto"` // Deprecated
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
func (t OptsGlobal) IsDryRun() bool {
	return t.DryRun
}
func (t OptsResourceSelector) GetResourceSelector() OptsResourceSelector {
	return t
}

func (t OptsResourceSelector) IsZero() bool {
	switch {
	case t.ID != "":
		return true
	case t.Subset != "":
		return true
	case t.Tag != "":
		return true
	case t.To != "":
		return true
	case t.UpTo != "":
		return true
	case t.DownTo != "":
		return true
	default:
		return false
	}
}

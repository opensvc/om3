package objectaction

import (
	"opensvc.com/opensvc/core/kind"
	"opensvc.com/opensvc/core/ordering"
)

type (
	T struct {
		Name        string
		Target      string
		Progress    string
		Order       ordering.T
		LocalExpect string
		Local       bool
		Freeze      bool
		Kinds       []kind.T
	}
)

var (
	Abort = T{
		Name:        "abort",
		Target:      "aborted",
		Progress:    "aborting",
		LocalExpect: "unset",
	}
	Delete = T{
		Name:     "delete",
		Target:   "deleted",
		Progress: "deleting",
		Order:    ordering.Desc,
		Local:    true,
		Kinds:    []kind.T{kind.Svc, kind.Vol, kind.Usr, kind.Sec, kind.Cfg},
	}
	Freeze = T{
		Name:        "freeze",
		Target:      "frozen",
		Progress:    "freezing",
		Local:       true,
		LocalExpect: "unset",
		Kinds:       []kind.T{kind.Svc, kind.Vol},
	}
	Giveback = T{
		Name:        "giveback",
		Target:      "placed",
		Progress:    "placing",
		LocalExpect: "unset",
		Kinds:       []kind.T{kind.Svc},
	}
	Move = T{
		Name:        "move",
		Target:      "placed@",
		Progress:    "placing@",
		LocalExpect: "unset",
		Kinds:       []kind.T{kind.Svc},
	}
	Provision = T{
		Name:        "provision",
		Target:      "provisioned",
		Progress:    "provisioning",
		Local:       true,
		LocalExpect: "unset",
		Kinds:       []kind.T{kind.Svc, kind.Vol},
	}
	Purge = T{
		Name:     "purge",
		Target:   "purged",
		Progress: "purging",
		Order:    ordering.Desc,
		Local:    true,
		Kinds:    []kind.T{kind.Svc, kind.Vol, kind.Usr, kind.Sec, kind.Cfg},
	}
	Restart = T{
		Name:        "restart",
		Target:      "restarted",
		Progress:    "restarting",
		Local:       true,
		LocalExpect: "unset",
		Kinds:       []kind.T{kind.Svc, kind.Vol},
	}
	Shutdown = T{
		Name:     "shutdown",
		Target:   "shutdown",
		Progress: "shutting",
		Local:    true,
		Order:    ordering.Desc,
		Kinds:    []kind.T{kind.Svc, kind.Vol},
	}
	Start = T{
		Name:        "start",
		Target:      "started",
		Progress:    "starting",
		Local:       true,
		LocalExpect: "unset",
		Kinds:       []kind.T{kind.Svc, kind.Vol},
	}
	Stop = T{
		Name:        "stop",
		Target:      "stopped",
		Progress:    "stopping",
		Local:       true,
		Order:       ordering.Desc,
		LocalExpect: "",
		Kinds:       []kind.T{kind.Svc, kind.Vol},
		Freeze:      true,
	}
	Rollback = T{
		Name:        "rollback",
		Progress:    "rollbacking",
		LocalExpect: "",
		Kinds:       []kind.T{kind.Svc, kind.Vol},
	}
	Switch = T{
		Name:        "switch",
		Target:      "placed@",
		Progress:    "placing@",
		LocalExpect: "unset",
		Kinds:       []kind.T{kind.Svc},
	}
	Takeover = T{
		Name:        "takeover",
		Target:      "placed@",
		Progress:    "placing@",
		LocalExpect: "unset",
		Kinds:       []kind.T{kind.Svc},
	}
	Thaw = T{
		Name:        "thaw",
		Target:      "thawed",
		Progress:    "thawing",
		Local:       true,
		LocalExpect: "unset",
		Kinds:       []kind.T{kind.Svc, kind.Vol},
	}
	TOC = T{
		Name:        "toc",
		Progress:    "tocing",
		Order:       ordering.Desc,
		LocalExpect: "",
	}
	Unprovision = T{
		Name:     "unprovision",
		Target:   "unprovisioned",
		Progress: "unprovisioning",
		Local:    true,
		Order:    ordering.Desc,
		Kinds:    []kind.T{kind.Svc, kind.Vol},
	}
)

package objectactionprops

import (
	"opensvc.com/opensvc/core/kind"
	"opensvc.com/opensvc/core/ordering"
)

type (
	T struct {
		Name                  string
		Target                string
		Progress              string
		Order                 ordering.T
		LocalExpect           string
		Local                 bool
		Freeze                bool
		Kinds                 []kind.T
		DisableNodeValidation bool
		RelayToAny            bool
		Rollback              bool
		PG                    bool
		TimeoutKeywords       []string
	}
)

var (
	Abort = T{
		Name:        "abort",
		Target:      "aborted",
		Progress:    "aborting",
		LocalExpect: "unset",
	}
	Decode = T{
		Name:       "decode",
		RelayToAny: true,
		Kinds:      []kind.T{kind.Usr, kind.Sec, kind.Cfg},
	}
	Delete = T{
		Name:       "delete",
		Target:     "deleted",
		Progress:   "deleting",
		Order:      ordering.Desc,
		Local:      true,
		RelayToAny: true,
		Kinds:      []kind.T{kind.Svc, kind.Vol, kind.Usr, kind.Sec, kind.Cfg},
	}
	Eval = T{
		Name:       "eval",
		RelayToAny: true,
	}
	Freeze = T{
		Name:        "freeze",
		Target:      "frozen",
		Progress:    "freezing",
		Local:       true,
		LocalExpect: "unset",
		Kinds:       []kind.T{kind.Svc, kind.Vol},
	}
	GenCert = T{
		Name:       "gen_cert",
		RelayToAny: true,
	}
	Get = T{
		Name:       "get",
		RelayToAny: true,
	}
	Set = T{
		Name:       "set",
		RelayToAny: true,
	}
	SetProvisioned = T{
		Name:  "set provisioned",
		Local: true,
	}
	SetUnprovisioned = T{
		Name:  "set unprovisioned",
		Local: true,
	}
	Status = T{
		Name: "status",
		PG:   true,
	}
	Unset = T{
		Name:       "unset",
		RelayToAny: true,
	}
	Giveback = T{
		Name:            "giveback",
		Target:          "placed",
		Progress:        "placing",
		LocalExpect:     "unset",
		Kinds:           []kind.T{kind.Svc},
		TimeoutKeywords: []string{"start_timeout", "timeout"},
	}
	Keys = T{
		Name:       "keys",
		RelayToAny: true,
	}
	ValidateConfig = T{
		Name:       "validate_config",
		RelayToAny: true,
	}
	Move = T{
		Name:            "move",
		Target:          "placed@",
		Progress:        "placing@",
		LocalExpect:     "unset",
		Kinds:           []kind.T{kind.Svc},
		TimeoutKeywords: []string{"start_timeout", "timeout"},
	}
	Provision = T{
		Name:            "provision",
		Target:          "provisioned",
		Progress:        "provisioning",
		Local:           true,
		LocalExpect:     "unset",
		Kinds:           []kind.T{kind.Svc, kind.Vol},
		Rollback:        true,
		TimeoutKeywords: []string{"unprovision_timeout", "timeout"},
		PG:              true,
	}
	Purge = T{
		Name:            "purge",
		Target:          "purged",
		Progress:        "purging",
		Order:           ordering.Desc,
		Local:           true,
		Kinds:           []kind.T{kind.Svc, kind.Vol, kind.Usr, kind.Sec, kind.Cfg},
		TimeoutKeywords: []string{"unprovision_timeout", "timeout"},
	}
	Restart = T{
		Name:            "restart",
		Target:          "restarted",
		Progress:        "restarting",
		Local:           true,
		LocalExpect:     "unset",
		Kinds:           []kind.T{kind.Svc, kind.Vol},
		TimeoutKeywords: []string{"start_timeout", "timeout"},
		PG:              true,
	}
	Run = T{
		Name:            "run",
		Local:           true,
		Kinds:           []kind.T{kind.Svc, kind.Vol},
		TimeoutKeywords: []string{"run_timeout", "timeout"},
		PG:              true,
	}
	Shutdown = T{
		Name:            "shutdown",
		Target:          "shutdown",
		Progress:        "shutting",
		Local:           true,
		Order:           ordering.Desc,
		Kinds:           []kind.T{kind.Svc, kind.Vol},
		TimeoutKeywords: []string{"stop_timeout", "timeout"},
		PG:              true,
	}
	Start = T{
		Name:            "start",
		Target:          "started",
		Progress:        "starting",
		Local:           true,
		LocalExpect:     "unset",
		Kinds:           []kind.T{kind.Svc, kind.Vol},
		Rollback:        true,
		TimeoutKeywords: []string{"start_timeout", "timeout"},
		PG:              true,
	}
	Stop = T{
		Name:            "stop",
		Target:          "stopped",
		Progress:        "stopping",
		Local:           true,
		Order:           ordering.Desc,
		LocalExpect:     "",
		Kinds:           []kind.T{kind.Svc, kind.Vol},
		Freeze:          true,
		TimeoutKeywords: []string{"stop_timeout", "timeout"},
		PG:              true,
	}
	Switch = T{
		Name:            "switch",
		Target:          "placed@",
		Progress:        "placing@",
		LocalExpect:     "unset",
		Kinds:           []kind.T{kind.Svc},
		TimeoutKeywords: []string{"start_timeout", "timeout"},
	}
	SyncResync = T{
		Name:  "sync_resync",
		Local: true,
		Kinds: []kind.T{kind.Svc, kind.Vol},
		PG:    true,
	}
	Takeover = T{
		Name:            "takeover",
		Target:          "placed@",
		Progress:        "placing@",
		LocalExpect:     "unset",
		Kinds:           []kind.T{kind.Svc},
		TimeoutKeywords: []string{"start_timeout", "timeout"},
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
		Name:            "unprovision",
		Target:          "unprovisioned",
		Progress:        "unprovisioning",
		Local:           true,
		Order:           ordering.Desc,
		Kinds:           []kind.T{kind.Svc, kind.Vol},
		TimeoutKeywords: []string{"unprovision_timeout", "timeout"},
		PG:              true,
	}
)

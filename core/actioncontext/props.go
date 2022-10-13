package actioncontext

import (
	"opensvc.com/opensvc/core/kind"
	"opensvc.com/opensvc/core/ordering"
)

type (
	Properties struct {
		Name                  string
		Target                string
		Progress              string
		Order                 ordering.T
		LocalExpect           string
		Local                 bool
		MustLock              bool
		LockGroup             string
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
	Abort = Properties{
		Name:        "abort",
		Target:      "aborted",
		Progress:    "aborting",
		LocalExpect: "unset",
	}
	Decode = Properties{
		Name:       "decode",
		RelayToAny: true,
		Kinds:      []kind.T{kind.Usr, kind.Sec, kind.Cfg},
	}
	Delete = Properties{
		Name:       "delete",
		Target:     "deleted",
		Progress:   "deleting",
		Order:      ordering.Desc,
		Local:      true,
		MustLock:   true,
		RelayToAny: true,
		Kinds:      []kind.T{kind.Svc, kind.Vol, kind.Usr, kind.Sec, kind.Cfg},
	}
	Eval = Properties{
		Name:       "eval",
		RelayToAny: true,
	}
	Freeze = Properties{
		Name:        "freeze",
		Target:      "frozen",
		Progress:    "freezing",
		Local:       true,
		LocalExpect: "unset",
		Kinds:       []kind.T{kind.Svc, kind.Vol},
		PG:          true,
	}
	GenCert = Properties{
		Name:       "gen_cert",
		RelayToAny: true,
	}
	Get = Properties{
		Name:       "get",
		RelayToAny: true,
	}
	Set = Properties{
		Name:       "set",
		RelayToAny: true,
		MustLock:   true,
	}
	SetProvisioned = Properties{
		Name:     "set provisioned",
		Local:    true,
		MustLock: true,
	}
	SetUnprovisioned = Properties{
		Name:     "set unprovisioned",
		Local:    true,
		MustLock: true,
	}
	Status = Properties{
		Name:      "status",
		PG:        true,
		MustLock:  true,
		LockGroup: "status",
	}
	Unset = Properties{
		Name:       "unset",
		MustLock:   true,
		RelayToAny: true,
	}
	Giveback = Properties{
		Name:            "giveback",
		Target:          "placed",
		Progress:        "placing",
		LocalExpect:     "unset",
		Kinds:           []kind.T{kind.Svc},
		TimeoutKeywords: []string{"start_timeout", "timeout"},
	}
	Keys = Properties{
		Name:       "keys",
		RelayToAny: true,
	}
	ValidateConfig = Properties{
		Name:       "validate_config",
		RelayToAny: true,
		MustLock:   true,
	}
	Move = Properties{
		Name:            "move",
		Target:          "placed@",
		Progress:        "placing@",
		LocalExpect:     "unset",
		Kinds:           []kind.T{kind.Svc},
		TimeoutKeywords: []string{"start_timeout", "timeout"},
	}
	Provision = Properties{
		Name:            "provision",
		Target:          "provisioned",
		Progress:        "provisioning",
		Local:           true,
		MustLock:        true,
		LocalExpect:     "unset",
		Kinds:           []kind.T{kind.Svc, kind.Vol},
		Rollback:        true,
		TimeoutKeywords: []string{"unprovision_timeout", "timeout"},
		PG:              true,
	}
	PRStart = Properties{
		Name:     "prstart",
		Local:    true,
		Kinds:    []kind.T{kind.Svc, kind.Vol},
		Rollback: true,
		PG:       true,
	}
	PRStop = Properties{
		Name:  "prstop",
		Local: true,
		Kinds: []kind.T{kind.Svc, kind.Vol},
		PG:    true,
	}
	Purge = Properties{
		Name:            "purge",
		Target:          "purged",
		Progress:        "purging",
		Order:           ordering.Desc,
		Local:           true,
		Kinds:           []kind.T{kind.Svc, kind.Vol, kind.Usr, kind.Sec, kind.Cfg},
		TimeoutKeywords: []string{"unprovision_timeout", "timeout"},
	}
	Restart = Properties{
		Name:            "restart",
		Target:          "restarted",
		Progress:        "restarting",
		Local:           true,
		LocalExpect:     "unset",
		Kinds:           []kind.T{kind.Svc, kind.Vol},
		TimeoutKeywords: []string{"start_timeout", "timeout"},
		PG:              true,
	}
	Run = Properties{
		Name:            "run",
		Local:           true,
		Kinds:           []kind.T{kind.Svc, kind.Vol},
		TimeoutKeywords: []string{"run_timeout", "timeout"},
		PG:              true,
	}
	Shutdown = Properties{
		Name:            "shutdown",
		Target:          "shutdown",
		Progress:        "shutting",
		Local:           true,
		Order:           ordering.Desc,
		Kinds:           []kind.T{kind.Svc, kind.Vol},
		TimeoutKeywords: []string{"stop_timeout", "timeout"},
		PG:              true,
	}
	Start = Properties{
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
	Stop = Properties{
		Name:            "stop",
		Target:          "stopped",
		Progress:        "stopping",
		Local:           true,
		MustLock:        true,
		Order:           ordering.Desc,
		LocalExpect:     "",
		Kinds:           []kind.T{kind.Svc, kind.Vol},
		Freeze:          true,
		TimeoutKeywords: []string{"stop_timeout", "timeout"},
		PG:              true,
	}
	Switch = Properties{
		Name:            "switch",
		Target:          "placed@",
		Progress:        "placing@",
		LocalExpect:     "unset",
		Kinds:           []kind.T{kind.Svc},
		TimeoutKeywords: []string{"start_timeout", "timeout"},
	}
	SyncResync = Properties{
		Name:     "sync_resync",
		Local:    true,
		MustLock: true,
		Kinds:    []kind.T{kind.Svc, kind.Vol},
		PG:       true,
	}
	Takeover = Properties{
		Name:            "takeover",
		Target:          "placed@",
		Progress:        "placing@",
		LocalExpect:     "unset",
		Kinds:           []kind.T{kind.Svc},
		TimeoutKeywords: []string{"start_timeout", "timeout"},
	}
	TOC = Properties{
		Name:        "toc",
		Progress:    "tocing",
		Order:       ordering.Desc,
		LocalExpect: "",
	}
	Unfreeze = Properties{
		Name:        "unfreeze",
		Target:      "thawed",
		Progress:    "thawing",
		Local:       true,
		LocalExpect: "unset",
		Kinds:       []kind.T{kind.Svc, kind.Vol},
		PG:          true,
	}
	Unprovision = Properties{
		Name:            "unprovision",
		Target:          "unprovisioned",
		Progress:        "unprovisioning",
		Local:           true,
		MustLock:        true,
		Order:           ordering.Desc,
		Kinds:           []kind.T{kind.Svc, kind.Vol},
		TimeoutKeywords: []string{"unprovision_timeout", "timeout"},
		PG:              true,
	}
)

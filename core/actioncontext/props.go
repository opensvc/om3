package actioncontext

import (
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/ordering"
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
		Kinds                 naming.Kinds
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
	Boot = Properties{
		Name:            "boot",
		Target:          "booted",
		Progress:        "booting",
		Local:           true,
		MustLock:        true,
		Order:           ordering.Desc,
		LocalExpect:     "",
		Kinds:           naming.NewKinds(naming.KindSvc, naming.KindVol),
		Freeze:          true,
		TimeoutKeywords: []string{"timeout"},
		PG:              true,
	}
	Decode = Properties{
		Name:       "decode",
		RelayToAny: true,
		Kinds:      naming.NewKinds(naming.KindCfg, naming.KindSec, naming.KindUsr),
	}
	Delete = Properties{
		Name:       "delete",
		Target:     "deleted",
		Progress:   "deleting",
		Order:      ordering.Desc,
		Local:      true,
		MustLock:   true,
		RelayToAny: true,
		Kinds:      naming.NewKinds(naming.KindSvc, naming.KindVol, naming.KindCfg, naming.KindSec, naming.KindUsr),
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
		Kinds:       naming.NewKinds(naming.KindSvc, naming.KindVol),
		PG:          true,
	}
	SyncFull = Properties{
		Name:     "sync_full",
		Local:    true,
		Progress: "syncing",
		MustLock: true,
		Kinds:    naming.NewKinds(naming.KindSvc, naming.KindVol),
		PG:       true,
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
	Enable = Properties{
		Name:       "enable",
		MustLock:   true,
		RelayToAny: true,
	}
	Disable = Properties{
		Name:       "disable",
		MustLock:   true,
		RelayToAny: true,
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
		Kinds:           naming.NewKinds(naming.KindSvc),
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
		Kinds:           naming.NewKinds(naming.KindSvc),
		TimeoutKeywords: []string{"start_timeout", "timeout"},
	}
	Provision = Properties{
		Name:            "provision",
		Target:          "provisioned",
		Progress:        "provisioning",
		Local:           true,
		MustLock:        true,
		LocalExpect:     "unset",
		Kinds:           naming.NewKinds(naming.KindSvc, naming.KindVol),
		Rollback:        true,
		TimeoutKeywords: []string{"unprovision_timeout", "timeout"},
		PG:              true,
	}
	PRStart = Properties{
		Name:     "prstart",
		Local:    true,
		Kinds:    naming.NewKinds(naming.KindSvc, naming.KindVol),
		Rollback: true,
		PG:       true,
	}
	PRStop = Properties{
		Name:  "prstop",
		Local: true,
		Kinds: naming.NewKinds(naming.KindSvc, naming.KindVol),
		PG:    true,
	}
	PushResInfo = Properties{
		Name:  "push resinfo",
		Local: true,
		Kinds: naming.NewKinds(naming.KindSvc, naming.KindVol),
		PG:    true,
	}
	Purge = Properties{
		Name:            "purge",
		Target:          "purged",
		Progress:        "purging",
		Order:           ordering.Desc,
		Local:           true,
		Kinds:           naming.NewKinds(naming.KindSvc, naming.KindVol, naming.KindCfg, naming.KindSec, naming.KindUsr),
		TimeoutKeywords: []string{"unprovision_timeout", "timeout"},
	}
	Restart = Properties{
		Name:            "restart",
		Target:          "restarted",
		Progress:        "restarting",
		Local:           true,
		LocalExpect:     "unset",
		Kinds:           naming.NewKinds(naming.KindSvc, naming.KindVol),
		TimeoutKeywords: []string{"start_timeout", "timeout"},
		PG:              true,
	}
	Run = Properties{
		Name:            "run",
		Progress:        "running",
		Local:           true,
		Kinds:           naming.NewKinds(naming.KindSvc, naming.KindVol),
		TimeoutKeywords: []string{"run_timeout", "timeout"},
		PG:              true,
	}
	Shutdown = Properties{
		Name:            "shutdown",
		Target:          "shutdown",
		Progress:        "shutting",
		Local:           true,
		Order:           ordering.Desc,
		Kinds:           naming.NewKinds(naming.KindSvc, naming.KindVol),
		TimeoutKeywords: []string{"stop_timeout", "timeout"},
		PG:              true,
	}
	Start = Properties{
		Name:            "start",
		Target:          "started",
		Progress:        "starting",
		Local:           true,
		LocalExpect:     "unset",
		Kinds:           naming.NewKinds(naming.KindSvc, naming.KindVol),
		Rollback:        true,
		TimeoutKeywords: []string{"start_timeout", "timeout"},
		PG:              true,
	}
	StartStandby = Properties{
		Name:            "startstandby",
		Progress:        "starting",
		Local:           true,
		LocalExpect:     "unset",
		Kinds:           naming.NewKinds(naming.KindSvc, naming.KindVol),
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
		Kinds:           naming.NewKinds(naming.KindSvc, naming.KindVol),
		Freeze:          true,
		TimeoutKeywords: []string{"stop_timeout", "timeout"},
		PG:              true,
	}
	Switch = Properties{
		Name:            "switch",
		Target:          "placed@",
		Progress:        "placing@",
		LocalExpect:     "unset",
		Kinds:           naming.NewKinds(naming.KindSvc),
		TimeoutKeywords: []string{"start_timeout", "timeout"},
	}
	SyncResync = Properties{
		Name:     "sync_resync",
		Local:    true,
		Progress: "syncing",
		MustLock: true,
		Kinds:    naming.NewKinds(naming.KindSvc, naming.KindVol),
		PG:       true,
	}
	Takeover = Properties{
		Name:            "takeover",
		Target:          "placed@",
		Progress:        "placing@",
		LocalExpect:     "unset",
		Kinds:           naming.NewKinds(naming.KindSvc),
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
		Kinds:       naming.NewKinds(naming.KindSvc, naming.KindVol),
		PG:          true,
	}
	Unprovision = Properties{
		Name:            "unprovision",
		Target:          "unprovisioned",
		Progress:        "unprovisioning",
		Local:           true,
		MustLock:        true,
		Order:           ordering.Desc,
		Kinds:           naming.NewKinds(naming.KindSvc, naming.KindVol),
		TimeoutKeywords: []string{"unprovision_timeout", "timeout"},
		PG:              true,
	}
	SyncIngest = Properties{
		Name:     "sync_ingest",
		Local:    true,
		Progress: "syncing",
		MustLock: true,
		Kinds:    naming.NewKinds(naming.KindSvc, naming.KindVol),
		PG:       true,
	}
	SyncUpdate = Properties{
		Name:     "sync_update",
		Progress: "syncing",
		Local:    true,
		MustLock: true,
		Kinds:    naming.NewKinds(naming.KindSvc, naming.KindVol),
		PG:       true,
	}
)

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
		Failure               string
		Order                 ordering.T
		LocalExpect           string
		Local                 bool
		MustLock              bool
		LockGroup             string
		Freeze                bool
		Kinds                 naming.Kinds
		DisableNodeValidation bool
		Rollback              bool
		PG                    bool
		TimeoutKeywords       []string
	}
)

var (
	Boot = Properties{
		Name:            "boot",
		Target:          "booted",
		Progress:        "booting",
		Failure:         "boot failed",
		Local:           true,
		MustLock:        true,
		Order:           ordering.Desc,
		LocalExpect:     "",
		Kinds:           naming.NewKinds(naming.KindSvc, naming.KindVol),
		Freeze:          true,
		TimeoutKeywords: []string{"timeout"},
		PG:              true,
	}
	Delete = Properties{
		Name:     "delete",
		Target:   "deleted",
		Progress: "deleting",
		Failure:  "delete failed",
		Order:    ordering.Desc,
		Local:    true,
		MustLock: true,
		Kinds:    naming.NewKinds(naming.KindSvc, naming.KindVol, naming.KindCfg, naming.KindSec, naming.KindUsr),
	}
	Freeze = Properties{
		Name:        "freeze",
		Target:      "frozen",
		Progress:    "freezing",
		Failure:     "freeze failed",
		Local:       true,
		LocalExpect: "unset",
		Kinds:       naming.NewKinds(naming.KindSvc, naming.KindVol),
		PG:          true,
	}
	SyncFull = Properties{
		Name:     "sync_full",
		Local:    true,
		Progress: "syncing",
		Failure:  "idle",
		MustLock: true,
		Kinds:    naming.NewKinds(naming.KindSvc, naming.KindVol),
		PG:       true,
	}
	Set = Properties{
		Name:     "set",
		MustLock: true,
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
		Name:     "enable",
		MustLock: true,
	}
	Disable = Properties{
		Name:     "disable",
		MustLock: true,
	}
	Unset = Properties{
		Name:     "unset",
		MustLock: true,
	}
	ValidateConfig = Properties{
		Name:     "validate_config",
		MustLock: true,
	}
	Provision = Properties{
		Name:            "provision",
		Target:          "provisioned",
		Progress:        "provisioning",
		Failure:         "provision failed",
		Local:           true,
		MustLock:        true,
		LocalExpect:     "unset",
		Kinds:           naming.NewKinds(naming.KindSvc, naming.KindVol),
		Rollback:        true,
		TimeoutKeywords: []string{"unprovision_timeout", "timeout"},
		PG:              true,
	}
	PushResInfo = Properties{
		Name:  "push resinfo",
		Local: true,
		Kinds: naming.NewKinds(naming.KindSvc, naming.KindVol),
		PG:    true,
	}
	Run = Properties{
		Name:            "run",
		Progress:        "running",
		Failure:         "idle",
		Local:           true,
		Kinds:           naming.NewKinds(naming.KindSvc, naming.KindVol),
		TimeoutKeywords: []string{"run_timeout", "timeout"},
		PG:              true,
	}
	Shutdown = Properties{
		Name:            "shutdown",
		Target:          "shutdown",
		Progress:        "shutting",
		Failure:         "shutdown failed",
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
		Failure:         "start failed",
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
		Failure:         "start failed",
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
		Failure:         "stop failed",
		Local:           true,
		MustLock:        true,
		Order:           ordering.Desc,
		LocalExpect:     "",
		Kinds:           naming.NewKinds(naming.KindSvc, naming.KindVol),
		Freeze:          true,
		TimeoutKeywords: []string{"stop_timeout", "timeout"},
		PG:              true,
	}
	SyncResync = Properties{
		Name:     "sync_resync",
		Local:    true,
		Progress: "syncing",
		Failure:  "idle",
		MustLock: true,
		Kinds:    naming.NewKinds(naming.KindSvc, naming.KindVol),
		PG:       true,
	}
	Unfreeze = Properties{
		Name:        "unfreeze",
		Target:      "thawed",
		Progress:    "thawing",
		Failure:     "unfreeze failed",
		Local:       true,
		LocalExpect: "unset",
		Kinds:       naming.NewKinds(naming.KindSvc, naming.KindVol),
		PG:          true,
	}
	Unprovision = Properties{
		Name:            "unprovision",
		Target:          "unprovisioned",
		Progress:        "unprovisioning",
		Failure:         "unprovision failed",
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
		Failure:  "idle",
		MustLock: true,
		Kinds:    naming.NewKinds(naming.KindSvc, naming.KindVol),
		PG:       true,
	}
	SyncUpdate = Properties{
		Name:     "sync_update",
		Progress: "syncing",
		Failure:  "idle",
		Local:    true,
		MustLock: true,
		Kinds:    naming.NewKinds(naming.KindSvc, naming.KindVol),
		PG:       true,
	}
)

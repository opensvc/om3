package actioncontext

import (
	"github.com/opensvc/om3/core/ordering"
)

type (
	Properties struct {
		Name            string
		Target          string
		Progress        string
		Failure         string
		Order           ordering.T
		MustLock        bool
		LockGroup       string
		Freeze          bool
		Rollback        bool
		PG              bool
		TimeoutKeywords []string
	}
)

var (
	Boot = Properties{
		Name:            "boot",
		Target:          "booted",
		Progress:        "booting",
		Failure:         "boot failed",
		MustLock:        true,
		Order:           ordering.Desc,
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
		MustLock: true,
	}
	Freeze = Properties{
		Name:     "freeze",
		Target:   "frozen",
		Progress: "freezing",
		Failure:  "freeze failed",
		PG:       true,
	}
	Set = Properties{
		Name:     "set",
		MustLock: true,
	}
	SetProvisioned = Properties{
		Name:     "set provisioned",
		MustLock: true,
	}
	SetUnprovisioned = Properties{
		Name:     "set unprovisioned",
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
		MustLock:        true,
		Rollback:        true,
		TimeoutKeywords: []string{"unprovision_timeout", "timeout"},
		PG:              true,
	}
	PushResInfo = Properties{
		Name: "push resinfo",
		PG:   true,
	}
	Run = Properties{
		Name:            "run",
		TimeoutKeywords: []string{"run_timeout", "timeout"},
		PG:              true,
	}
	Shutdown = Properties{
		Name:            "shutdown",
		Target:          "shutdown",
		Progress:        "shutting",
		Failure:         "shutdown failed",
		Order:           ordering.Desc,
		TimeoutKeywords: []string{"stop_timeout", "timeout"},
		PG:              true,
	}
	Start = Properties{
		Name:            "start",
		Target:          "started",
		Progress:        "starting",
		Failure:         "start failed",
		Rollback:        true,
		TimeoutKeywords: []string{"start_timeout", "timeout"},
		PG:              true,
	}
	StartStandby = Properties{
		Name:            "startstandby",
		Progress:        "starting",
		Failure:         "start failed",
		Rollback:        true,
		TimeoutKeywords: []string{"start_timeout", "timeout"},
		PG:              true,
	}
	Stop = Properties{
		Name:            "stop",
		Target:          "stopped",
		Progress:        "stopping",
		Failure:         "stop failed",
		MustLock:        true,
		Order:           ordering.Desc,
		Freeze:          true,
		TimeoutKeywords: []string{"stop_timeout", "timeout"},
		PG:              true,
	}
	SyncFull = Properties{
		Name:     "sync_full",
		MustLock: true,
		PG:       true,
	}
	SyncIngest = Properties{
		Name:     "sync_ingest",
		MustLock: true,
		PG:       true,
	}
	SyncResync = Properties{
		Name:     "sync_resync",
		MustLock: true,
		PG:       true,
	}
	SyncUpdate = Properties{
		Name:     "sync_update",
		MustLock: true,
		PG:       true,
	}
	Unfreeze = Properties{
		Name:     "unfreeze",
		Target:   "thawed",
		Progress: "thawing",
		Failure:  "unfreeze failed",
		PG:       true,
	}
	Unprovision = Properties{
		Name:            "unprovision",
		Target:          "unprovisioned",
		Progress:        "unprovisioning",
		Failure:         "unprovision failed",
		MustLock:        true,
		Order:           ordering.Desc,
		TimeoutKeywords: []string{"unprovision_timeout", "timeout"},
		PG:              true,
	}
)

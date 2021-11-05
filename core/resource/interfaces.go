package resource

import (
	"context"

	"opensvc.com/opensvc/core/actionresdeps"
	"opensvc.com/opensvc/core/schedule"
)

type (
	//
	// Aborter implements the Abort func, which can return true to
	// block a start action before any resource has entered Start().
	//
	Aborter interface {
		Abort(ctx context.Context) bool
	}

	//
	// ActionResourceDepser implements the ActionResourceDeps func, which
	// return a list of {action, rid depending on, rid depended on} tuples.
	//
	ActionResourceDepser interface {
		ActionResourceDeps() []actionresdeps.Dep
	}

	//
	// Scheduler implements the Schedules func, which returns the list of
	// schedulable job definition on behalf of the resource.
	//
	Scheduler interface {
		Schedules() schedule.Table
	}

	//
	// StatusInfoer implements the StatusInfo func, which returns a
	// resource specific key-val mapping pushed to the collector on
	// "pushinfo" action.
	//
	StatusInfoer interface {
		StatusInfo() map[string]interface{}
	}

	resyncer interface {
		Resync(context.Context) error
	}
)

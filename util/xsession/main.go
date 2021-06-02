package xsession

import (
	"os"

	"github.com/google/uuid"
)

var (
	//
	// ID is an uuid identifying the command execution.
	//
	// This uuid is embedded in the logs so it's easy to retrieve
	// the logs of an execution.
	//
	// Asynchronous commands posted on the API return a ID,
	// so logs can be streamed for this execution after posting.
	//
	// The opensvc daemon forges an ID and exports it in
	// the CRM commands it executes.
	//
	// The ID is also used as a caching session. Spawned
	// subprocesses using the "cache" package store and retrieve
	// their out, err, ret from the session cache identified by
	// the spawner ID.
	//
	ID string
)

func getID() string {
	id := os.Getenv("OSVC_SESSION_ID")
	if id == "" {
		// No uuid set. Generate a new one.
		return newID()
	}
	if _, err := uuid.Parse(id); err != nil {
		// Invalid uuid format. Generate a new one.
		return newID()
	}
	return id
}

func newID() string {
	return uuid.New().String()
}

// for init() test
func initID() {
	ID = getID()
}

func init() {
	initID()
}

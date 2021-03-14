package config

import (
	"os"

	"github.com/google/uuid"
)

var (
	//
	// SessionID is an uuid identifying the command execution.
	//
	// This uuid is embedded in the logs so it's easy to retrieve
	// the logs of an execution.
	//
	// Asynchronous commands posted on the API return a SessionID,
	// so logs can be streamed for this execution after posting.
	//
	// The opensvc daemon forges an SessionID and exports it in
	// the CRM commands it executes.
	//
	// The SessionID is also used as a caching session. Spawned
	// subprocesses using the "cache" package store and retrieve
	// their out, err, ret from the session cache identified by
	// the spawner SessionID.
	//
	SessionID string
)

func init() {
	var err error
	SessionID = os.Getenv("OSVC_SESSION_ID")
	if SessionID == "" {
		// No uuid set. Generate a new one.
		SessionID = uuid.New().String()
		return
	}
	if _, err = uuid.Parse(SessionID); err != nil {
		// Invalid uuid format. Generate a new one.
		SessionID = uuid.New().String()
	}
}

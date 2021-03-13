package config

import (
	"os"

	"github.com/google/uuid"
)

var (
	SessionId string
)

func init() {
	var err error
	SessionId = os.Getenv("OSVC_SESSION_ID")
	if SessionId == "" {
		// No uuid set. Generate a new one.
		SessionId = uuid.New().String()
		return
	}
	if _, err = uuid.Parse(SessionId); err != nil {
		// Invalid uuid format. Generate a new one.
		SessionId = uuid.New().String()
	}
}

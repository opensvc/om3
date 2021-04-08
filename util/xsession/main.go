package xsession

import (
	"github.com/google/uuid"
	"os"
)

var (
	id string
)

func initValue() {
	id = os.Getenv("OSVC_SESSION_ID")
	if id == "" {
		// No uuid set. Generate a new one.
		id = uuid.New().String()
		return
	}
	if _, err := uuid.Parse(id); err != nil {
		// Invalid uuid format. Generate a new one.
		id = uuid.New().String()
	}
}

func Id() string {
	if id != "" {
		return id
	}
	initValue()
	return id
}

package daemonsys

import (
	"context"
	"os"

	sddaemon "github.com/coreos/go-systemd/daemon"
	"github.com/coreos/go-systemd/v22/dbus"
)

type (
	// T handle dbus.Conn for systemd
	T struct {
		conn *dbus.Conn
	}
)

const (
	name = "opensvc-agent.service"
)

// Activated detects if opensvc unit is activated
func (t *T) Activated(ctx context.Context) (bool, error) {
	prop, err := t.conn.GetUnitPropertyContext(ctx, name, "ActiveState")
	if err != nil {
		return false, err
	}
	if prop == nil {
		return false, nil
	}
	return prop.Value.String() == "\"active\"", nil
}

// CalledFromManager detects if current process as been launched by systemd
func (t *T) CalledFromManager() bool {
	return os.Getenv("INVOCATION_ID") != ""
}

// Close closes systemd dbus connection
func (t *T) Close() error {
	if t.conn != nil {
		t.conn.Close()
	}
	return nil
}

// NotifyWatchdog sends watch dog notify to systemd
func (t *T) NotifyWatchdog() (bool, error) {
	if t.conn == nil {
		return false, nil
	}
	return sddaemon.SdNotify(false, sddaemon.SdNotifyWatchdog)
}

// Start starts the opensvc systemd unit
func (t *T) Start(ctx context.Context) error {
	c := make(chan string)
	_, err := t.conn.StartUnitContext(ctx, name, "replace", c)
	if err != nil {
		return err
	}
	<-c
	return nil
}

// Stop stops the opensvc systemd unit
func (t *T) Stop(ctx context.Context) error {
	c := make(chan string)
	_, err := t.conn.StopUnitContext(ctx, name, "replace", c)
	if err != nil {
		return err
	}
	<-c
	return nil
}

// New provides a connected object to dbus systemd that implement following interfaces:
//
//	Activated(ctx context.Context) (bool, error)
//	CalledFromManager() bool
//	Close() error
//	NotifyWatchdog() (bool, error)
//	Start(ctx context.Context) error
//	Stop(ctx context.Context) error
func New(ctx context.Context) (*T, error) {
	c, err := dbus.NewSystemdConnectionContext(ctx)
	if err != nil {
		return nil, err
	}
	return &T{conn: c}, nil
}

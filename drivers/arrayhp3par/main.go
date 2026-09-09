// Package arrayhp3par drives an HPE 3PAR array through its command line
// interface.
//
// It is a port of the v2 agent driver, which reports the configuration of the
// array to the collector and reads it on the command line. The array is
// reached in one of three ways, as v2 reaches it: over ssh, through the cli
// binary installed on this node, or through a proxy serving the cli to nodes
// that cannot reach the array themselves.
package arrayhp3par

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/opensvc/om3/v3/core/array"
	"github.com/opensvc/om3/v3/core/datarecv"
	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/drivers/shared/hp3par"
	"github.com/opensvc/om3/v3/util/key"
	"github.com/opensvc/om3/v3/util/plog"
)

type (
	// Array is one 3PAR array, as the node or cluster configuration declares
	// it.
	Array struct {
		array.Array
	}
)

func init() {
	driver.Register(driver.NewID(driver.GroupArray, "hp3par"), NewDriver)
}

// NewDriver returns a 3PAR array driver.
func NewDriver() array.Driver {
	t := New()
	var i any = t
	return i.(array.Driver)
}

// New returns a 3PAR array.
func New() *Array {
	return &Array{}
}

// Run builds the command tree of this array and runs the arguments through
// it. What the tree holds is declared in Actions.
func (t *Array) Run(args []string) error {
	return array.RunActions(context.Background(), t.Actions(), args, os.Stdout)
}

// arrayName is the name the array answers to, which is the "name" keyword when
// it is set and the name of the section otherwise.
func (t Array) arrayName() string {
	if s := t.Config().GetString(t.Key("name")); s != "" {
		return s
	}
	return strings.TrimPrefix(t.Name(), "array#")
}

func (t Array) method() string {
	return t.Config().GetString(t.Key("method"))
}

func (t Array) manager() string {
	return t.Config().GetString(t.Key("manager"))
}

func (t Array) username() string {
	return t.Config().GetString(t.Key("username"))
}

func (t Array) sshKey() string {
	return t.Config().GetString(t.Key("key"))
}

func (t Array) cli() string {
	if s := t.Config().GetString(t.Key("cli")); s != "" {
		return s
	}
	return "cli"
}

func (t Array) passwordFile() string {
	return t.Config().GetString(t.Key("pwf"))
}

// nodeUUID is what the proxy authenticates this node by.
func (t Array) nodeUUID() string {
	return t.Config().GetString(key.T{Section: "node", Option: "uuid"})
}

// connectionConfig resolves what is needed to reach the array.
//
// A key or a password file may be named as a path or as a reference to a
// datastore of the system namespace, which is materialised into a file the
// command line can name. The array configuration is the node's, so the
// reference is read relative to the node rather than to an object.
func (t *Array) connectionConfig() (hp3par.Config, error) {
	config := hp3par.Config{
		Method:  t.method(),
		Manager: t.manager(),
	}
	if config.Manager == "" {
		// v2 names the array by its manager, and falls back to the name of
		// the array when the keyword says nothing.
		config.Manager = t.arrayName()
	}
	switch config.Method {
	case hp3par.MethodSSH:
		config.Username = t.username()
		file, err := t.materialize(t.Config().GetString(t.Key("key")))
		if err != nil {
			return config, fmt.Errorf("%s: key: %w", t.Name(), err)
		}
		config.KeyFile = file
	case hp3par.MethodCLI:
		config.CLI = t.Config().GetString(t.Key("cli"))
		file, err := t.materialize(t.Config().GetString(t.Key("pwf")))
		if err != nil {
			return config, fmt.Errorf("%s: pwf: %w", t.Name(), err)
		}
		config.PWFile = file
	}
	return config, nil
}

// materialize returns the path of a file holding what a keyword names.
//
// A value beginning with a slash is a path already. Anything else names a key
// of a datastore, whose content is written to a file only this user can read,
// so a private key never has to sit on the disk of every node.
func (t *Array) materialize(s string) (string, error) {
	if s == "" || strings.HasPrefix(s, "/") {
		return s, nil
	}
	km, err := datarecv.ParseKeyMetaRel(s, naming.NsSys)
	if err != nil {
		return "", err
	}
	return km.CacheFile()
}

// run runs one cli command on the array and returns what it printed.
func (t *Array) run(ctx context.Context, command string) (string, error) {
	config, err := t.connectionConfig()
	if err != nil {
		return "", err
	}
	return config.Run(ctx, t.log(), command)
}

// log returns the logger the commands of this array are traced with.
func (t *Array) log() *plog.Logger {
	return plog.NewDefaultLogger().WithPrefix("array: "+t.Name()+": ").Attr("array", t.Name())
}

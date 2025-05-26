/*
Package hbcfg provides helpers to create hb drivers from configuration

candidate hb drivers needs to be registered before use

Usage example on hb unicast driver

	import "opensvc.com/opensvc/core/hbcfg"
	type (
		T struct {hbcfg.T} // Concrete object that implement Configure
		tx struct {...}    // concrete object that implement Transmitter
		tx struct {...}    // concrete object that implement Receiver
	)

	func New() hbcfg.Confer {
		var i interface{} = &T{}
		return i.(hbcfg.Confer)
	}

	// register unicast hb driver
	func init() { hbcfg.Register("unicast", NewConfer) }

	func NewTx(..., timeout time.Duration) *tx { ... }
	func NewRx(..., timeout time.Duration) *rx { ... }

	func (t *T) Configure(ctx context.Context) {
		...
		timeout := t.GetDuration("timeout", 5*time.Second)
		rx := NewRx(..., timeout)
		tx := NewTx(..., timeout)
		t.SetTx(rx)
		t.SetRx(tx)
	}
*/
package hbcfg

import (
	"context"
	"time"

	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/hbtype"
	"github.com/opensvc/om3/core/xconfig"
	"github.com/opensvc/om3/util/key"
)

type (
	// T struct implement TConfer
	T struct {
		driver   string
		name     string
		config   *xconfig.T
		rx       hbtype.Receiver
		tx       hbtype.Transmitter
		nodes    []string
		timeout  time.Duration
		interval time.Duration

		// sig configured signature to detect conf changes
		sig string
	}

	// Confer is the interface a hb driver has to implement
	// type struct composed on T have to implement Configure to satisfy Confer
	Confer interface {
		TConfer

		// Configure prepare Receiver and Transmitter concrete objects
		// composed hb driver struct have to implement Configure
		Configure(ctx context.Context)
	}

	// TConfer interface hold getter and setter for hb configuration
	// T struct implement TConfer
	TConfer interface {
		Name() string
		Type() string
		Config() *xconfig.T
		Interval() time.Duration
		Timeout() time.Duration
		Tx() hbtype.Transmitter
		Rx() hbtype.Receiver
		Nodes() []string
		Signature() string

		SetName(string)
		SetDriver(string)
		SetConfig(*xconfig.T)
		SetRx(receiver hbtype.Receiver)
		SetTx(transmitter hbtype.Transmitter)
		SetInterval(time.Duration)
		SetTimeout(time.Duration)
		SetNodes([]string)
		SetSignature(string)
	}
)

func New(name string, config *xconfig.T) Confer {
	hbFamily := config.GetString(key.New(name, "type"))
	fn := Driver(hbFamily)
	if fn == nil {
		return nil
	}
	t := fn()
	t.SetName(name)
	t.SetDriver(hbFamily)
	t.SetConfig(config)
	return t.(Confer)
}

// Register function register a new hb driver confer
func Register(driverName string, fn func() Confer) {
	did := driver.NewID(driver.GroupHeartbeat, driverName)
	driver.Register(did, fn)
}

func Driver(driverName string) func() Confer {
	did := driver.NewID(driver.GroupHeartbeat, driverName)
	i := driver.Get(did)
	if i == nil {
		return nil
	}
	if a, ok := i.(func() Confer); ok {
		return a
	}
	return nil
}

func (t T) Name() string {
	return t.name
}

func (t *T) SetName(name string) {
	t.name = name
}

func (t *T) SetDriver(driver string) {
	t.driver = driver
}

func (t T) Type() string {
	return t.driver
}

func (t *T) Config() *xconfig.T {
	return t.config
}

func (t *T) SetConfig(c *xconfig.T) {
	t.config = c
}

func (t *T) GetBool(s string) bool {
	k := key.New(t.name, s)
	return t.Config().GetBool(k)
}

func (t *T) GetString(s string) string {
	k := key.New(t.name, s)
	return t.Config().GetString(k)
}

func (t *T) GetStringAs(s, impersonate string) string {
	k := key.New(t.name, s)
	return t.Config().GetStringAs(k, impersonate)
}

func (t *T) GetInt(s string) int {
	k := key.New(t.name, s)
	return t.Config().GetInt(k)
}

func (t *T) GetIntAs(s, impersonate string) int {
	k := key.New(t.name, s)
	if i, err := t.Config().EvalAs(k, impersonate); err == nil {
		return i.(int)
	}
	return 0
}

func (t *T) GetDuration(s string, defaultValue time.Duration) time.Duration {
	k := key.New(t.name, s)
	found := t.Config().GetDuration(k)
	if found == nil {
		return defaultValue
	}
	return *found
}

func (t *T) GetStrings(s string) []string {
	k := key.New(t.name, s)
	nodes := t.Config().GetStrings(k)
	return nodes
}

func (t *T) SetRx(rx hbtype.Receiver) {
	t.rx = rx
}

func (t *T) SetTx(tx hbtype.Transmitter) {
	t.tx = tx
}

func (t *T) Rx() hbtype.Receiver {
	return t.rx
}

func (t *T) Tx() hbtype.Transmitter {
	return t.tx
}

func (t *T) SetNodes(nodes []string) {
	t.nodes = nodes
}

func (t *T) Nodes() []string {
	return t.nodes
}

func (t *T) SetInterval(interval time.Duration) {
	t.interval = interval
}

func (t *T) SetTimeout(timeout time.Duration) {
	t.timeout = timeout
}

// SetSignature set a string that identify config details
func (t *T) SetSignature(s string) {
	t.sig = s
}

// Signature return the string identification of config details
func (t *T) Signature() string {
	return t.sig
}

func (t *T) Interval() time.Duration {
	return t.interval
}

func (t *T) Timeout() time.Duration {
	return t.timeout
}

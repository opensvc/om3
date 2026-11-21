package networkl2bridge

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
)

// state is what the setup of the network made on the node, so a later setup
// undoes what the configuration no longer asks for, and the teardown what it
// made only: a port om did not enslave, a vlan interface it did not create, a
// bridge it adopted are never touched.
type state struct {
	// Bridge is the bridge the state is of: a setup of another, as the dev
	// keyword set to adopt one, first undoes what the state records of
	// this one.
	Bridge string `json:"bridge,omitempty"`

	// CreatedBridge says the bridge was created by om, rather than adopted.
	CreatedBridge bool `json:"created_bridge,omitempty"`

	// Ports are the devices om enslaved to the bridge.
	Ports []string `json:"ports,omitempty"`

	// VLANs are the vlan interfaces om created to enslave.
	VLANs []string `json:"vlans,omitempty"`

	// VXLAN is the vxlan device om created and enslaved, empty for none.
	VXLAN string `json:"vxlan,omitempty"`
}

func loadState(p string) (state, error) {
	var st state
	b, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return st, nil
	} else if err != nil {
		return st, err
	}
	err = json.Unmarshal(b, &st)
	return st, err
}

// save writes the state to p, through a temporary file renamed in place, so
// a crash leaves the former state or this one.
func (t state) save(p string) error {
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	b, err := json.Marshal(t)
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

func (t *state) addPort(dev string) {
	if !slices.Contains(t.Ports, dev) {
		t.Ports = append(t.Ports, dev)
	}
}

func (t *state) addVLAN(dev string) {
	if !slices.Contains(t.VLANs, dev) {
		t.VLANs = append(t.VLANs, dev)
	}
}

// removeState removes the state file p, absent being no error.
func removeState(p string) error {
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

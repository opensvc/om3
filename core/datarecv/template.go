package datarecv

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/network"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/util/hostname"
)

type (
	// templateNetwork is a network as a template reads it.
	templateNetwork struct {
		Name    string
		Type    string
		Network string

		// Peers are the other nodes of a network reaching them itself, as
		// this node reaches them, and none for the other networks.
		Peers []network.Peer
	}

	ridder interface {
		RID() string
	}
)

// render renders b, the content of the key k, as a template: the env section
// of the object is the data, and the functions read what the node knows.
//
// It runs on each node at each install, so a file says what the node it is
// installed on knows when it is: its name, its peers, the tunnels to them.
func (t *DataRecv) render(k string, b []byte) ([]byte, error) {
	o, ok := t.to.GetObject().(object.Core)
	if !ok {
		return nil, fmt.Errorf("the receiver of key %s is not an object", k)
	}
	config := o.Config()
	section := ""
	if r, ok := t.to.(ridder); ok {
		section = r.RID()
	}
	namespace := o.Path().Namespace
	funcs := template.FuncMap{
		// ref returns the value of an om reference, written without its
		// braces, evaluated for this node: "nodename", "clusternodes",
		// "env.port", "volume#1.mnt".
		"ref": func(ref string) (string, error) {
			return config.DereferenceAs(ref, section, hostname.Hostname())
		},
		// split returns the words of s, as a list a reference returns.
		"split": strings.Fields,
		// key returns the content of the key name of the datastore at
		// path, relative to the namespace of the object, which must be
		// allowed to read it.
		"key": func(path, name string) (string, error) {
			return templateKey(namespace, path, name)
		},
		// network returns the network name, and its peers.
		"network": templateNetworkOf,
		// networks returns the networks of the node, and their peers.
		"networks": templateNetworks,
		// tunnels returns the tunnels the networks of the node carry
		// their addresses to the other nodes by, once each: networks
		// joining the same two addresses share a tunnel.
		"tunnels": templateTunnels,
	}
	tmpl, err := template.New(k).Option("missingkey=error").Funcs(funcs).Parse(string(b))
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, buildEnvTemplateData(config)); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func templateKey(namespace, path, name string) (string, error) {
	p, err := naming.ParsePathRel(path, namespace)
	if err != nil {
		return "", err
	}
	switch p.Kind {
	case naming.KindSec, naming.KindCfg:
	default:
		return "", fmt.Errorf("%s is not a cfg or sec datastore", p)
	}
	if !p.Exists() {
		return "", fmt.Errorf("datastore %s does not exist", p)
	}
	ds, err := object.NewDataStore(p, object.WithVolatile(true))
	if err != nil {
		return "", err
	}
	if !ds.Allow(namespace) {
		return "", fmt.Errorf("the %s namespace is not allowed to read %s keys", namespace, p)
	}
	b, err := ds.DecodeKey(name)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func templateNetworkOf(name string) (templateNetwork, error) {
	nw, names, err := network.Lookup(name)
	if err != nil {
		return templateNetwork{}, err
	}
	if nw == nil {
		return templateNetwork{}, fmt.Errorf("network %s not found among %s", name, strings.Join(names, ", "))
	}
	return newTemplateNetwork(nw)
}

func templateNetworks() ([]templateNetwork, error) {
	node, err := object.NewNode(object.WithVolatile(true))
	if err != nil {
		return nil, err
	}
	l := make([]templateNetwork, 0)
	for _, nw := range network.Networks(node) {
		data, err := newTemplateNetwork(nw)
		if err != nil {
			return nil, err
		}
		l = append(l, data)
	}
	return l, nil
}

func templateTunnels() ([]network.Peer, error) {
	nws, err := templateNetworks()
	if err != nil {
		return nil, err
	}
	return uniqueTunnels(nws), nil
}

// uniqueTunnels returns the peers of the networks reached by a tunnel, once
// each tunnel.
func uniqueTunnels(nws []templateNetwork) []network.Peer {
	l := make([]network.Peer, 0)
	done := make(map[string]bool)
	for _, nw := range nws {
		for _, peer := range nw.Peers {
			if peer.Tunnel == "" || done[peer.Tunnel] {
				continue
			}
			done[peer.Tunnel] = true
			l = append(l, peer)
		}
	}
	return l
}

func newTemplateNetwork(nw network.Networker) (templateNetwork, error) {
	name := nw.Name()
	data := templateNetwork{
		Name:    name,
		Type:    nw.Type(),
		Network: nw.Network(),
		Peers:   []network.Peer{},
	}
	if p, ok := nw.(network.Peerer); ok {
		var err error
		if data.Peers, err = p.Peers(); err != nil {
			return templateNetwork{}, fmt.Errorf("network %s peers: %w", name, err)
		}
	}
	return data, nil
}

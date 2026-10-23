package object

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/opensvc/om3/v3/core/backup"
	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/util/hostname"
	"github.com/opensvc/om3/v3/util/key"
	"github.com/opensvc/om3/v3/util/sshnode"
)

// The keywords of a store section are the ones of the cluster configuration,
// in a sec too.
func init() {
	for _, kw := range []*keywords.Keyword{&kwNodeStoreType, &kwNodeStoreURL, &kwNodeStoreHostKey, &kwNodeStoreSSHPrivateKey, &kwNodeStoreKeep} {
		c := kw.Clone()
		c.Kind = naming.NewKinds(naming.KindSec)
		keywordStore = append(keywordStore, c)
	}
}

// StoreItem is a store declared in the cluster configuration, or in a sec.
type StoreItem struct {
	// Ref is the reference to the store, as a store keyword names it.
	Ref  string `json:"ref"`
	Type string `json:"type"`
	URL  string `json:"url"`

	// Share are the namespaces whose objects may use the store: every
	// one for a store of the cluster configuration, the namespace of the
	// sec and the ones it shares with otherwise.
	Share []string `json:"share"`
}

// Store returns the driver of the store ref names, as the object at reader
// uses it, zero for a node command, configured from its section.
//
// A bare name is a store#<name> section of the cluster configuration. A
// store of a sec is named from <sec> store#<name>, a ./ path naming a sec of
// the namespace of reader: the stores never clash, a bare name never meaning
// a store of a sec.
//
// A store of a sec is one its namespace, or one it shares with, uses, its
// keys read as the object using it, and a store the daemon reaches over the
// network: a namespace does not choose a directory of the node, which the
// daemon would read and write as root.
func (t *Node) Store(ref string, reader naming.Path) (backup.Driver, error) {
	if ref == "" {
		return nil, fmt.Errorf("no store named")
	}
	if fields := strings.Fields(ref); len(fields) > 1 {
		return t.secStore(fields, reader)
	}
	if strings.Contains(ref, "/") {
		return nil, fmt.Errorf("store %q: name a store of the cluster configuration bare, and one of a sec from <sec> store#<name>", ref)
	}
	section := backup.SectionName(ref)
	config := t.MergedConfig()
	if !config.HasSectionString(section) {
		return nil, fmt.Errorf("store %q: no %s section in the cluster configuration", ref, section)
	}
	storeType := config.GetString(key.New(section, "type"))
	drv, err := storeDriver(ref, storeType)
	if err != nil {
		return nil, err
	}
	drv.SetName(section)
	drv.SetConfig(config)
	if sharer, ok := drv.(backup.Sharer); ok && !sharer.IsShared() {
		return t.nodesStore(drv, ref, section, storeType), nil
	}
	return drv, nil
}

// nodesStore spreads the store drv, a directory of each node, over the
// directories of the cluster nodes: the ones of the peers reached over ssh,
// as root, through the trust om cluster ssh trust sets up.
func (t *Node) nodesStore(drv backup.Driver, ref, section, storeType string) backup.Driver {
	config := t.MergedConfig()
	local := hostname.Hostname()
	peers := make([]string, 0)
	for _, node := range config.GetStrings(key.New("cluster", "nodes")) {
		if node != local {
			peers = append(peers, node)
		}
	}
	keyFile := ""
	if home, err := os.UserHomeDir(); err == nil {
		keyFile = filepath.Join(home, ".ssh", config.GetString(key.New("node", "sshkey")))
	}
	newPeer := func(node string) (backup.Driver, error) {
		peer, err := storeDriver(ref, storeType)
		if err != nil {
			return nil, err
		}
		peer.SetName(section)
		peer.SetConfig(config)
		conn, err := sshnode.NewClient(node, sshnode.WithPrivateKeyFiles(keyFile), sshnode.WithTimeout(storePeerTimeout))
		if err != nil {
			return nil, fmt.Errorf("ssh as root: %w: is the trust of 'om cluster ssh trust' in place?", err)
		}
		if err := peer.(backup.Sharer).OverSSH(conn); err != nil {
			_ = conn.Close()
			return nil, err
		}
		return peer, nil
	}
	return backup.NewNodesStore(drv, local, peers, newPeer, t.Log().Warnf)
}

// storePeerTimeout is how long a node waits for the ssh connection to the
// directory of a store on a peer.
const storePeerTimeout = 10 * time.Second

func (t *Node) secStore(fields []string, reader naming.Path) (backup.Driver, error) {
	ref := strings.Join(fields, " ")
	if len(fields) != 3 || fields[0] != "from" || !strings.HasPrefix(fields[2], backup.SectionPrefix) {
		return nil, fmt.Errorf("store %q: name a store of a sec from <sec> store#<name>", ref)
	}
	var p naming.Path
	var err error
	if reader.IsZero() {
		p, err = naming.ParsePath(fields[1])
	} else {
		p, err = naming.ParsePathRel(fields[1], reader.Namespace)
	}
	if err != nil {
		return nil, fmt.Errorf("store %q: %w", ref, err)
	}
	if p.Kind != naming.KindSec {
		return nil, fmt.Errorf("store %q: %s is not a sec", ref, p)
	}
	if !p.Exists() {
		return nil, fmt.Errorf("store %q: %s does not exist", ref, p)
	}
	sec, err := NewSec(p, WithVolatile(true))
	if err != nil {
		return nil, fmt.Errorf("store %q: %w", ref, err)
	}
	if !reader.IsZero() && !sec.Allow(reader.Namespace) {
		return nil, fmt.Errorf("store %q: the %s namespace may not use the stores of %s, which shares with %s", ref, reader.Namespace, p, strings.Join(sec.Shares(), " "))
	}
	section := fields[2]
	config := sec.Config()
	if !config.HasSectionString(section) {
		return nil, fmt.Errorf("store %q: no %s section in %s", ref, section, p)
	}
	drv, err := storeDriver(ref, config.GetString(key.New(section, "type")))
	if err != nil {
		return nil, err
	}
	drv.SetName(section)
	drv.SetConfig(config)
	drv.SetOwner(p)
	drv.SetReader(reader.Namespace)
	if l, ok := drv.(backup.Localer); ok && l.IsLocal() {
		return nil, fmt.Errorf("store %q: a store of a sec is reached over the network, as an sftp server: the daemon would read and write a directory of the node as root", ref)
	}
	return drv, nil
}

// Cluster returns the id and the name of the cluster of the node, which the
// runs it writes into a store say: several clusters may write one store.
func (t *Node) Cluster() (id, name string) {
	config := t.MergedConfig()
	return config.GetString(key.New("cluster", "id")), config.GetString(key.New("cluster", "name"))
}

func storeDriver(ref, storeType string) (backup.Driver, error) {
	drv := backup.GetDriver(storeType)
	if drv == nil {
		return nil, fmt.Errorf("store %q: no driver for the store type %q", ref, storeType)
	}
	return drv, nil
}

// ListStores returns the stores declared in the cluster configuration, then
// the ones declared in the secs of the node.
func (t *Node) ListStores() []StoreItem {
	l := make([]StoreItem, 0)
	config := t.MergedConfig()
	for _, s := range config.SectionStrings() {
		if !strings.HasPrefix(s, backup.SectionPrefix) {
			continue
		}
		l = append(l, StoreItem{
			Ref:   strings.TrimPrefix(s, backup.SectionPrefix),
			Type:  config.GetString(key.New(s, "type")),
			URL:   config.GetString(key.New(s, "url")),
			Share: []string{"*"},
		})
	}
	paths, err := naming.InstalledPaths()
	if err != nil {
		return l
	}
	for _, p := range paths {
		if p.Kind != naming.KindSec {
			continue
		}
		sec, err := NewSec(p, WithVolatile(true))
		if err != nil {
			continue
		}
		config := sec.Config()
		for _, s := range config.SectionStrings() {
			if !strings.HasPrefix(s, backup.SectionPrefix) {
				continue
			}
			l = append(l, StoreItem{
				Ref:   fmt.Sprintf("from %s %s", p, s),
				Type:  config.GetString(key.New(s, "type")),
				URL:   config.GetString(key.New(s, "url")),
				Share: sec.Shares(),
			})
		}
	}
	return l
}

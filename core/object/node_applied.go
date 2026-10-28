package object

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/util/funcopt"
)

// appliedConfigFile returns the file recording the configuration of the node
// the janitor named consumer last applied.
func appliedConfigFile(consumer string) string {
	return filepath.Join(rawconfig.NodeVarDir(), "applied", consumer+".conf")
}

// SaveApplied records the configuration of the node, the cluster
// configuration merged, as the one the janitor named consumer applied.
//
// A janitor undoes what the configuration it applied held and the one it
// applies no longer does. The configuration of the node changes through the
// daemon, a write made without it, an edit of the file or a restore, so the
// janitor diffs against what it applied, not against a change it was told of:
// a change made while the daemon was down would never be.
func (t *Node) SaveApplied(consumer string) error {
	b, err := t.MergedConfig().Dump()
	if err != nil {
		return err
	}
	p := appliedConfigFile(consumer)
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), "."+filepath.Base(p)+".")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), p)
}

// LoadApplied returns the node configured as it was when the janitor named
// consumer last applied its configuration, or nil when it never did. The node
// is volatile, and opts apply to it as to NewNode.
func LoadApplied(consumer string, opts ...funcopt.O) (*Node, error) {
	b, err := os.ReadFile(appliedConfigFile(consumer))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	opts = append(opts, WithClusterConfigFile(""), WithConfigData(b), WithVolatile(true))
	return NewNode(opts...)
}

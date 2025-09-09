package nodesinfo

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/daemonsubsystem"
	"github.com/opensvc/om3/util/label"
	"github.com/opensvc/om3/util/san"
)

type (
	// M is the dataset exposed via the GET /nodes_info handler,
	// used by nodes to:
	// * expand node selector expressions based on labels
	// * setup clusterwide lun mapping from pools backed by san arrays
	M map[string]T

	T struct {
		Env    string    `json:"env"`
		Labels label.M   `json:"labels"`
		Paths  san.Paths `json:"paths"`

		Lsnr daemonsubsystem.Listener `json:"listener"`
	}
)

func cacheFile() string {
	return filepath.Join(rawconfig.Paths.Var, "nodes_info.json")
}

func cacheFilePair() (final, tmp string) {
	final = cacheFile()
	tmp = filepath.Join(filepath.Dir(final), "."+filepath.Base(final)+".swp")
	return
}

func Save(data M) error {
	p, tmp := cacheFilePair()
	jsonFile, err := os.Create(tmp)
	if err != nil {
		return err
	}
	defer func() { _ = jsonFile.Close() }()
	defer func() { _ = os.Remove(tmp) }()
	enc := json.NewEncoder(jsonFile)
	err = enc.Encode(data)
	if err != nil {
		return err
	}
	if err := os.Rename(tmp, p); err != nil {
		return err
	}
	return nil
}

func Load() (M, error) {
	data := M{}
	p := cacheFile()
	jsonFile, err := os.Open(p)
	if err != nil {
		return data, err
	}
	defer jsonFile.Close()
	dec := json.NewDecoder(jsonFile)
	err = dec.Decode(&data)
	return data, err
}

// GetNodesWithAnyPaths return the list of nodes having any of the given paths.
func (m M) GetNodesWithAnyPaths(paths san.Paths) []string {
	l := make([]string, 0)
	for nodename, node := range m {
		if paths.HasAnyOf(node.Paths) {
			l = append(l, nodename)
		}
	}
	return l
}

func (m M) Keys() []string {
	l := make([]string, len(m))
	i := 0
	for k := range m {
		l[i] = k
		i++
	}
	return l
}

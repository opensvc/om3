package nodesinfo

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/util/san"
	"github.com/opensvc/om3/util/xerrors"
)

type (
	// NodesInfo is the dataset exposed via the GET /nodes_info handler,
	// used by nodes to:
	// * expand node selector expressions based on labels
	// * setup clusterwide lun mapping from pools backed by san arrays
	NodesInfo map[string]NodeInfo

	NodeInfo struct {
		Env    string    `json:"env"`
		Labels Labels    `json:"labels"`
		Paths  san.Paths `json:"paths"`
	}

	// Labels holds the key/value pairs defined in the labels section of the node.conf
	Labels map[string]string
)

func (t Labels) DeepCopy() Labels {
	labels := make(Labels)
	for k, v := range t {
		labels[k] = v
	}
	return labels
}

func cacheFile() string {
	return filepath.Join(rawconfig.Paths.Var, "nodes_info.json")
}

func cacheFilePair() (final, tmp string) {
	final = cacheFile()
	tmp = filepath.Join(filepath.Dir(final), "."+filepath.Base(final)+".swp")
	return
}

func Save(data NodesInfo) error {
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

func Load() (NodesInfo, error) {
	data := NodesInfo{}
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

func Req() (NodesInfo, error) {
	c, err := client.New()
	if err != nil {
		return nil, err
	}
	return ReqWithClient(c)
}

func ReqWithClient(c *client.T) (NodesInfo, error) {
	if c == nil {
		panic("nodesinfo.ReqWithClient(nil): no client")
	}
	resp, err := c.GetNodesInfo(context.Background())
	if err != nil {
		return nil, err
	}
	var data NodesInfo
	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(&data)
	return data, err
}

// Get returns the nodes info structure from the daemon api
// if available, and falls back to a file cache.
func Get() (NodesInfo, error) {
	var errs error
	if data, err := Req(); err == nil {
		return data, nil
	} else {
		errs = xerrors.Append(errs, err)
	}
	if data, err := Load(); err == nil {
		return data, nil
	} else {
		errs = xerrors.Append(errs, err)
	}
	return nil, errs
}

// GetNodesWithAnyPaths return the list of nodes having any of the given paths.
func (t NodesInfo) GetNodesWithAnyPaths(paths san.Paths) []string {
	l := make([]string, 0)
	for nodename, node := range t {
		if paths.HasAnyOf(node.Paths) {
			l = append(l, nodename)
		}
	}
	return l
}

package nodesinfo

import (
	"encoding/json"
	"os"
	"path/filepath"

	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/util/san"
	"opensvc.com/opensvc/util/xerrors"
)

type (
	// NodesInfo is the dataset exposed via the GET /nodes_info handler,
	// used by nodes to:
	// * expand node selector expressions based on labels
	// * setup clusterwide lun mapping from pools backed by san arrays
	NodesInfo map[string]NodeInfo

	NodeInfo struct {
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
	defer os.Remove(tmp)
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
	req := c.NewGetNodesInfo()
	b, err := req.Do()
	if err != nil {
		return nil, err
	}
	var data NodesInfo
	err = json.Unmarshal(b, &data)
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

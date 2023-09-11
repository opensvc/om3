package nodesinfo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/opensvc/om3/core/node"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/api"
)

func cacheFile() string {
	return filepath.Join(rawconfig.Paths.Var, "nodes_info.json")
}

func cacheFilePair() (final, tmp string) {
	final = cacheFile()
	tmp = filepath.Join(filepath.Dir(final), "."+filepath.Base(final)+".swp")
	return
}

func Save(data node.NodesInfo) error {
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

func Load() (node.NodesInfo, error) {
	data := node.NodesInfo{}
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

func ReqWithClient(c api.ClientInterface) (node.NodesInfo, error) {
	if c == nil {
		panic("nodesinfo.ReqWithClient(nil): no client")
	}
	resp, err := c.GetNodesInfo(context.Background())
	if err != nil {
		return nil, err
	} else if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected get nodes info status code %s", resp.Status)
	}
	var data node.NodesInfo
	defer resp.Body.Close()
	err = json.NewDecoder(resp.Body).Decode(&data)
	return data, err
}

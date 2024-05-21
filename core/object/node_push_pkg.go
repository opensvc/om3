package object

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/opensvc/om3/core/rawconfig"

	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/packages"
)

func (t Node) nodePackageCacheFile() string {
	return filepath.Join(rawconfig.NodeVarDir(), "package.json")
}

func (t Node) PushPkg() ([]packages.Pkg, error) {
	l, err := packages.List()
	if err != nil {
		return l, err
	}
	err = t.dumpPkg(l)
	if err != nil {
		return l, err
	}
	if err := t.pushPkg(l); err != nil {
		return l, err
	}
	return l, nil
}

func (t Node) dumpPkg(data []packages.Pkg) error {
	file, err := os.OpenFile(t.nodePackageCacheFile(), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0660)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	return json.NewEncoder(file).Encode(data)
}

func (t Node) LoadPkg() ([]packages.Pkg, error) {
	var data []packages.Pkg
	file, err := os.Open(t.nodePackageCacheFile())
	if err != nil {
		return data, err
	}
	defer func() { _ = file.Close() }()
	err = json.NewDecoder(file).Decode(&data)
	return data, err
}

func (t Node) pushPkg(data []packages.Pkg) error {
	url, err := t.Collector3RestAPIURL()
	if err != nil {
		return err
	}
	url.Path += "/oc3/feed/system"
	b, err := json.Marshal(map[string]any{"package": data})
	if err != nil {
		return fmt.Errorf("encode request body: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, url.String(), bytes.NewBuffer(b))
	req.SetBasicAuth(hostname.Hostname(), rawconfig.GetNodeSection().UUID)
	req.Header.Add("Content-Type", "application/json")
	c := t.CollectorRestAPIClient()
	response, err := c.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != 202 {
		return fmt.Errorf("unexpected %s %s response: %s", req.Method, req.URL, response.Status)
	}
	return nil
}

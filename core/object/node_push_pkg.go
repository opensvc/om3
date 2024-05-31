package object

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/opensvc/om3/core/rawconfig"

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
		return l, fmt.Errorf("push pkg: %w", err)
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
	oc3, err := t.CollectorClient()
	if err != nil {
		return err
	}

	b, err := json.Marshal(map[string]any{"package": data})
	if err != nil {
		return fmt.Errorf("encode request body: %w", err)
	}

	method := http.MethodPost
	path := "/oc3/feed/system"

	response, err := oc3.DoRequest(method, path, bytes.NewBuffer(b))
	if err != nil {
		return fmt.Errorf("collector %s %s: %w", method, path, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusAccepted {
		return fmt.Errorf("unexpected collector response status code for %s %s: wanted %d got %d",
			method, path, http.StatusAccepted, response.StatusCode)
	}
	return nil
}

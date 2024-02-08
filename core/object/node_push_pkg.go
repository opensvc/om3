package object

import (
	"encoding/json"
	"fmt"
	"github.com/opensvc/om3/core/rawconfig"
	"os"
	"path/filepath"

	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/packages"
)

var nodePkgCacheFile = filepath.Join(rawconfig.NodeVarDir(), "package.json")

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
	file, err := os.OpenFile(nodePkgCacheFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0660)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	return json.NewEncoder(file).Encode(data)
}

func (t Node) LoadPkg() ([]packages.Pkg, error) {
	var data []packages.Pkg
	file, err := os.Open(nodePkgCacheFile)
	if err != nil {
		return data, err
	}
	defer func() { _ = file.Close() }()
	err = json.NewDecoder(file).Decode(&data)
	return data, err
}

func (t Node) pushPkg(data []packages.Pkg) error {
	nodename := hostname.Hostname()
	pkgAsList := func(t packages.Pkg) []string {
		installedAt := ""
		if !t.InstalledAt.IsZero() {
			installedAt = t.InstalledAt.Format("2006-01-02 15:04:05")
		}
		return []string{
			nodename,
			t.Name,
			t.Version,
			t.Arch,
			t.Type,
			installedAt,
			t.Sig,
		}
	}
	pkgsAsList := func(t []packages.Pkg) [][]string {
		l := make([][]string, len(t))
		for i, p := range t {
			l[i] = pkgAsList(p)
		}
		return l
	}
	vars := []string{
		"pkg_nodename",
		"pkg_name",
		"pkg_version",
		"pkg_arch",
		"pkg_type",
		"pkg_install_date",
		"pkg_sig",
	}
	client, err := t.CollectorFeedClient()
	if err != nil {
		return err
	}
	if response, err := client.Call("insert_pkg", vars, pkgsAsList(data)); err != nil {
		return err
	} else if response.Error != nil {
		return fmt.Errorf("rpc: %s %s", response.Error.Message, response.Error.Data)
	}
	return nil
}

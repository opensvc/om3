package object

import (
	"github.com/pkg/errors"
	"opensvc.com/opensvc/util/hostname"
	"opensvc.com/opensvc/util/packages"
)

func (t Node) PushPkg() ([]packages.Pkg, error) {
	l, err := packages.List()
	if err != nil {
		return l, err
	}
	if err := t.pushPkg(l); err != nil {
		return l, err
	}
	return l, nil
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
	client, err := t.collectorFeedClient()
	if err != nil {
		return err
	}
	if response, err := client.Call("insert_pkg", vars, pkgsAsList(data)); err != nil {
		return err
	} else if response.Error != nil {
		return errors.Errorf("rpc: %s %s", response.Error.Message, response.Error.Data)
	}
	return nil
}

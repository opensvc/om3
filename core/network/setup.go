package network

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strings"

	"github.com/opensvc/om3/core/keyop"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/util/file"
)

const (
	// CNIVersion is the version we write in the generated CNI network configuration files
	CNIVersion = "0.3.0"
)

var (
	ErrInvalidType = errors.New("invalid network type")
)

func Setup(n *object.Node) error {
	errs := make([]error, 0)
	dir, err := mkCNIConfigDir(n)
	if err != nil {
		return err
	}
	cluster, err := object.NewCluster()
	if err != nil {
		return err
	}
	nws := Networks(n)
	needCommit := make([]string, 0)
	kops := make(keyop.L, 0)
	for _, nw := range nws {
		if err := checkOverlap(nw, nws); err != nil {
			nw.Log().Errorf("network setup: %s", err)
			errs = append(errs, err)
			continue
		}
		if err := setupNetwork(n, nw, dir); err != nil {
			nw.Log().Errorf("network setup: %s", err)
			errs = append(errs, err)
		}
		if nw.NeedCommit() {
			needCommit = append(needCommit, nw.Name())
			kops = append(kops, nw.Kops()...)
		}
	}
	if len(needCommit) > 0 {
		n.Log().Infof("network setup: commit config changes on %s", strings.Join(needCommit, ","))
		cluster.Config().Set(kops...)
	}
	if err := setupFW(n, nws); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return fmt.Errorf("network setup: %s", errs)
	}
	return nil
}

func intersect(n1, n2 *net.IPNet) bool {
	return n2.Contains(n1.IP) || n1.Contains(n2.IP)
}

func checkOverlap(nw Networker, nws []Networker) error {
	_, refIPNet, err := net.ParseCIDR(nw.Network())
	if err != nil {
		return nil
	}
	if prefix, err := netip.ParsePrefix(nw.Network()); err != nil {
		return err
	} else if prefix.Addr().String() != refIPNet.IP.String() {
		// ex: 172.10.10.0/22 prefix addr 172.10.10.0 does not match the prefix length (expected 172.10.8.0)
		return fmt.Errorf("%s prefix addr %s does not match the prefix length (expected %s)", prefix, prefix.Addr(), refIPNet.IP)
	}
	for _, other := range nws {
		if nw == other {
			continue
		}
		_, otherIPNet, err := net.ParseCIDR(other.Network())
		if err != nil {
			continue
		}
		if intersect(refIPNet, otherIPNet) {
			return fmt.Errorf("%s overlaps %s (%s)", refIPNet, otherIPNet, other.Name())
		}
	}
	return nil
}

func setupNetwork(n *object.Node, nw Networker, dir string) error {
	if IsDisabled(nw) {
		nw.Log().Tracef("network setup: skip disabled network")
		return nil
	}
	if !IsValid(nw) {
		nw.Log().Infof("network setup: skip invalid network")
		return nil
	}
	if i, ok := nw.(Setuper); ok {
		if err := i.Setup(); err != nil {
			return err
		}
	}
	if err := setupNetworkCNI(n, dir, nw); err != nil {
		return err
	}
	return nil
}

func mkCNIConfigDir(n *object.Node) (string, error) {
	dir, err := n.CNIConfig()
	if err != nil {
		return dir, err
	}
	if file.Exists(dir) {
		return dir, nil
	}
	if err := os.MkdirAll(dir, 0600); err != nil {
		return dir, err
	}
	return dir, nil
}

func CNIConfigFile(dir string, nw Networker) string {
	return filepath.Join(dir, nw.Name()+".conf")
}

func setupNetworkCNI(n *object.Node, dir string, nw Networker) error {
	i, ok := nw.(CNIer)
	if !ok {
		return nil
	}
	data, err := i.CNIConfigData()
	if err != nil {
		return err
	}
	p := CNIConfigFile(dir, nw)
	nw.Log().Infof("cni %s write", p)
	return writeCNIConfig(p, data)
}

func writeCNIConfig(fpath string, data interface{}) error {
	tmp, err := os.CreateTemp(filepath.Dir(fpath), "."+filepath.Base(fpath)+".")
	if err != nil {
		return err
	}
	if b, err := json.MarshalIndent(data, "", "   "); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return err
	} else if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return err
	} else {
		_ = tmp.Close()
		return os.Rename(tmp.Name(), fpath)
	}
}

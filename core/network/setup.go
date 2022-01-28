package network

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/util/file"
)

const (
	// The cniVersion key value in generated CNI network configuration files
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
	nws := Networks(n)
	needCommit := make([]string, 0)
	for _, nw := range nws {
		if err := checkOverlap(nw, nws); err != nil {
			nw.Log().Error().Err(err).Msgf("network setup")
			errs = append(errs, err)
			continue
		}
		if err := setupNetwork(n, nw, dir); err != nil {
			nw.Log().Error().Err(err).Msgf("network setup")
			errs = append(errs, err)
		}
		if nw.NeedCommit() {
			needCommit = append(needCommit, nw.Name())
		}
	}
	if len(needCommit) > 0 {
		n.Log().Info().Msgf("network setup: commit config changes on %s", strings.Join(needCommit, ","))
		n.MergedConfig().Commit()
	}
	if len(errs) > 0 {
		return fmt.Errorf("network setup: %d failed", len(errs))
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
	if !nw.IsValid() {
		nw.Log().Info().Msgf("network setup: skip invalid network")
		return nil
	}
	if err := SetupNetworkCNI(n, dir, nw); err != nil {
		return err
	}
	if i, ok := nw.(Setuper); ok {
		return i.Setup()
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

func SetupNetworkCNI(n *object.Node, dir string, nw Networker) error {
	i, ok := nw.(CNIer)
	if !ok {
		return nil
	}
	data, err := i.CNIConfigData()
	if err != nil {
		return err
	}
	p := CNIConfigFile(dir, nw)
	if file.Exists(p) {
		nw.Log().Info().Msgf("cni %s is already setup", p)
		return nil
	}
	nw.Log().Info().Msgf("cni %s write", p)
	return writeCNIConfig(p, data)
}

func writeCNIConfig(fpath string, data interface{}) error {
	if b, err := json.MarshalIndent(data, "", "   "); err != nil {
		return err
	} else {
		return ioutil.WriteFile(fpath, b, 0644)
	}
}

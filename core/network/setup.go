package network

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"

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
	dir, err := mkCNIConfigDir(n)
	if err != nil {
		return err
	}
	for _, nw := range Networks(n) {
		if err := setupNetwork(n, nw, dir); err != nil {
			return err
		}
	}
	return nil
}

func setupNetwork(n *object.Node, nw Networker, dir string) error {
	if !nw.IsValid() {
		n.Log().Info().
			Str("name", nw.Name()).
			Str("network", nw.Network()).
			Msgf("skip setup of invalid network")
		return nil
	}
	if err := SetupNetworkCNI(n, dir, nw); err != nil {
		return err
	}
	if i, ok := nw.(Setuper); ok {
		return i.Setup(n)
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
		n.Log().Info().Msgf("preserve %s", p)
		return nil
	}
	n.Log().Info().Msgf("create %s", p)
	return writeCNIConfig(p, data)
}

func writeCNIConfig(fpath string, data interface{}) error {
	if b, err := json.MarshalIndent(data, "", "   "); err != nil {
		return err
	} else {
		return ioutil.WriteFile(fpath, b, 0644)
	}
}

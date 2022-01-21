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
	dir, err := n.CNIConfig()
	if err != nil {
		return err
	}
	if !file.Exists(dir) {
		if err := os.MkdirAll(dir, 0600); err != nil {
			return err
		}
	}
	for _, nw := range Networks(n) {
		if !nw.IsValid() {
			n.Log().Info().
				Str("name", nw.Name()).
				Str("network", nw.Network()).
				Msgf("skip setup of invalid network")
			continue
		}
		if err := Create(n, nw, dir); err != nil {
			return err
		}
	}
	return nil
}

func Create(n *object.Node, nw Networker, dir string) error {
	p := filepath.Join(dir, nw.Name()+".conf")
	if file.Exists(p) {
		n.Log().Info().Msgf("preserve %s", p)
		return nil
	}
	n.Log().Info().Msgf("create %s", p)
	if data, err := nw.CNIConfigData(); err != nil {
		return err
	} else if b, err := json.MarshalIndent(data, "", "   "); err != nil {
		return err
	} else {
		return ioutil.WriteFile(p, b, 0644)
	}

}

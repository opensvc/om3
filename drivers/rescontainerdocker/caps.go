package rescontainerdocker

import (
	"bytes"
	"encoding/json"
	"os/exec"

	"github.com/hashicorp/go-version"
	"github.com/opensvc/om3/v3/util/capabilities"
)

type (
	dockerInfo struct {
		ClientInfo struct {
			Version string
		}
	}
)

var (
	drvCap            = DrvID.Cap()
	capRegistryCreds  = drvCap + ".registry_creds"
	capSignal         = drvCap + ".signal"
	capHasTimeoutFlag = drvCap + ".has_timeout_flag"
)

func init() {
	capabilities.Register(capabilitiesScanner)
}

func getDockerInfo() (*dockerInfo, error) {
	var di dockerInfo
	b, err := exec.Command("docker", "info", "-f", "json").Output()
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(b, &di)
	return &di, err
}

func IsGenuine() bool {
	b, err := exec.Command("docker", "--version").Output()
	if err != nil {
		return false
	} else if bytes.Contains(b, []byte("Docker")) {
		return true
	}
	return false
}

func isTimeoutCapable(di *dockerInfo) bool {
	if di == nil {
		return false
	}
	v, err := version.NewVersion(di.ClientInfo.Version)
	if err != nil {
		return false
	}
	constraints, err := version.NewConstraint(">= 22")
	if err != nil {
		return false
	}
	if constraints.Check(v) {
		return true
	}
	return false
}

func capabilitiesScanner() ([]string, error) {
	l := make([]string, 0)
	if !IsGenuine() {
		return l, nil
	}
	l = append(l, drvCap)
	l = append(l, capRegistryCreds)
	l = append(l, capSignal)

	di, err := getDockerInfo()
	if err == nil {
		if isTimeoutCapable(di) {
			l = append(l, capHasTimeoutFlag)
		}
	}

	return l, nil
}

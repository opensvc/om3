package scsi

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/hashicorp/go-version"

	"github.com/opensvc/om3/v3/util/capabilities"
	"github.com/opensvc/om3/v3/util/command"
)

const (
	MpathPersistCapability = "node.x.scsi.mpathpersist"
	SGPersistCapability    = "node.x.scsi.sg_persist"
)

var (
	mpathReservationKeyFileRegexp = regexp.MustCompile(`(?m)^\s*reservation_key\s+("file"|file)\s*$`)
)

// capabilitiesScanner is the capabilities scanner for scsi
func capabilitiesScanner(ctx context.Context) ([]string, error) {
	l := make([]string, 0)
	if _, err := exec.LookPath("mpathpersist"); err != nil {
		// out
	} else if mpathReservationKeyConfigured, err := isMpathReservationKeyConfigured(); err != nil {
		// out
	} else if !mpathReservationKeyConfigured {
		// out
	} else if mpathVersionSufficent, err := isMpathVersionSufficent(ctx); err != nil {
		// out
	} else if mpathVersionSufficent {
		l = append(l, MpathPersistCapability)
	}
	if _, err := exec.LookPath("sg_persist"); err != nil {
		// pass
	} else {
		l = append(l, SGPersistCapability)
	}
	return l, nil
}

func isMpathVersionSufficent(ctx context.Context) (bool, error) {
	v, err := mpathVersion(ctx)
	if err != nil {
		return false, err
	}
	minVer, err := version.NewVersion("0.7.8")
	if err != nil {
		return false, err
	}
	curVer, err := version.NewVersion(v)
	if err != nil {
		return false, err
	}
	return minVer.LessThanOrEqual(curVer), nil
}

func mpathVersion(ctx context.Context) (string, error) {
	cmd := command.New(
		command.WithContext(ctx),
		command.WithName("multipath"),
		command.WithVarArgs("-h"),
		command.WithBufferedStderr(),
	)
	if err := cmd.Run(); err != nil {
		return "", err
	}
	b := cmd.Stderr()
	scanner := bufio.NewScanner(bytes.NewReader(b))
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "multipath-tools v") {
			words := strings.Fields(line)
			return words[1][1:], nil
		}
	}
	return "", fmt.Errorf("multipath tools version not found")
}

func isMpathReservationKeyConfigured() (bool, error) {
	b, err := os.ReadFile("/etc/multipath.conf")
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return mpathReservationKeyFileRegexp.Match(b), nil
}

// register node scanners
func init() {
	capabilities.Register(capabilitiesScanner)
}

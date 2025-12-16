package ressyncrsync

import (
	"bytes"
	"context"
	"os/exec"

	"github.com/opensvc/om3/v3/util/capabilities"
)

func init() {
	capabilities.Register(capabilitiesScanner)
}

func capabilitiesScanner(ctx context.Context) ([]string, error) {
	l := make([]string, 0)
	bin, err := exec.LookPath(rsync)
	if err != nil {
		return l, err
	}
	cmd := exec.CommandContext(ctx, bin, "--version")
	b, err := cmd.Output()
	if err != nil {
		return l, err
	}
	baseCap := drvID.Cap()
	l = append(l, baseCap)
	if !bytes.Contains(b, []byte("no xattrs")) {
		l = append(l, baseCap+".xattrs")
	}
	if !bytes.Contains(b, []byte("no ACLs")) {
		l = append(l, baseCap+".acls")
	}
	return l, nil
}

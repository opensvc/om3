package rescontainervbox

import (
	"bufio"
	"context"
	"io"
	"strings"

	"github.com/opensvc/om3/v3/core/resource"
)

// Info implements the resource infoer: the uuid of the virtual machine,
// known while it is powered off. The guest sees it as its SMBIOS system
// uuid, and the collector links the guest node to this resource by it.
//
// A virtual machine not registered adds no key.
func (t *T) Info(ctx context.Context) (resource.InfoKeys, error) {
	keys := resource.InfoKeys{}
	s, err := t.vBoxManageCommand("showvminfo", "--machinereadable", t.Name)
	if err != nil {
		t.Log().Debugf("info: %s", err)
		return keys, nil
	}
	if u := vmUUIDFromReader(strings.NewReader(s)); u != "" {
		keys = append(keys, resource.InfoKey{Key: "uuid", Value: u})
	}
	return keys, nil
}

// vmUUIDFromReader returns the UUID line of a showvminfo --machinereadable
// output, lowercased, empty when there is none.
func vmUUIDFromReader(r io.Reader) string {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		if v, ok := strings.CutPrefix(scanner.Text(), "UUID="); ok {
			return strings.ToLower(strings.Trim(v, "\""))
		}
	}
	return ""
}

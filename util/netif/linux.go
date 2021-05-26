// +build linux

package netif

import (
	"fmt"
	"strings"

	"opensvc.com/opensvc/util/file"
)

func HasCarrier(name string) (bool, error) {
	p := fmt.Sprintf("/sys/class/net/%s/carrier", name)
	b, err := file.ReadAll(p)
	if err != nil {
		return false, err
	}
	return strings.TrimSuffix(string(b), "\n") == "1", nil
}

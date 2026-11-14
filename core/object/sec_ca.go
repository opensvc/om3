package object

import (
	"fmt"

	"github.com/opensvc/om3/v3/core/naming"
)

// ClusterCAPrivateKey returns the private key of the cluster authority,
// system/sec/ca, in PEM, read from its configuration. The api tokens and
// the kvm vsock tokens are signed with it.
//
// It is read where it signs, and never written out of the configuration of
// the sec: a copy in a file is one more path for it to leak by.
func ClusterCAPrivateKey() ([]byte, error) {
	sec, err := NewSec(naming.SecCa, WithVolatile(true))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", naming.SecCa, err)
	}
	b, err := sec.DecodeKey("private_key")
	if err != nil {
		return nil, fmt.Errorf("%s private_key: %w", naming.SecCa, err)
	}
	return b, nil
}

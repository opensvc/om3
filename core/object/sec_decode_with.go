package object

import (
	"github.com/opensvc/om3/v3/core/omcrypto"
	"github.com/opensvc/om3/v3/util/hostname"
)

// DecodeSecValueWith decodes the value of a sec key as written in its
// configuration file, crypt:<data>, with the secret of the cluster named
// clusterName rather than the one of the cluster of this node: a node
// joining a cluster reads the keys of the sec it fetched from it before it
// installs its configuration.
func DecodeSecValueWith(value, clusterName, secret string) ([]byte, error) {
	d := &secEncodeDecode{encryptDecrypter: &omcrypto.T{
		NodeName:    hostname.Hostname(),
		ClusterName: clusterName,
		Key:         secret,
	}}
	return d.Decode(value)
}

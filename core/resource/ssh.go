package resource

import (
	"golang.org/x/crypto/ssh"

	"github.com/opensvc/om3/core/cluster"
	"github.com/opensvc/om3/util/funcopt"
	"github.com/opensvc/om3/util/sshnode"
)

// SSH represents a configuration for SSH-based interactions,
// including the private key file used for authentication.
type SSH struct {
	// sshKeyFile specifies the file path to the SSH private key used for
	// authentication with NewSSHClient, when its value is not zero
	sshKeyFile string
}

// SetSSHKeyFile implements SetSSHKeyFiler
func (t *SSH) SetSSHKeyFile() {
	if keyFile, ok := cluster.ConfigData.Get().SSHKeyFile(); ok {
		t.sshKeyFile = keyFile
	}
}

func (t *SSH) GetSSHKeyFile() string {
	return t.sshKeyFile
}

func (t *SSH) NewSSHClient(nodename string, opts ...funcopt.O) (*ssh.Client, error) {
	if t.sshKeyFile != "" {
		opts = append(opts, sshnode.WithPrivateKeyFiles(t.sshKeyFile))
	}
	return sshnode.NewClient(nodename, opts...)
}

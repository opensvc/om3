package zfs

import (
	"golang.org/x/crypto/ssh"

	"github.com/opensvc/om3/util/funcopt"
	"github.com/opensvc/om3/util/sshnode"
)

func (t *Filesystem) newSSHClient(nodename string, opts ...funcopt.O) (*ssh.Client, error) {
	if t.SSHKeyFile != "" {
		opts = append(opts, sshnode.WithPrivateKeyFiles(t.SSHKeyFile))
	}
	return sshnode.NewClient(nodename, opts...)
}

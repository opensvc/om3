package oxcmd

import (
	"github.com/opensvc/om3/v3/core/clientcontext"
)

type (
	ContextClusterAddCmd struct {
		Name                 string
		Server               string
		Insecure             bool
		CertificateAuthority string
	}
)

func (t *ContextClusterAddCmd) Run() error {
	cluster := clientcontext.Cluster{
		Server: t.Server,
	}
	if t.Insecure {
		cluster.InsecureSkipVerify = &t.Insecure
	}
	if t.CertificateAuthority != "" {
		cluster.CertificateAuthority = &t.CertificateAuthority
	}

	config, err := clientcontext.Load()
	if err != nil {
		return err
	}

	if err := config.AddCluster(t.Name, cluster); err != nil {
		return err
	}

	return config.Save()
}

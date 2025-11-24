package oxcmd

import (
	"fmt"

	"github.com/opensvc/om3/core/clientcontext"
)

type (
	ContextClusterChangeCmd struct {
		Name                 string
		Server               string
		Insecure             bool
		CertificateAuthority string
	}
)

func (t *ContextClusterChangeCmd) Run() error {
	configs, err := clientcontext.Load()
	if err != nil {
		return err
	}

	cluster, ok := configs.Clusters[t.Name]
	if !ok {
		return fmt.Errorf("cluster %s does not exist", t.Name)
	}

	cluster.Server = t.Server
	if t.Insecure {
		cluster.InsecureSkipVerify = &t.Insecure
	}
	if t.CertificateAuthority != "" {
		cluster.CertificateAuthority = &t.CertificateAuthority
	}

	if err := configs.ChangeCluster(t.Name, cluster); err != nil {
		return err
	}

	return configs.Save()
}

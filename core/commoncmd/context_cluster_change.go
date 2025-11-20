package commoncmd

import (
	"fmt"

	"github.com/spf13/cobra"

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

func NewCmdContextClusterChange() *cobra.Command {
	var options ContextClusterChangeCmd

	cmd := &cobra.Command{
		Use:   "change",
		Short: "Change an existing cluster in the context",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&options.Name, "name", "", "Cluster name")
	flags.StringVar(&options.Server, "server", "", "Cluster server address")
	flags.BoolVar(&options.Insecure, "insecure", false, "Skip TLS certificate verification")
	flags.StringVar(&options.CertificateAuthority, "certificate-authority", "", "Path to the certificate authority file")

	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("server")

	return cmd
}

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
		cluster.InsecureSkipVerify = true
	} else {
		cluster.InsecureSkipVerify = false
	}
	if t.CertificateAuthority != "" {
		cluster.CertificateAuthority = t.CertificateAuthority
	}

	if err := configs.ChangeCluster(t.Name, cluster); err != nil {
		return err
	}

	return configs.Save()
}

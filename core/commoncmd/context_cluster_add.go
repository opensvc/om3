package commoncmd

import (
	"github.com/spf13/cobra"

	"github.com/opensvc/om3/core/clientcontext"
)

type (
	ContextClusterAddCmd struct {
		Name                 string
		Server               string
		Insecure             bool
		CertificateAuthority string
	}
)

func NewCmdContextClusterAdd() *cobra.Command {
	var options ContextClusterAddCmd

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a new cluster to the context",
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

package commoncmd

import (
	"github.com/spf13/cobra"

	"github.com/opensvc/om3/core/clientcontext"
)

type (
	ContextUserAddCmd struct {
		Name              string
		Username          string
		ClientKey         string
		ClientCertificate string
	}
)

func NewCmdContextUserAdd() *cobra.Command {
	var options ContextUserAddCmd

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a user to the context",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}

	flags := cmd.Flags()

	flags.StringVar(&options.Name, "name", "", "Name of the user element")
	flags.StringVar(&options.Username, "username", "", "Username associated with the user")
	flags.StringVar(&options.ClientKey, "client-key", "", "Path to the client key file")
	flags.StringVar(&options.ClientCertificate, "client-certificate", "", "Path to the client certificate file")

	_ = cmd.MarkFlagRequired("name")

	return cmd
}

func (t *ContextUserAddCmd) Run() error {
	user := clientcontext.User{}

	if t.Username != "" {
		user.Name = t.Username
	} else {
		user.Name = t.Name
	}

	if t.ClientKey != "" {
		user.ClientKey = t.ClientKey
	}

	if t.ClientCertificate != "" {
		user.ClientCertificate = t.ClientCertificate
	}

	cfg, err := clientcontext.Load()
	if err != nil {
		return err
	}

	if err := cfg.AddUser(t.Name, user); err != nil {
		return err
	}

	return cfg.Save()
}

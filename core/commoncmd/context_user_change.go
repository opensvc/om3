package commoncmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/opensvc/om3/core/clientcontext"
)

type (
	ContextUserChangeCmd struct {
		Name              string
		Username          string
		ClientKey         string
		ClientCertificate string
	}
)

func NewCmdContextUserChange() *cobra.Command {
	var options ContextUserChangeCmd

	cmd := &cobra.Command{
		Use:   "change",
		Short: "Change a user in the context",
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

func (t *ContextUserChangeCmd) Run() error {
	configs, err := clientcontext.Load()
	if err != nil {
		return err
	}

	usr, ok := configs.Users[t.Name]
	if !ok {
		return fmt.Errorf("user %s does not exist", t.Name)
	}

	if t.Username != "" {
		usr.Name = t.Username
	} else {
		usr.Name = t.Name
	}

	if t.ClientKey != "" {
		usr.ClientKey = t.ClientKey
	}

	if t.ClientCertificate != "" {
		usr.ClientCertificate = t.ClientCertificate
	}

	if err := configs.ChangeUser(t.Name, usr); err != nil {
		return err
	}

	return configs.Save()
}

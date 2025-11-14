package commoncmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	reqtoken "github.com/opensvc/om3/core/client/token"
	"github.com/opensvc/om3/core/env"
)

type (
	CmdDaemonLogout struct {
		Context string
	}
)

func NewCmdDaemonLogout() *cobra.Command {
	var options CmdDaemonLogout
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Clear cached authentication tokens",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&options.Context, "context", "", "The context to use to logout")
	return cmd
}

func (t *CmdDaemonLogout) Run() error {
	if t.Context == "" {
		if ctx := env.Context(); ctx != "" {
			t.Context = ctx
		} else {
			var valid []string
			tokens := reqtoken.GetAll()
			for name := range tokens {
				tok := tokens[name]
				if time.Now().Before(tok.RefreshTokenExpire) {
					valid = append(valid, name)
				}
			}
			if len(valid) == 0 {
				return fmt.Errorf("no valid context login found")
			}
			fmt.Println("Current valid context logins:")
			for _, name := range valid {
				fmt.Println(" - " + name)
			}
			fmt.Println()
			name, _ := reqtoken.GetLast()
			fmt.Print("Select context")
			if name != "" {
				fmt.Printf(" [<%s>]", name)
			}
			fmt.Print(": ")
			reader := bufio.NewReader(os.Stdin)
			input, err := reader.ReadString('\n')
			if err != nil && err != io.EOF {
				return err
			}
			t.Context = strings.TrimSpace(input)
		}
	}

	if !reqtoken.Exists(t.Context) {
		return fmt.Errorf("no token found for context %s", t.Context)
	}

	if err := reqtoken.Delete(t.Context); err != nil {
		return err
	}

	return nil
}

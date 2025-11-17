package commoncmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/opensvc/om3/core/client/tokencache"
	"github.com/opensvc/om3/core/env"
)

type (
	CmdContextLogout struct {
		Context string
	}
)

func NewCmdDaemonLogout() *cobra.Command {
	var options CmdContextLogout
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

func (t *CmdContextLogout) Run() error {
	if t.Context == "" {
		if ctx := env.Context(); ctx != "" {
			t.Context = ctx
		} else {
			tokens := tokencache.GetAll()
			for name := range tokens {
				tok := tokens[name]
				if !time.Now().Before(tok.RefreshTokenExpire) {
					delete(tokens, name)
				}
			}
			if len(tokens) == 0 {
				return fmt.Errorf("no valid context login found")
			}
			fmt.Println("Current valid context logins:")
			for name := range tokens {
				fmt.Println(" - " + name)
			}
			fmt.Println()
			name, _ := tokencache.GetLast()
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
			if input == "\n" && name != "" {
				t.Context = name
			} else if input == "\n" {
				return fmt.Errorf("no context selected")
			} else {
				t.Context = strings.TrimSpace(input)
			}
		}
	}

	if !tokencache.Exists(t.Context) {
		return fmt.Errorf("no tokencache found for context %s", t.Context)
	}

	if err := tokencache.Delete(t.Context); err != nil {
		return err
	}

	return nil
}

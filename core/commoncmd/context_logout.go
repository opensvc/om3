package commoncmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/opensvc/om3/core/client/tokencache"
	"github.com/opensvc/om3/core/env"
)

type (
	CmdContextLogout struct {
		Context string
		All     bool
	}
)

func NewCmdContextLogout() *cobra.Command {
	var options CmdContextLogout
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Clear cached authentication tokens",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run(cmd)
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&options.Context, "context", "", "The context to use to logout")
	flags.BoolVar(&options.All, "all", false, "Logout from all contexts")
	return cmd
}

func (t *CmdContextLogout) Run(cmd *cobra.Command) error {

	contextChanged := cmd.Flag("context").Changed

	if t.Context == "" && !t.All {
		if ctx := env.Context(); ctx != "" && !contextChanged {
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
			i := 0
			contextName := make([]string, len(tokens))
			lastName, _ := tokencache.GetLast()
			lastIndex := -1
			for name := range tokens {
				contextName[i] = name
				i++
			}
			slices.Sort(contextName)

			for i, name := range contextName {
				fmt.Printf("%d) %s\n", i+1, name)
				if name == lastName {
					lastIndex = i
				}
			}

			fmt.Println()
			fmt.Print("Select context")
			if lastName != "" && lastIndex != -1 {
				fmt.Printf(" [%d]", lastIndex+1)
			}
			fmt.Print(": ")
			reader := bufio.NewReader(os.Stdin)
			input, err := reader.ReadString('\n')
			if err != nil && err != io.EOF {
				return err
			}
			if input == "\n" && lastIndex != -1 {
				t.Context = lastName
			} else if input == "\n" {
				return fmt.Errorf("no context selected")
			} else {
				inputTrimmed := strings.TrimSpace(input)
				if idx, err := strconv.Atoi(inputTrimmed); err == nil {
					if idx < 1 || idx > len(contextName) {
						return fmt.Errorf("invalid context index")
					}
					t.Context = contextName[idx-1]
				} else {
					return fmt.Errorf("invalid context selection : must be a number")
				}
			}
		}
	}

	if !t.All {
		if !tokencache.Exists(t.Context) {
			return fmt.Errorf("no tokencache found for context %s", t.Context)
		}

		if err := tokencache.Delete(t.Context); err != nil {
			return err
		}
		return nil
	}

	tokens := tokencache.GetAll()
	for name := range tokens {
		if err := tokencache.Delete(name); err != nil {
			return err
		}
	}

	return nil
}

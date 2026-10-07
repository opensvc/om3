package oxcmd

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

	"github.com/opensvc/om3/v3/core/client/tokencache"
	"github.com/opensvc/om3/v3/core/env"
)

type (
	CmdContextLogout struct {
		Context string
		All     bool
	}
)

func (t *CmdContextLogout) Run(cmd *cobra.Command) error {

	contextChanged := cmd.Flag("context").Changed
	if !contextChanged {
		t.Context = env.Context()
	}

	if t.Context == "" && !t.All {
		tokens := tokencache.GetAll()
		if len(tokens) == 0 {
			return fmt.Errorf("no context login found")
		}

		fmt.Println("Current context logins:")
		contextName := make([]string, 0, len(tokens))
		lastName, _ := tokencache.GetLast()
		lastIndex := -1
		for name := range tokens {
			contextName = append(contextName, name)
		}
		slices.Sort(contextName)

		now := time.Now()
		for i, name := range contextName {
			// The expired logins are listed too: their token files are
			// still to delete. The mark is only shown, the selection
			// names the context.
			label := name
			if !tokens[name].HasValidRefresh(now) {
				label += " (expired)"
			}
			fmt.Printf("%d) %s\n", i+1, label)
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

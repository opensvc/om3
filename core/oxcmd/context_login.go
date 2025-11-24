package oxcmd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/client/tokencache"
	"github.com/opensvc/om3/core/clientcontext"
	"github.com/opensvc/om3/core/env"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/util/duration"
)

type (
	CmdContextLogin struct {
		Context         string
		AccessDuration  time.Duration
		RefreshDuration time.Duration
	}
)

func (t *CmdContextLogin) Run(cmd *cobra.Command) error {
	config, err := clientcontext.Load()
	if err != nil {
		return err
	}

	contextChanged := cmd.Flag("context").Changed
	if !contextChanged {
		t.Context = env.Context()
	}

	if t.Context == "" {
		fmt.Println("Known Contexts:")
		i := 0
		contextName := make([]string, len(config.Contexts))
		lastName, _ := tokencache.GetLast()
		lastIndex := -1
		for name := range config.Contexts {
			contextName[i] = name
			i++
		}
		slices.Sort(contextName)

		tokens := tokencache.GetAll()
		for i, name := range contextName {
			if tok, ok := tokens[name]; ok && !time.Now().Before(tok.RefreshTokenExpire) {
				name = name + " (expired)"
			}
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
		if input == "\n" && lastName != "" {
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

	fmt.Printf("Password for %s: ", t.Context)
	pwdBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return err
	}
	fmt.Println()
	password := string(pwdBytes)
	if password == "" {
		return fmt.Errorf("empty password")
	}

	os.Setenv("OSVC_CONTEXT", t.Context)

	clientc, ok := config.Contexts[t.Context]
	if !ok {
		return fmt.Errorf("context %s not found in config", t.Context)
	}

	userName := clientc.UserRefName
	if usr, ok := config.Users[clientc.UserRefName]; ok && usr.Name != nil && *usr.Name != "" {
		userName = *usr.Name
	}
	c, err := client.New(client.WithUsername(userName), client.WithPassword(password))
	if err != nil {
		return err
	}

	params := api.PostAuthTokenParams{}
	refresh := true
	params.Refresh = &refresh

	if v := chooseDuration(duration.New(t.RefreshDuration), config.Contexts[t.Context].RefreshTokenDuration); v.Positive() {
		refreshDurationStr := v.String()
		params.RefreshDuration = &refreshDurationStr
	}

	accessDuration := chooseDuration(duration.New(t.AccessDuration), config.Contexts[t.Context].AccessTokenDuration)
	if accessDuration.Positive() {
		accessDurationStr := accessDuration.String()
		params.AccessDuration = &accessDurationStr
	}

	resp, err := c.PostAuthTokenWithResponse(context.Background(), &params)

	if err != nil {
		return err
	}

	if resp.StatusCode() != http.StatusOK {
		switch resp.StatusCode() {
		case 400:
			return fmt.Errorf("%s", resp.JSON400)
		case 401:
			return fmt.Errorf("%s", resp.JSON401)
		case 403:
			return fmt.Errorf("%s", resp.JSON403)
		case 404:
			return fmt.Errorf("%s", resp.JSON404)
		case 500:
			return fmt.Errorf("%s", resp.JSON500)
		case 503:
			return fmt.Errorf("%s", resp.JSON503)
		default:
			return fmt.Errorf("unexpected status code: %d", resp.StatusCode())
		}
	}

	resp200 := resp.JSON200
	token := tokencache.Entry{
		AccessTokenExpire:  resp200.AccessExpiredAt,
		AccessToken:        resp200.AccessToken,
		RefreshTokenExpire: *resp200.RefreshExpiredAt,
		RefreshToken:       *resp200.RefreshToken,
	}
	if accessDuration.Positive() {
		token.AccessTokenDuration = accessDuration
	}

	err = tokencache.Save(t.Context, token)
	if err != nil {
		return err
	}

	fmt.Printf("Login successful. Switch to this context with :\nexport OSVC_CONTEXT=%s\n", t.Context)
	return nil
}

func chooseDuration(first *duration.Duration, second *duration.Duration) *duration.Duration {
	if first != nil && first.Positive() {
		return first
	}
	if second != nil && second.Positive() {
		return second
	}
	return duration.New(0)
}

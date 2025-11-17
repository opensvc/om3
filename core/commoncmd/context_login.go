package commoncmd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/client/tokencache"
	"github.com/opensvc/om3/core/clientcontext"
	"github.com/opensvc/om3/core/env"
	"github.com/opensvc/om3/daemon/api"
)

type (
	CmdContextLogin struct {
		Context         string
		AccessDuration  time.Duration
		RefreshDuration time.Duration
	}
)

func NewCmdDaemonLogin() *cobra.Command {
	var options CmdContextLogin
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Request and cache authentication tokens",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&options.Context, "context", "", "The context to use to login")
	flags.DurationVar(&options.RefreshDuration, "refresh-duration", 0, "refresh_token duration.")
	flags.DurationVar(&options.AccessDuration, "duration", 0, "access_token duration.")

	return cmd
}

func (t *CmdContextLogin) Run() error {

	if t.Context == "" {
		if ctx := env.Context(); ctx != "" {
			t.Context = ctx
		} else {
			config, err := clientcontext.Load()
			var firstContext string
			if err != nil {
				return err
			}
			fmt.Println("Known Contexts:")
			for name := range config.Contexts {
				fmt.Println(" - " + name)
				if firstContext == "" {
					firstContext = name
				}
			}
			fmt.Println()
			name, _ := tokencache.GetLast()
			if name == "" {
				name = firstContext
			}
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

	fmt.Print("Password: ")
	pwdBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return err
	}
	fmt.Println()
	password := string(pwdBytes)

	os.Setenv("OSVC_CONTEXT", t.Context)

	clientc, err := clientcontext.New()
	if err != nil {
		return err
	}

	c, err := client.New(client.WithUsername(clientc.User.Name), client.WithPassword(password))
	if err != nil {
		return err
	}
	refresh := true
	refreshDuration := t.RefreshDuration.String()
	accessDuration := t.AccessDuration.String()
	params := api.PostAuthTokenParams{}
	params.Refresh = &refresh
	if t.RefreshDuration != 0 {
		params.RefreshDuration = &refreshDuration
	}
	if t.AccessDuration != 0 {
		params.AccessDuration = &accessDuration
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
	err = tokencache.Save(t.Context, token)
	if err != nil {
		return err
	}

	return nil
}

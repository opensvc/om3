package commoncmd

import (
	"cmp"
	"slices"
	"time"

	"github.com/spf13/cobra"

	"github.com/opensvc/om3/core/client/tokencache"
	"github.com/opensvc/om3/core/clientcontext"
	"github.com/opensvc/om3/core/output"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/util/unstructured"
)

type (
	CmdContextList struct {
		OptsGlobal
	}
)

func NewCmdContextList() *cobra.Command {
	var options CmdContextList
	cmd := &cobra.Command{
		Use:   "list",
		Short: "list configured contexts",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	FlagColor(flags, &options.Color)
	FlagOutput(flags, &options.Output)

	return cmd
}

func (t *CmdContextList) Run() error {
	cols := "NAME:name,AUTHENTICATED:authenticated,ACCESS_EXPIRE:access_expired_at,REFRESH_EXPIRE:refresh_expired_at,AUTHENTICATED_AT:authenticated_at"

	config, err := clientcontext.Load()
	if err != nil {
		return err
	}
	lines := make([]clientcontext.TokenInfo, 0, len(config.Contexts))
	for name, _ := range config.Contexts {
		tok, err := tokencache.Load(name)
		if err != nil {
			return tokencache.ReconnectError(err, name)
		}

		info := clientcontext.TokenInfo{
			Name: name,
		}
		if tok == nil {
			info.AccessExpireAt = "-"
			info.RefreshExpireAt = "-"
			info.AuthenticatedAt = "-"
		} else {
			info.AccessExpireAt = tok.AccessTokenExpire.Format(time.RFC3339)
			info.RefreshExpireAt = tok.RefreshTokenExpire.Format(time.RFC3339)
			info.Authenticated = time.Now().Before(tok.RefreshTokenExpire)
			modTime, err := tokencache.ModTime(name)
			if err != nil {
				return err
			}
			info.AuthenticatedAt = modTime.Format(time.RFC3339)
		}
		lines = append(lines, info)
	}

	slices.SortFunc(lines, func(a, b clientcontext.TokenInfo) int {
		return cmp.Compare(a.Name, b.Name)
	})

	render := func(items []clientcontext.TokenInfo) {
		lines := make(unstructured.List, len(items))
		for i, item := range items {
			u := item.Unstructured()
			lines[i] = u
		}
		output.Renderer{
			DefaultOutput: "tab=" + cols,
			Output:        t.Output,
			Color:         t.Color,
			Data:          lines,
			Colorize:      rawconfig.Colorize,
		}.Print()
	}

	render(lines)
	return nil
}

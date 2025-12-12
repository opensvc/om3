package oxcmd

import (
	"cmp"
	"slices"
	"time"

	"github.com/opensvc/om3/v3/core/client/tokencache"
	"github.com/opensvc/om3/v3/core/clientcontext"
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/util/unstructured"
)

type (
	CmdContextList struct {
		commoncmd.OptsGlobal
	}
)

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
			tok = nil
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

package daemonapi

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/clusterdump"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/xconfig"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/daemon/rbac"
	"github.com/opensvc/om3/daemon/relay"
	"github.com/opensvc/om3/util/key"
)

func (a *DaemonAPI) GetRelayStatus(ctx echo.Context, params api.GetRelayStatusParams) error {
	if v, err := assertGrant(ctx, rbac.GrantHeartbeat, rbac.GrantRoot); !v {
		return err
	}
	if params.Remote != nil && *params.Remote {
		return a.getRelayStatusRemote(ctx, params)
	} else {
		return a.getRelayStatusLocal(ctx, params)
	}
}

func (a *DaemonAPI) getRelayStatusLocal(ctx echo.Context, params api.GetRelayStatusParams) error {
	data := api.RelayStatusList{
		Kind:  "RelayStatusList",
		Items: make(api.RelayStatusItems, 0),
	}
	var slots []relay.Slot
	if grantsFromContext(ctx).HasGrant(rbac.GrantRoot) {
		// root is allowed to read all user relay slots
		slots = relay.Map.List("")
	} else {
		// non-root is allowed to read its own user relay slots
		username := userFromContext(ctx).GetUserName()
		slots = relay.Map.List(username)
	}
	for _, slot := range slots {
		v := slot.Value.(api.RelayMessage)
		item := api.RelayStatusItem{
			ClusterID:   v.ClusterID,
			ClusterName: v.ClusterName,
			MsgLen:      len(v.Msg),
			NodeAddr:    v.NodeAddr,
			Nodename:    v.Nodename,
			Relay:       a.localhost,
			Status:      "",
			UpdatedAt:   v.UpdatedAt,
			Username:    v.Username,
		}
		data.Items = append(data.Items, item)
	}
	return ctx.JSON(http.StatusOK, data)
}

func (a *DaemonAPI) getRelayStatusRemote(ctx echo.Context, params api.GetRelayStatusParams) error {
	falseValue := false
	items := make(api.RelayStatusItems, 0)
	relayMap := make(map[string]any)
	if params.Relays != nil {
		for _, s := range *params.Relays {
			relayMap[s] = nil
		}
	}
	node, err := object.NewNode()
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "new node", "%s", err)
	}
	config := node.MergedConfig()
	for _, section := range config.SectionStrings() {
		if !strings.HasPrefix(section, "hb#") {
			continue
		}
		hbType := config.Get(key.New(section, "type"))
		if hbType != "relay" {
			continue
		}
		hbRelay := config.GetString(key.New(section, "relay"))
		if len(relayMap) > 0 {
			// some relay filtering is on
			if _, ok := relayMap[hbRelay]; !ok {
				// filtered out
				continue
			}
		}
		insecure := config.GetBool(key.New(section, "insecure"))
		username := config.GetString(key.New(section, "username"))
		password, err := configSectionPassword(config, section)
		if err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "configSectionPassword", "%s: %s", hbRelay, err)
		}
		cli, err := client.New(
			client.WithURL(hbRelay),
			client.WithUsername(username),
			client.WithPassword(password),
			client.WithInsecureSkipVerify(insecure),
		)
		if err != nil {
			return JSONProblemf(ctx, http.StatusInternalServerError, "new client", "%s: %s", hbRelay, err)
		}
		params.Remote = &falseValue
		resp, err := cli.GetRelayStatusWithResponse(context.Background(), &params)
		if err != nil {
			// add a placeholder data, so the user can see something went wrong
			clusterConfigData := clusterdump.ConfigData.Get()
			items = append(items, api.RelayStatusItem{
				ClusterID:   clusterConfigData.ID,
				ClusterName: clusterConfigData.Name,
				Relay:       hbRelay,
				Username:    username,
				Status:      fmt.Sprint(err),
			})
		} else if resp.StatusCode() != http.StatusOK {
			// add a placeholder data, so the user can see something went wrong
			clusterConfigData := clusterdump.ConfigData.Get()
			items = append(items, api.RelayStatusItem{
				ClusterID:   clusterConfigData.ID,
				ClusterName: clusterConfigData.Name,
				Relay:       hbRelay,
				Username:    username,
				Status:      resp.Status(),
			})
		} else {
			items = append(items, resp.JSON200.Items...)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		switch {
		case items[i].ClusterName < items[j].ClusterName:
			return true
		case items[i].ClusterName > items[j].ClusterName:
			return false
		case items[i].ClusterID < items[j].ClusterID:
			return true
		case items[i].ClusterID > items[j].ClusterID:
			return false
		case items[i].Nodename < items[j].Nodename:
			return true
		default:
			return false
		}
	})
	data := api.RelayStatusList{
		Kind:  "RelayStatusList",
		Items: items,
	}
	return ctx.JSON(http.StatusOK, data)
}

func configSectionPasswordSec(config *xconfig.T, section string) (object.Sec, error) {
	s := config.GetString(key.New(section, "password"))
	secPath, err := naming.ParsePath(s)
	if err != nil {
		return nil, err
	}
	return object.NewSec(secPath, object.WithVolatile(true))
}

func configSectionPassword(config *xconfig.T, section string) (string, error) {
	sec, err := configSectionPasswordSec(config, section)
	if err != nil {
		return "", err
	}
	b, err := sec.DecodeKey("password")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

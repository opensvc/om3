package daemonapi

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/cluster"
	"github.com/opensvc/om3/v3/core/datarecv"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/xconfig"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/rbac"
	"github.com/opensvc/om3/v3/daemon/relay"
	"github.com/opensvc/om3/v3/util/converters"
	"github.com/opensvc/om3/v3/util/key"
)

func (a *DaemonAPI) GetRelayStatus(ctx echo.Context, params api.GetRelayStatusParams) error {
	if v, err := assertGrant(ctx, rbac.GrantHeartbeat, rbac.GrantRoot); !v {
		return err
	}
	since, err := relaySince(params)
	if err != nil {
		return JSONProblemf(ctx, http.StatusBadRequest, "Invalid parameter", "%s", err)
	}
	if params.Remote != nil && *params.Remote {
		return a.getRelayStatusRemote(ctx, params, since)
	} else {
		return a.getRelayStatusLocal(ctx, since)
	}
}

// relaySince is the duration the since parameter asks for, 0 for none: the
// relay clients whose data was updated within it are listed, judged by the
// clock of the relay holding the data. The parameter is passed on as is to
// the relays a remote listing asks.
func relaySince(params api.GetRelayStatusParams) (time.Duration, error) {
	if params.Since == nil || *params.Since == "" {
		return 0, nil
	}
	v, err := converters.Duration.Convert(*params.Since)
	if err != nil {
		return 0, fmt.Errorf("field 'since' with value '%s' validation error: %w", *params.Since, err)
	}
	d := *v.(*time.Duration)
	if d < 0 {
		return 0, fmt.Errorf("field 'since' with value '%s' validation error: negative", *params.Since)
	}
	return d, nil
}

// sortRelayStatusItems orders the relay clients by the update of their data,
// the latest first: the live ones on top, the stale ones below.
func sortRelayStatusItems(items api.RelayStatusItems) {
	sort.SliceStable(items, func(i, j int) bool {
		if !items[i].UpdatedAt.Equal(items[j].UpdatedAt) {
			return items[i].UpdatedAt.After(items[j].UpdatedAt)
		}
		if items[i].ClusterID != items[j].ClusterID {
			return items[i].ClusterID < items[j].ClusterID
		}
		if items[i].Nodename != items[j].Nodename {
			return items[i].Nodename < items[j].Nodename
		}
		return items[i].NodeAddr < items[j].NodeAddr
	})
}

func (a *DaemonAPI) getRelayStatusLocal(ctx echo.Context, since time.Duration) error {
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
		username := userFromContext(ctx).Username
		slots = relay.Map.List(username)
	}
	now := time.Now()
	for _, slot := range slots {
		v := slot.Value.(api.RelayMessage)
		if since > 0 && now.Sub(v.UpdatedAt) > since {
			continue
		}
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
	sortRelayStatusItems(data.Items)
	return ctx.JSON(http.StatusOK, data)
}

func (a *DaemonAPI) getRelayStatusRemote(ctx echo.Context, params api.GetRelayStatusParams, since time.Duration) error {
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

	// configured are the relays of the relay heartbeats, as "hb#4
	// relay3.opensvc.com:3333", for the error of a filter naming none.
	configured := make([]string, 0)
	for _, section := range config.SectionStrings() {
		if !strings.HasPrefix(section, "hb#") {
			continue
		}
		if config.Get(key.New(section, "type")) != "relay" {
			continue
		}
		hbRelay := config.GetString(key.New(section, "relay"))
		configured = append(configured, section+" "+hbRelay)
		if len(relayMap) > 0 {
			// some relay filtering is on
			if _, ok := relayMap[hbRelay]; !ok {
				// filtered out
				continue
			}
			delete(relayMap, hbRelay)
		}
	}
	if len(relayMap) > 0 {
		// A relay named in the filter is no relay of a heartbeat: an
		// empty listing would read as a relay holding nothing.
		unknown := make([]string, 0, len(relayMap))
		for s := range relayMap {
			unknown = append(unknown, s)
		}
		sort.Strings(unknown)
		return JSONProblemf(ctx, http.StatusNotFound, "Relay not found",
			"%s: no relay heartbeat has this relay, the relays are the relay keywords of the relay heartbeats: %s",
			strings.Join(unknown, ", "), strings.Join(configured, ", "))
	}
	if params.Relays != nil {
		for _, s := range *params.Relays {
			relayMap[s] = nil
		}
	}

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
			clusterConfigData := cluster.ConfigData.Get()
			items = append(items, api.RelayStatusItem{
				ClusterID:   clusterConfigData.ID,
				ClusterName: clusterConfigData.Name,
				Relay:       hbRelay,
				Username:    username,
				Status:      fmt.Sprint(err),
			})
		} else if resp.StatusCode() != http.StatusOK {
			// add a placeholder data, so the user can see something went wrong
			clusterConfigData := cluster.ConfigData.Get()
			items = append(items, api.RelayStatusItem{
				ClusterID:   clusterConfigData.ID,
				ClusterName: clusterConfigData.Name,
				Relay:       hbRelay,
				Username:    username,
				Status:      resp.Status(),
			})
		} else {
			// The relay names itself by its node name: the item is
			// named by the relay keyword instead, the address the
			// relay filter takes, as the placeholders of a relay not
			// answering are.
			for _, item := range resp.JSON200.Items {
				// A relay of a version not knowing the since
				// parameter lists every client: judged here.
				if since > 0 && time.Since(item.UpdatedAt) > since {
					continue
				}
				item.Relay = hbRelay
				items = append(items, item)
			}
		}
	}
	sortRelayStatusItems(items)
	data := api.RelayStatusList{
		Kind:  "RelayStatusList",
		Items: items,
	}
	return ctx.JSON(http.StatusOK, data)
}

// configSectionPassword returns the password of the relay heartbeat of the
// section, read as the heartbeat reads it: a key reference, "from <sec> key
// <key>", or the path of a sec of the system namespace and its password key.
func configSectionPassword(config *xconfig.T, section string) (string, error) {
	s := config.GetString(key.New(section, "password"))
	km, err := datarecv.ParseKeyMetaRelWithFallback(s, naming.NsSys, "password")
	if err != nil {
		return "", err
	}
	b, err := km.Decode()
	if err != nil {
		return "", err
	}
	return string(b), nil
}

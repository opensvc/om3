package commands

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/output"
	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/core/xconfig"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/util/key"
	"github.com/opensvc/om3/util/render/tree"
)

type (
	CmdNodeRelayStatus struct {
		OptsGlobal
		Relays string
	}
	relayMessage struct {
		api.RelayMessage
		Relay string `json:"relay" yaml:"relay"`
	}
	relayMessages []relayMessage
)

func (t *CmdNodeRelayStatus) Run() error {
	messages := make(relayMessages, 0)
	relayMap := make(map[string]any)
	if t.Relays != "" {
		for _, s := range strings.Split(t.Relays, ",") {
			relayMap[s] = nil
		}
	}
	node, err := object.NewNode()
	if err != nil {
		return err
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
			return err
		}
		cli, err := client.New(
			client.WithURL(hbRelay),
			client.WithUsername(username),
			client.WithPassword(password),
			client.WithInsecureSkipVerify(insecure),
		)
		if err != nil {
			return err
		}
		params := api.GetRelayMessageParams{}
		resp, err := cli.GetRelayMessageWithResponse(context.Background(), &params)
		if err != nil {
			return err
		} else if resp.StatusCode() != http.StatusOK {
			return fmt.Errorf("unexpected get relay message status code %s", resp.Status())
		}
		for _, message := range resp.JSON200.Messages {
			messages = append(messages, relayMessage{
				Relay:        hbRelay,
				RelayMessage: message,
			})
		}
	}
	output.Renderer{
		Format:   t.Output,
		Color:    t.Color,
		Data:     messages,
		Colorize: rawconfig.Colorize,
		HumanRenderer: func() string {
			return messages.Render()
		},
	}.Print()
	return nil
}

func (t relayMessages) Len() int {
	return len(t)
}

func (t relayMessages) Less(i, j int) bool {
	switch {
	case t[i].ClusterName < t[j].ClusterName:
		return true
	case t[i].ClusterName > t[j].ClusterName:
		return false
	case t[i].ClusterId < t[j].ClusterId:
		return true
	case t[i].ClusterId > t[j].ClusterId:
		return false
	case t[i].Nodename < t[j].Nodename:
		return true
	default:
		return false
	}
}

func (t relayMessages) Swap(i, j int) {
	t[i], t[j] = t[j], t[i]
}

func (t relayMessages) Render() string {
	tree := tree.New()
	tree.AddColumn().AddText("Relay").SetColor(rawconfig.Color.Bold)
	tree.AddColumn().AddText("ClusterId").SetColor(rawconfig.Color.Bold)
	tree.AddColumn().AddText("ClusterName").SetColor(rawconfig.Color.Bold)
	tree.AddColumn().AddText("NodeName").SetColor(rawconfig.Color.Bold)
	tree.AddColumn().AddText("NodeAddr").SetColor(rawconfig.Color.Bold)
	tree.AddColumn().AddText("UpdatedAt").SetColor(rawconfig.Color.Bold)
	tree.AddColumn().AddText("MessageLength").SetColor(rawconfig.Color.Bold)
	sort.Sort(t)
	for _, e := range t {
		n := tree.AddNode()
		n.AddColumn().AddText(e.Relay).SetColor(rawconfig.Color.Primary)
		n.AddColumn().AddText(e.ClusterId)
		n.AddColumn().AddText(e.ClusterName).SetColor(rawconfig.Color.Primary)
		n.AddColumn().AddText(e.Nodename).SetColor(rawconfig.Color.Primary)
		n.AddColumn().AddText(e.Addr)
		n.AddColumn().AddText(fmt.Sprint(e.UpdatedAt))
		n.AddColumn().AddText(fmt.Sprint(len(e.Msg)))
	}
	return tree.Render()
}

func configSectionPasswordSec(config *xconfig.T, section string) (object.Sec, error) {
	s := config.GetString(key.New(section, "password"))
	secPath, err := path.Parse(s)
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

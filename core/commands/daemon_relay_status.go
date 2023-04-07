package commands

import (
	"encoding/json"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/output"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/util/hostname"
)

type (
	CmdDaemonRelayStatus struct {
		OptsGlobal
	}
)

func (t *CmdDaemonRelayStatus) Run() error {
	messages := make(relayMessages, 0)
	cli, err := client.New(client.WithURL(t.Server))
	if err != nil {
		return err
	}
	req := cli.NewGetRelayMessage()
	b, err := req.Do()
	if err != nil {
		return err
	}
	var data api.RelayMessages
	if err := json.Unmarshal(b, &data); err != nil {
		return err
	}
	relay := t.Server
	if t.Server == "" {
		relay = hostname.Hostname()
	}
	for _, message := range data.Messages {
		messages = append(messages, relayMessage{
			Relay:        relay,
			RelayMessage: message,
		})
	}
	output.Renderer{
		Format:   t.Format,
		Color:    t.Color,
		Data:     messages,
		Colorize: rawconfig.Colorize,
		HumanRenderer: func() string {
			return messages.Render()
		},
	}.Print()
	return nil
}

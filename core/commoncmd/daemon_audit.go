package commoncmd

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/fatih/color"
	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/logging"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

type (
	CmdDaemonAudit struct {
		CmdDaemonSubAction
		Output     string
		Level      string
		Subsystems []string
	}
)

const (
	maxAuditLineSize = 1024 * 1024
)

func NewCmdDaemonAudit() *cobra.Command {
	options := &CmdDaemonAudit{}
	cmd := &cobra.Command{
		Use:   "audit",
		Short: "read and render the log stream of the selected daemon subsystems up to debug and trace.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	FlagNodeSelector(flags, &options.NodeSelector)
	FlagOutput(flags, &options.Output)
	flags.StringVar(&options.Level, "level", "trace", "audit level")
	flags.StringSliceVar(&options.Subsystems, "sub", []string{}, "the names of the subsystems to audit (ccfg, collector, imon:<path>, hook, runner, daemondata, cstat, omon:<path>, dns, scheduler, discover, nmon, lsnrhttpinet, lsnrhttpux, istat, icfg:<path>, hb.peer_drop_worker, hb, daemonauth, pubsub)")
	return cmd
}

func (t *CmdDaemonAudit) Run() error {
	fn := func(ctx context.Context, c *client.T, nodename string) (response *http.Response, err error) {
		var writer io.Writer

		level := api.PostDaemonAuditParamsLevel(t.Level)

		if !level.Valid() {
			return nil, fmt.Errorf("invalid level: %s", t.Level)
		}

		subsystems := strings.Join(t.Subsystems, ",")
		params := &api.PostDaemonAuditParams{
			Level: &level,
			Sub:   &subsystems,
		}
		cli, err := client.New(client.WithTimeout(0))
		if err != nil {
			return nil, fmt.Errorf("client new : %s", err)
		}

		resp, err := cli.PostDaemonAudit(ctx, nodename, params)
		if err != nil {
			return resp, err
		}

		if resp == nil || resp.Body == nil {
			return resp, fmt.Errorf("empty response body")
		}

		defer func() {
			_ = resp.Body.Close()
		}()

		switch resp.StatusCode {
		case http.StatusOK:
		case http.StatusBadRequest:
			return resp, fmt.Errorf("bad request: %s", resp.Status)
		case http.StatusUnauthorized:
			return resp, fmt.Errorf("unauthorized: %s", resp.Status)
		case http.StatusForbidden:
			return resp, fmt.Errorf("forbidden: %s", resp.Status)
		case http.StatusInternalServerError:
			return resp, fmt.Errorf("internal server error: %s", resp.Status)
		default:
			return resp, fmt.Errorf("unexpected status code %s", resp.Status)
		}
		eventC := make(chan string)
		errC := make(chan error)
		if t.Output != "json" {
			writer = newAuditConsoleWriter()
		}
		body := resp.Body
		go auditParse(ctx, body, eventC, errC)

		for eventC != nil || errC != nil {
			select {
			case <-ctx.Done():
				err = ctx.Err()
				return resp, err
			case msg, ok := <-eventC:
				if !ok {
					eventC = nil
					continue
				}
				if err := auditRender(writer, msg, t.Output); err != nil {
					return resp, err
				}
			case err := <-errC:
				errC = nil
				if err == nil || errors.Is(err, io.EOF) {
					return resp, nil
				}
				return resp, err
			}
		}
		return resp, nil
	}
	return t.CmdDaemonSubAction.Run(fn)
}

func auditParse(ctx context.Context, body io.Reader, eventC chan<- string, errC chan<- error) {
	defer close(eventC)
	defer close(errC)

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), maxAuditLineSize)

	for scanner.Scan() {
		line := scanner.Bytes()

		if len(line) > 0 {
			if fieldName, fieldValue, ok := bytes.Cut(line, []byte{':'}); ok {
				fieldValue = bytes.TrimLeft(fieldValue, " ")
				switch string(fieldName) {
				case "":
				case "data":
					eventC <- string(fieldValue)
				default:
				}
			}
		}
	}
	err := scanner.Err()
	if err == nil {
		err = io.EOF
	}
	select {
	case <-ctx.Done():
	case errC <- err:
	}
}

func newAuditConsoleWriter() io.Writer {
	w := zerolog.NewConsoleWriter()
	w.TimeFormat = "2006-01-02T15:04:05.000Z07:00"
	w.NoColor = color.NoColor
	w.FormatLevel = logging.FormatLevel
	w.FormatFieldName = func(i any) string { return "" }
	w.FormatFieldValue = func(i any) string { return "" }
	w.FormatMessage = func(i any) string {
		return rawconfig.Colorize.Bold(i)
	}
	return w
}

func auditRender(w io.Writer, msg, format string) (err error) {
	switch format {
	case "json":
		fmt.Printf("%s\n", msg)
	default:
		_, err = w.Write([]byte(msg))
	}
	return err
}

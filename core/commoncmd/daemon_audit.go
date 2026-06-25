package commoncmd

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/logging"
)

type (
	CmdDaemonAudit struct {
		CmdDaemonSubAction
		Output     string
		Level      string
		Subsystems []string
		Preempt    bool
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
		Long: "Stream the logs of the selected daemon subsystems up to debug and trace.\n\n" +
			"Auditable subsystems:\n\n" +
			"  api api.inet api.ux ccfg collector cstat daemonauth daemondata discover dns hb hb.main hb.ctrl\n" +
			"  hb.peer_dropper hb:<hbid> hook icfg icfg:<path> imon imon:<path> istat nmon omon omon:<path> pubsub\n" +
			"  runner scheduler",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	FlagNodeSelector(flags, &options.NodeSelector)
	FlagOutput(flags, &options.Output)
	flags.StringVar(&options.Level, "level", "trace", "trace, debug, info, warn, error, fatal, panic")
	flags.StringSliceVar(&options.Subsystems, "sub", []string{}, "the names of the subsystems to audit, or empty for all")
	flags.BoolVar(&options.Preempt, "preempt", false, "preempt the current audit if any is running.")
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
			Level:   &level,
			Sub:     &subsystems,
			Preempt: &t.Preempt,
		}

		if t.Output != "json" {
			writer = newAuditConsoleWriter()
		}

		retries := 0
		maxRetries := 600

		for {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}

			cli, err := client.New(client.WithTimeout(0))
			if err != nil {
				if retries >= maxRetries {
					return nil, err
				}
				if retries == 0 {
					fmt.Fprintf(os.Stderr, "audit stream connection to %s failed: %s\n", nodename, err)
					fmt.Fprintln(os.Stderr, "press ctrl+c to interrupt retries")
				} else {
					fmt.Fprint(os.Stderr, ".")
				}
				retries++
				time.Sleep(100 * time.Millisecond)
				continue
			}

			resp, err := cli.PostDaemonAudit(ctx, nodename, params)
			if err != nil {
				if retries >= maxRetries {
					return nil, err
				}
				if retries == 0 {
					fmt.Fprintf(os.Stderr, "audit stream connection to %s failed: %s\n", nodename, err)
					fmt.Fprintln(os.Stderr, "press ctrl+c to interrupt retries")
				} else {
					fmt.Fprint(os.Stderr, ".")
				}
				retries++
				time.Sleep(100 * time.Millisecond)
				continue
			}

			if resp == nil || resp.Body == nil {
				if retries >= maxRetries {
					return nil, fmt.Errorf("empty response body")
				}
				if retries == 0 {
					fmt.Fprintf(os.Stderr, "audit stream connection to %s failed: empty response body\n", nodename)
					fmt.Fprintln(os.Stderr, "press ctrl+c to interrupt retries")
				} else {
					fmt.Fprint(os.Stderr, ".")
				}
				retries++
				time.Sleep(100 * time.Millisecond)
				continue
			}

			switch resp.StatusCode {
			case http.StatusOK:
			case http.StatusBadRequest:
				body, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				return resp, fmt.Errorf("bad request: %s, body: %s", resp.Status, string(body))
			case http.StatusUnauthorized:
				body, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				return resp, fmt.Errorf("unauthorized: %s, body: %s", resp.Status, string(body))
			case http.StatusForbidden:
				body, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				return resp, fmt.Errorf("forbidden: %s, body: %s", resp.Status, string(body))
			case http.StatusConflict:
				b, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				var p api.Problem
				if err := json.Unmarshal(b, &p); err == nil {
					return resp, fmt.Errorf("conflict: %s", p.Detail)
				}
				return resp, fmt.Errorf("conflict: %s %s", resp.Status, string(b))
			case http.StatusInternalServerError:
				body, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				return resp, fmt.Errorf("internal server error: %s, body: %s", resp.Status, string(body))
			default:
				body, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				if retries >= maxRetries {
					return resp, fmt.Errorf("unexpected status code %s, body: %s", resp.Status, string(body))
				}
				if retries == 0 {
					fmt.Fprintf(os.Stderr, "audit stream connection to %s failed: unexpected status %s\n", nodename, resp.Status)
					fmt.Fprintln(os.Stderr, "press ctrl+c to interrupt retries")
				} else {
					fmt.Fprint(os.Stderr, ".")
				}
				retries++
				time.Sleep(100 * time.Millisecond)
				continue
			}

			retries = 0

			eventC := make(chan string)
			errC := make(chan error)
			go auditParse(ctx, resp.Body, eventC, errC)

			func() {
				defer resp.Body.Close()
				for {
					select {
					case <-ctx.Done():
						return
					case msg, ok := <-eventC:
						if !ok {
							eventC = nil
							continue
						}
						if err := auditRender(writer, msg, t.Output); err != nil {
							return
						}
					case err := <-errC:
						if err == nil || errors.Is(err, io.EOF) {
							return
						}
						fmt.Fprintf(os.Stderr, "audit read error from %s: %s\n", nodename, err)
						return
					}
				}
			}()

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}
		}
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
	w.TimeFormat = time.RFC3339Nano
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

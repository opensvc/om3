package commoncmd

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/daemon/msgbus"
)

type (
	CmdDaemonHeartbeatRotate struct {
		Wait bool
		Time time.Duration
	}
)

func NewCmdDaemonHeartbeatRotate() *cobra.Command {
	options := CmdDaemonHeartbeatRotate{}
	cmd := &cobra.Command{
		Use:   "rotate",
		Short: "rotate the heartbeat encryption secret",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	flags.BoolVar(&options.Wait, "wait", false, "wait for the rotate operation to complete")
	flags.DurationVar(&options.Time, "time", 30*time.Second, "stop waiting for the rotate operation after the specified duration")
	return cmd
}

func (t *CmdDaemonHeartbeatRotate) Run() error {
	done := make(chan error, 1)

	c, err := client.New(client.WithTimeout(t.Time))
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if t.Wait {
		if err := startEventWatcher(ctx, c, done, t.Time); err != nil {
			return err
		}
	}

	resp, err := c.PostClusterHeartbeatRotateWithResponse(ctx)
	if err != nil {
		return err
	}

	if t.Wait && resp.StatusCode() == 200 {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout or cancelled while waiting for event: %w", ctx.Err())
		case e := <-done:
			return e
		}
	}

	switch resp.StatusCode() {
	case 200:
		if !t.Wait {
			fmt.Printf("%s\n", resp.JSON200.ID.String())
		}
	case 400:
		return fmt.Errorf("%s", resp.JSON400)
	case 401:
		return fmt.Errorf("%s", resp.JSON401)
	case 403:
		return fmt.Errorf("%s", resp.JSON403)
	case 409:
		return fmt.Errorf("%s", resp.JSON409)
	case 500:
		return fmt.Errorf("%s", resp.JSON500)
	default:
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode())

	}

	return nil
}

func startEventWatcher(ctx context.Context, c *client.T, done chan<- error, timeoutDuration time.Duration) error {
	getEvents := c.NewGetEvents().
		SetFilters([]string{"HeartbeatRotateSuccess", "HeartbeatRotateError"}).
		SetDuration(timeoutDuration)

	evReader, err := getEvents.GetReader()
	if err != nil {
		return err
	}
	started := make(chan struct{})
	go func() {
		ctx, cancel := context.WithTimeout(ctx, timeoutDuration)
		defer cancel()
		go func() {
			select {
			case <-ctx.Done():
				_ = evReader.Close()
			}
		}()
		started <- struct{}{}
		for {
			ev, readErr := evReader.Read()
			if readErr != nil {
				done <- readErr
				return
			}
			switch ev.Kind {
			case "HeartbeatRotateSuccess":
				var msg msgbus.HeartbeatRotateSuccess
				if err := json.Unmarshal(ev.Data, &msg); err != nil {
					done <- err
					return
				}
				fmt.Printf("%s\n", msg.ID.String())
				done <- nil
				return
			case "HeartbeatRotateError":
				var msgError msgbus.HeartbeatRotateError
				if err := json.Unmarshal(ev.Data, &msgError); err != nil {
					done <- err
					return
				}
				err = fmt.Errorf("heartbeat rotate error : %s", msgError.Reason)
				return
			}
		}
	}()
	<-started

	return nil
}

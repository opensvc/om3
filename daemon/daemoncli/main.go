package daemoncli

import (
	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/daemon/daemon"
)

func Run() error {
	main, err := daemon.RunDaemon()
	if err != nil {
		return err
	}
	main.WaitDone()
	return nil
}

func Start() error {
	_, err := daemon.RunDaemon()
	if err != nil {
		return err
	}
	return nil
}

func Stop() error {
	cli, err := client.New(client.WithURL("raw:///tmp/lsnr_ux"))
	_, err = cli.NewPostDaemonStop().Do()
	return err
}

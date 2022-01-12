package daemoncli

import (
	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/daemon/daemon"
)

var (
	socketPathUds = "/tmp/lsnr_ux"
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
	cli, err := client.New(client.WithURL("raw://" + socketPathUds))
	_, err = cli.NewPostDaemonStop().Do()
	return err
}

func Running() bool {
	var data []byte
	cli, err := client.New(client.WithURL("raw://" + socketPathUds))
	if err != nil {
		return false
	}
	data, err = cli.NewGetDaemonRunning().Do()
	if err != nil || string(data) != "running" {
		return false
	}
	if string(data) == "running" {
		return true
	}
	return false
}

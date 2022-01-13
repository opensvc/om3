package lsnrraw

import (
	"encoding/json"
	"net"

	"opensvc.com/opensvc/core/client/request"
)

func (t *T) handle(conn net.Conn) {
	defer conn.Close()
	var b = make([]byte, 4096)
	n, err := conn.Read(b)
	if err != nil {
		t.log.Error().Err(err).Msg("handle")
		return
	}
	var req request.T
	b = b[:n-1]
	if err := json.Unmarshal(b, &req); err != nil {
		t.log.Error().Err(err).Msgf("handle message: %s", string(b))
		return
	}
	t.log.Info().Msgf("handle request read %v", req)
	switch req.Action {
	case "daemon_running":
		if t.rootDaemon.Running() {
			conn.Write([]byte("running"))
		} else {
			conn.Write([]byte("not running"))
		}
	case "daemon_stop":
		t.log.Info().Msg("daemon_stop...")
		if t.rootDaemon.Running() {
			if err := t.rootDaemon.StopAndQuit(); err != nil {
				t.log.Error().Err(err).Msg("daemon_stop error")
				conn.Write([]byte("stop daemon error"))
			} else {
				t.log.Info().Msg("daemon_stop done")
				conn.Write([]byte("daemon_stop done"))
			}
		} else {
			conn.Write([]byte("daemon_stop no daemon to stop"))
		}
	}
}

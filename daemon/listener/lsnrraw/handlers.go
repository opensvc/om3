package lsnrraw

import (
	"bytes"
	"encoding/json"
	"net"
	"time"

	"opensvc.com/opensvc/core/client/request"
)

var (
	readTimeOut = 5 * time.Second
)

func (t *T) handle(conn net.Conn) {
	defer func() {
		err := conn.Close()
		if err != nil {
			t.log.Debug().Err(err).Msg("handle defer close")
			return
		}
	}()
	if err := conn.SetDeadline(time.Now().Add(readTimeOut)); err != nil {
		t.log.Error().Err(err).Msg("handle SetReadDeadline")
	}
	var b = make([]byte, 4096)
	_, err := conn.Read(b)
	if err != nil {
		t.log.Error().Err(err).Msg("handle")
		return
	}
	var req request.T
	b = bytes.TrimRight(b, "\x00")
	if err := json.Unmarshal(b, &req); err != nil {
		t.log.Error().Err(err).Msgf("handle message: %s", string(b))
		return
	}
	t.log.Info().Msgf("handle request read %v", req)
	switch req.Action {
	case "daemon_running":
		if t.rootDaemon.Running() {
			_, err := conn.Write([]byte("running"))
			if err != nil {
				t.log.Debug().Err(err).Msg("daemon_running: Write - running")
				return
			}
		} else {
			_, err := conn.Write([]byte("not running"))
			if err != nil {
				t.log.Debug().Err(err).Msg("daemon_running: Write - not running")
				return
			}
		}
	case "daemon_stop":
		t.log.Info().Msg("daemon_stop...")
		if t.rootDaemon.Running() {
			if err := t.rootDaemon.StopAndQuit(); err != nil {
				t.log.Error().Err(err).Msg("daemon_stop error")
				_, err := conn.Write([]byte("stop daemon error"))
				if err != nil {
					t.log.Debug().Err(err).Msg("daemon_running: Write - stop daemon error")
					return
				}
			} else {
				t.log.Info().Msg("daemon_stop done")
				_, err := conn.Write([]byte("daemon_stop done"))
				if err != nil {
					t.log.Debug().Err(err).Msg("daemon_running: Write - daemon_stop done")
					return
				}
			}
		} else {
			_, err := conn.Write([]byte("daemon_stop no daemon to stop"))
			if err != nil {
				t.log.Debug().Err(err).Msg("daemon_running: Write - daemon_stop no daemon to stop")
				return
			}
		}
	}
}

package lsnrrawinet

import (
	"net"
	"strings"
	"time"

	"opensvc.com/opensvc/daemon/listener/encryptconn"
	"opensvc.com/opensvc/daemon/listener/routehttp"
	"opensvc.com/opensvc/daemon/listener/routeraw"
)

func (t *T) stop() error {
	if err := (*t.listener).Close(); err != nil {
		t.log.Error().Err(err).Msg("listener Close failure")
		return err
	}
	t.log.Info().Msg("listener stopped")
	return nil
}

func (t *T) start() error {
	listener, err := net.Listen("tcp", t.addr)
	if err != nil {
		t.log.Error().Err(err).Msg("listen failed")
		return err
	}
	mux := routeraw.New(routehttp.New(t.ctx), t.log, 5*time.Second)
	c := make(chan bool)
	go func() {
		c <- true
		for {
			conn, err := listener.Accept()
			if err != nil {
				if strings.Contains(err.Error(), "use of closed network connection") {
					break
				} else {
					t.log.Error().Err(err).Msg("Accept")
					continue
				}
			}
			clearConn := encryptconn.New(conn)
			go mux.Serve(clearConn)
		}
	}()
	t.listener = &listener
	<-c
	t.log.Info().Msg("listener started " + t.addr)
	return nil
}

package dns

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/user"
	"strconv"
	"syscall"
	"time"

	"github.com/opensvc/om3/core/rawconfig"
)

type (
	domain struct {
		Zone string `json:"zone"`
	}

	getAllDomainMetadataResponse map[string][]string

	getAllDomainsResponse []domain

	getDomainMetadata struct {
		Method     string                      `json:"method"`
		Parameters getDomainMetadataParameters `json:"parameters"`
	}

	getDomainMetadataParameters struct {
		Kind string `json:"kind"`
		Name string `json:"name"`
	}

	getDomainMetadataResponse []string

	lookup struct {
		Method     string           `json:"method"`
		Parameters lookupParameters `json:"parameters"`
	}

	lookupParameters struct {
		Type string `json:"qtype"`
		Name string `json:"qname"`
	}

	lookupResponse any

	request struct {
		Method string `json:"method"`
	}
)

func (t *dns) getAllDomainMetadata(b []byte) getAllDomainMetadataResponse {
	return getAllDomainMetadataResponse{
		"ALLOW-AXFR-FROM": []string{"0.0.0.0/0", "AUTO-NS"},
	}
}

func (t *dns) getAllDomains(b []byte) getAllDomainsResponse {
	return getAllDomainsResponse{
		domain{Zone: t.cluster.Name},
	}
}

func (t *dns) getDomainMetadata(b []byte) getDomainMetadataResponse {
	var req getDomainMetadata
	if err := json.Unmarshal(b, &req); err != nil {
		t.log.Error().Err(err).Msg("request parse")
		return getDomainMetadataResponse{}
	}
	switch req.Parameters.Kind {
	case "ALLOW-AXFR-FROM":
		return getDomainMetadataResponse{"0.0.0.0/0", "AUTO-NS"}
	default:
		return getDomainMetadataResponse{}
	}
}

func (t *dns) lookup(b []byte) lookupResponse {
	var req lookup
	if err := json.Unmarshal(b, &req); err != nil {
		t.log.Error().Err(err).Msg("request parse")
		return nil
	}
	return t.getRecords(req.Parameters.Type, req.Parameters.Name)
}

func (t *dns) getRecords(recordType, recordName string) Zone {
	err := make(chan error, 1)
	c := cmdGet{
		errC: err,
		Name: recordName,
		Type: recordType,
		resp: make(chan Zone),
	}
	t.cmdC <- c
	if <-err != nil {
		return Zone{}
	}
	return <-c.resp
}

func (t *dns) sockGID() (int, error) {
	s := t.cluster.Listener.DNSSockGID
	if s == "" {
		return -1, nil
	}
	if i, err := strconv.Atoi(s); err == nil {
		return i, nil
	} else if grp, err := user.LookupGroupId(s); err != nil {
		return -1, err
	} else if grp == nil {
		return -1, nil
	} else {
		i, _ := strconv.Atoi(grp.Gid)
		return i, nil
	}
}

func (t *dns) sockUID() (int, error) {
	s := t.cluster.Listener.DNSSockUID
	if s == "" {
		return -1, nil
	}
	if i, err := strconv.Atoi(s); err == nil {
		return i, nil
	} else if usr, err := user.LookupId(s); err != nil {
		return -1, err
	} else if usr == nil {
		return -1, nil
	} else {
		i, _ := strconv.Atoi(usr.Uid)
		return i, nil
	}
}

func (t *dns) sockChown() error {
	var uid, gid int
	sockPath := rawconfig.DNSUDSFile()
	if info, err := os.Stat(sockPath); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	} else if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		uid = int(stat.Uid)
		gid = int(stat.Gid)
	}
	if sockUID, err := t.sockUID(); err != nil {
		return err
	} else if sockGID, err := t.sockGID(); err != nil {
		return err
	} else if (sockUID == uid) && (sockGID == gid) {
		// no change
		return nil
	} else if err := os.Chown(sockPath, sockUID, sockGID); err != nil {
		return err
	} else {
		t.log.Info().Msgf("chown %d:%d %s", sockUID, sockGID, sockPath)
		return nil
	}
}

func (t *dns) startUDSListener() error {
	sockDir := rawconfig.DNSUDSDir()
	sockPath := rawconfig.DNSUDSFile()

	if err := os.MkdirAll(sockDir, 0755); err != nil {
		return err
	}

	_ = os.Remove(sockPath)

	l, err := net.Listen("unix", sockPath)
	if err != nil {
		return err
	}

	if err := t.sockChown(); err != nil {
		return err
	}

	type PDNSResponse struct {
		Result any    `json:"result"`
		Error  string `json:"error,omitempty"`
	}

	sendBytes := func(conn net.Conn, b []byte) error {
		b = append(b, []byte("\n")...)
		t.log.Debug().Msgf("response: %s", string(b))
		conn.SetWriteDeadline(time.Now().Add(1 * time.Second))
		for {
			n, err := conn.Write(b)
			if err != nil {
				t.log.Error().Err(err).Msg("response write")
				return err
			}
			if n == 0 {
				break
			}
			b = b[n:]
		}
		return err
	}

	sendError := func(conn net.Conn, err error) error {
		response := PDNSResponse{
			Error:  fmt.Sprint(err),
			Result: false,
		}
		b, _ := json.Marshal(response)
		return sendBytes(conn, b)
	}

	send := func(conn net.Conn, data any) error {
		response := PDNSResponse{
			Result: data,
		}
		b, err := json.Marshal(response)
		if err != nil {
			return sendError(conn, err)
		}
		return sendBytes(conn, b)
	}

	serve := func(conn net.Conn) {
		var (
			message []byte
			req     request
		)
		defer conn.Close()
		t.log.Debug().Msg("Client connected")
		for {
			conn.SetReadDeadline(time.Now().Add(1 * time.Second))
			buffer := make([]byte, 1024)

			n, err := conn.Read(buffer)
			message = buffer[:n]

			if os.IsTimeout(err) {
				return
			} else if errors.Is(err, io.EOF) {
				// pass
			} else if err != nil {
				t.log.Error().Err(err).Msg("request read")
				return
			}

			if n <= 0 {
				// empty message
				return
			}

			t.log.Debug().Msgf("request: %s", string(message))

			if err := json.Unmarshal(message, &req); err != nil {
				t.log.Error().Err(err).Msg("request parse")
				return
			}
			switch req.Method {
			case "getAllDomainMetadata":
				_ = send(conn, t.getAllDomainMetadata(message))
			case "getAllDomains":
				_ = send(conn, t.getAllDomains(message))
			case "getDomainMetadata":
				_ = send(conn, t.getDomainMetadata(message))
			case "lookup":
				_ = send(conn, t.lookup(message))
			case "initialize":
				_ = send(conn, true)
			}
		}
	}
	listen := func() {
		defer l.Close()

		for {
			conn, err := l.Accept()
			if err != nil {
				t.log.Error().Err(err).Msg("UDS accept")
			}
			go serve(conn)
		}
	}
	go listen()
	return nil
}

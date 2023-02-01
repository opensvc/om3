package dns

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"time"

	"opensvc.com/opensvc/core/rawconfig"
)

type (
	request struct {
		Method string `json:"method"`
	}

	getDomainMetadataParameters struct {
		Kind string `json:"kind"`
		Name string `json:"name"`
	}
	getDomainMetadata struct {
		Method     string                      `json:"method"`
		Parameters getDomainMetadataParameters `json:"parameters"`
	}
	getDomainMetadataResponse []string

	lookupParameters struct {
		Type string `json:"qtype"`
		Name string `json:"qname"`
	}
	lookup struct {
		Method     string           `json:"method"`
		Parameters lookupParameters `json:"parameters"`
	}
	lookupResponse any
)

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
	resp := make(chan Zone)
	t.cmdC <- cmdGet{
		Type: recordType,
		Name: recordName,
		resp: resp,
	}
	return <-resp
}

func (t *dns) startUDSListener() error {
	sockDir := rawconfig.DNSUDSDir()
	sockPath := rawconfig.DNSUDSFile()

	if err := os.MkdirAll(sockDir, 0750); err != nil {
		return err
	}

	_ = os.Remove(sockPath)

	l, err := net.Listen("unix", sockPath)
	if err != nil {
		return err
	}

	type PDNSResponse struct {
		Result any    `json:"result"`
		Error  string `json:"error,omitempty"`
	}

	sendBytes := func(conn net.Conn, b []byte) error {
		b = append(b, []byte("\n")...)
		t.log.Info().Msgf("response: %s", string(b))
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
		t.log.Info().Msgf("Client connected [%s]", conn.RemoteAddr().Network())
		for {
			conn.SetReadDeadline(time.Now().Add(1 * time.Second))
			buffer := make([]byte, 1024)

			n, err := conn.Read(buffer)
			message = buffer[:n]

			if n > 0 {
				t.log.Info().Msgf("request: %s", string(message))
			}
			if err != nil {
				t.log.Error().Err(err).Msg("request read")
				return
			}
			if err := json.Unmarshal(message, &req); err != nil {
				t.log.Error().Err(err).Msg("request parse")
				return
			}
			switch req.Method {
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

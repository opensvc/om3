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

func (t *Manager) getAllDomainMetadata(b []byte) getAllDomainMetadataResponse {
	return getAllDomainMetadataResponse{
		"ALLOW-AXFR-FROM": []string{"0.0.0.0/0", "AUTO-NS"},
	}
}

func (t *Manager) getAllDomains(b []byte) getAllDomainsResponse {
	return getAllDomainsResponse{
		domain{Zone: t.cluster.Name},
	}
}

func (t *Manager) getDomainMetadata(b []byte) getDomainMetadataResponse {
	var req getDomainMetadata
	if err := json.Unmarshal(b, &req); err != nil {
		t.log.Errorf("request parse: %s", err)
		return getDomainMetadataResponse{}
	}
	switch req.Parameters.Kind {
	case "ALLOW-AXFR-FROM":
		return getDomainMetadataResponse{"0.0.0.0/0", "AUTO-NS"}
	default:
		return getDomainMetadataResponse{}
	}
}

func (t *Manager) lookup(b []byte) lookupResponse {
	var req lookup
	if err := json.Unmarshal(b, &req); err != nil {
		t.log.Errorf("request parse: %s", err)
		return nil
	}
	return t.getRecords(req.Parameters.Type, req.Parameters.Name)
}

func (t *Manager) getRecords(recordType, recordName string) Zone {
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

func (t *Manager) sockGID() (int, error) {
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

func (t *Manager) sockUID() (int, error) {
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

// sockChown chown dns uds file and return bool true on changes
func (t *Manager) sockChown() (bool, error) {
	var uid, gid int
	sockPath := rawconfig.DNSUDSFile()
	if info, err := os.Stat(sockPath); os.IsNotExist(err) {
		return false, err
	} else if err != nil {
		return false, err
	} else if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		uid = int(stat.Uid)
		gid = int(stat.Gid)
	}
	if sockUID, err := t.sockUID(); err != nil {
		return false, err
	} else if sockGID, err := t.sockGID(); err != nil {
		return false, err
	} else if (sockUID == uid) && (sockGID == gid) {
		// no change
		return false, nil
	} else if err := os.Chown(sockPath, sockUID, sockGID); err != nil {
		return false, err
	} else {
		t.log.Infof("chown %d:%d %s", sockUID, sockGID, sockPath)
		return true, nil
	}
}

func (t *Manager) startUDSListener() error {
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

	if changed, err := t.sockChown(); err != nil {
		return fmt.Errorf("sock chown error: %s", err)
	} else if changed {
		t.status.ConfiguredAt = time.Now()
		t.publishSubsystemDnsUpdated()
	}

	type PDNSResponse struct {
		Result any    `json:"result"`
		Error  string `json:"error,omitempty"`
	}

	sendBytes := func(conn net.Conn, b []byte) error {
		b = append(b, []byte("\n")...)
		t.log.Debugf("response: %s", string(b))
		if err := conn.SetWriteDeadline(time.Now().Add(1 * time.Second)); err != nil {
			t.log.Warnf("can't set response write deadline: %s", err)
		}
		for {
			n, err := conn.Write(b)
			if err != nil {
				t.log.Errorf("response write: %s", err)
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
		t.log.Debugf("client connected")
		for {
			if err := conn.SetReadDeadline(time.Now().Add(1 * time.Second)); err != nil {
				t.log.Warnf("can't set client read deadline: %s", err)
			}
			buffer := make([]byte, 1024)

			n, err := conn.Read(buffer)
			message = buffer[:n]

			if os.IsTimeout(err) {
				return
			} else if errors.Is(err, io.EOF) {
				// pass
			} else if err != nil {
				t.log.Errorf("request read: %s", err)
				return
			}

			if n <= 0 {
				// empty message
				return
			}

			t.log.Debugf("request: %s", string(message))

			if err := json.Unmarshal(message, &req); err != nil {
				t.log.Errorf("request parse: %s", err)
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
		go func() {
			defer l.Close()
			select {
			case <-t.ctx.Done():
				return
			}
		}()

		for {
			conn, err := l.Accept()
			if err != nil {
				if !errors.Is(err, net.ErrClosed) {
					t.log.Warnf("UDS accept: %s", err)
				}
				return
			}
			t.wg.Add(1)
			go func() {
				defer t.wg.Done()
				serve(conn)
			}()
		}
	}

	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		listen()
	}()

	return nil
}

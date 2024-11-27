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

	list struct {
		Method     string         `json:"method"`
		Parameters listParameters `json:"parameters"`
	}

	listParameters struct {
		Zonename string `json:"zonename"`
	}

	lookup struct {
		Method     string           `json:"method"`
		Parameters lookupParameters `json:"parameters"`
	}

	lookupParameters struct {
		Type string `json:"qtype"`
		Name string `json:"qname"`
	}

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
		domain{Zone: t.clusterConfig.Name},
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

func (t *Manager) list(b []byte) Zone {
	var req list
	if err := json.Unmarshal(b, &req); err != nil {
		t.log.Errorf("request parse: %s", err)
		return nil
	}
	return t.getList(req.Parameters.Zonename)
}

func (t *Manager) getList(zonename string) Zone {
	if zonename != t.clusterConfig.Name+"." {
		return Zone{}
	}
	err := make(chan error, 1)
	c := cmdGetZone{
		errC: err,
		resp: make(chan Zone),
	}
	t.cmdC <- c
	if <-err != nil {
		return Zone{}
	}
	return <-c.resp
}

func (t *Manager) lookup(b []byte) Zone {
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
	s := t.clusterConfig.Listener.DNSSockGID
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
	s := t.clusterConfig.Listener.DNSSockUID
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
	sockDir := rawconfig.DNSUDSDir()
	sockPath := rawconfig.DNSUDSFile()
	changed := false
	if c, err := t.chown(sockDir); err != nil {
		return changed, fmt.Errorf("%s chown error: %s", sockDir, err)
	} else {
		changed = changed || c
	}
	if c, err := t.chown(sockPath); err != nil {
		return changed, fmt.Errorf("%s chown error: %s", sockPath, err)
	} else {
		changed = changed || c
	}
	return changed, nil
}

// chown file and return bool true on changes
func (t *Manager) chown(filename string) (bool, error) {
	var uid, gid int
	if info, err := os.Stat(filename); os.IsNotExist(err) {
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
	} else if err := os.Chown(filename, sockUID, sockGID); err != nil {
		return false, err
	} else {
		t.log.Infof("chown %d:%d %s", sockUID, sockGID, filename)
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
		return err
	} else if changed {
		t.status.ConfiguredAt = time.Now()
		t.publishSubsystemDnsUpdated()
	}

	type PDNSResponse struct {
		Result any    `json:"result"`
		Error  string `json:"error,omitempty"`
	}

	sendBytes := func(id uint64, conn net.Conn, b []byte) error {
		b = append(b, []byte("\n")...)
		t.log.Debugf("%d: >>> %s", id, string(b))
		if err := conn.SetWriteDeadline(time.Now().Add(1 * time.Second)); err != nil {
			t.log.Warnf("%d: can't set response write deadline: %s", id, err)
		}
		for {
			n, err := conn.Write(b)
			if err != nil {
				t.log.Errorf("%d: %s", id, err)
				return err
			}
			if n == 0 {
				break
			}
			b = b[n:]
		}
		return err
	}

	sendError := func(id uint64, conn net.Conn, err error) error {
		response := PDNSResponse{
			Error:  fmt.Sprint(err),
			Result: false,
		}
		b, _ := json.Marshal(response)
		t.log.Debugf("%d: >>> %s", id, string(b))
		return sendBytes(id, conn, b)
	}

	send := func(id uint64, conn net.Conn, data any) error {
		response := PDNSResponse{
			Result: data,
		}
		b, err := json.Marshal(response)
		if err != nil {
			return sendError(id, conn, err)
		}
		return sendBytes(id, conn, b)
	}

	serve := func(id uint64, conn net.Conn) {
		var (
			message  []byte
			req      request
			reqCount uint64
		)
		defer conn.Close()
		t.log.Infof("%d: new connection", id)
		for {
			if err := conn.SetReadDeadline(time.Now().Add(60 * time.Second)); err != nil {
				t.log.Infof("%d: can't set client read deadline: %s", id, err)
			}
			buffer := make([]byte, 1024)

			n, err := conn.Read(buffer)
			message = buffer[:n]

			if os.IsTimeout(err) {
				t.log.Debugf("%d: alive", id)
				continue
			} else if errors.Is(err, io.EOF) {
				t.log.Infof("%d: close connection (%s), served %d requests", id, err, reqCount)
				return
			} else if err != nil {
				t.log.Errorf("%d: close connection (%s), served %d requests", id, err, reqCount)
				return
			}

			if n <= 0 {
				// empty message
				continue
			}

			reqCount++
			t.log.Debugf("%d: <<< %s", id, string(message))

			if err := json.Unmarshal(message, &req); err != nil {
				t.log.Errorf("%d: close connection (%s), served %d requests", id, err, reqCount)
				return
			}
			switch req.Method {
			case "getAllDomainMetadata":
				_ = send(id, conn, t.getAllDomainMetadata(message))
			case "getAllDomains":
				_ = send(id, conn, t.getAllDomains(message))
			case "getDomainMetadata":
				_ = send(id, conn, t.getDomainMetadata(message))
			case "list":
				_ = send(id, conn, t.list(message))
			case "lookup":
				_ = send(id, conn, t.lookup(message))
			case "initialize":
				_ = send(id, conn, true)
			}
		}
	}
	listen := func() {
		go func() {
			select {
			case <-t.ctx.Done():
				t.log.Debugf("stop listening (abort)")
				_ = l.Close()
				return
			}
		}()

		var i uint64
		for {
			conn, err := l.Accept()
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					t.log.Debugf("stop listening (%s)", err)
					return
				}
				t.log.Warnf("accept connection error: %s", err)
				continue
			}
			// TODO: limit number conn routines ?
			t.wg.Add(1)
			i += 1
			go func() {
				defer t.wg.Done()
				serve(i, conn)
			}()
		}
	}

	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		t.log.Debugf("start listening")
		listen()
	}()

	return nil
}

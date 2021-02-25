package client

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"path/filepath"

	"opensvc.com/opensvc/config"
)

type (
	// JSONRPC is the agent JSON RPC api struct
	JSONRPC struct {
		URL string
	}
)

const (
	// JSONRPCScheme is the JSONRPC protocol scheme prefix in URL
	JSONRPCScheme string = "raw://"
)

// JSONRPCUDSPath formats the JSONRPC api unix domain socket path
func JSONRPCUDSPath() string {
	return filepath.FromSlash(fmt.Sprintf("%s/lsnr/lsnr.sock", config.Viper.GetString("paths.var")))
}

// Get implements the Get interface method for the JSONRPC api
func (t JSONRPC) Get(req Request) (*http.Response, error) {
	conn, err := net.Dial("unix", JSONRPCUDSPath())

	if err != nil {
		return nil, err
	}
	req.Method = "GET"
	b, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	conn.Write(b)
	conn.Write([]byte("\x00"))
	if err != nil {
		conn.Close()
		return nil, err
	}
	resp := &http.Response{
		Body: conn,
	}
	return resp, nil
}

func newJSONRPC(c Config) (JSONRPC, error) {
	return JSONRPC{}, nil
}

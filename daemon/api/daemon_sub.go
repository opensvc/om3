package api

import (
	"context"
	"net/http"
)

const (
	DaemonSubHeartbeat string = "hb"
	DaemonSubListener  string = "listener"
)

type (
	PostDaemonSubAction func(context.Context, InPathNodeName, InPathDaemonSubAction, DaemonSubNameBody, ...RequestEditorFn) (*http.Response, error)
)

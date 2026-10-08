//go:generate oapi-codegen --config=codegen_server.yaml ./api.yaml
//go:generate oapi-codegen --config=codegen_type.yaml ./api.yaml
//go:generate oapi-codegen --config=codegen_client.yaml ./api.yaml

package api

import (
	"fmt"
)

const (
	HeaderGroup        = "OM-Group"
	HeaderLastModified = "OM-Last-Modified"
	HeaderPerm         = "OM-Perm"
	HeaderRelativePath = "OM-relative-path"
	HeaderServedBy     = "OM-Served-By"
	HeaderUser         = "OM-User"
)

// AuditEventPreempted names the server-sent event a daemon audit stream ends
// with when another session preempts it.
const AuditEventPreempted = "preempted"

const (
	AliasLocalhost      = "localhost"
	AliasShortLocalhost = "_"
)

func (t OrchestrationQueued) String() (out string) {
	return fmt.Sprint(t.OrchestrationID)
}

func (t Problem) String() (out string) {
	if t.Status >= 300 {
		out += fmt.Sprintf("[%d] ", t.Status)
	}
	out += t.Title
	if t.Detail != "" {
		out += fmt.Sprintf(": %s", t.Detail)
	}
	return
}

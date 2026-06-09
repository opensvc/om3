// xsession is a package managing the ExecId and SessionId lifecycles.
//
// ExecId identifies every om process spawned.
//
// SessionId identifies the command execution and all crm commands
// forked from this execution.
//
// 1/ Who allocates a SessionId ?
//
//    OR user
//    OR daemon scheduler
//    OR daemon imon
//    OR daemon api (if no session_id query parameter)
//    OR crm (if no OSVC_SESSION_ID env variable)
//
// 2/ How a SessionId is propagated ?
//
//    OR query params: session_id
//    OR environement: OSVC_SESSION_ID
//
// 3/ When the SessionId is propagated:
//
//    - When the crm execs the crm
//      => export OSVC_SESSION_ID=xxx
//
//      Use-cases:
//      - encap
//      - volumes
//      - task (can exec crm)
//      - app (can exec crm)
//      - trigger pre/post (can exec crm)
//
//    - When the daemon decides of a crm exec
//      => New SessionId created by the daemon
//      => export OSVC_SESSION_ID=xxx
//
//      Use-cases:
//      - scheduler
//      - imon
//      - nmon (drain)
//
//    - When the daemon is asked to exec the crm
//      => New SessionId created by the requester, passed by the `session_id` query parameter.
//      => If not, created by the api handler.
//
//      Use-cases:
//      - Remote exec instance action
//

package xsession

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/google/uuid"
)

type Id struct {
	id            uuid.UUID
	varName       string
	parentVarName string
}

var (
	//
	// sid is an uuid identifying the command execution and all crm commands
	// forked from this execution.
	//
	// This uuid is embedded in the logs so it's easy to retrieve
	// the logs of an execution.
	//
	// Asynchronous commands posted on the API return a Id,
	// so logs can be streamed for this execution after posting.
	//
	// The opensvc daemon forges an Id and exports it in
	// the CRM commands it executes as a OSVC_SESSION_ID environment
	// variable.
	//
	// The ID is also used as a caching session. Spawned
	// subprocesses using the "cache" package store and retrieve
	// their out, err, ret from the session cache identified by
	// the spawner ID.
	//
	sid Id
	eid Id
	oid Id
)

// NewEid creates a new ExecId. If no uuid is given, assign a random one.
func NewEid(ids ...uuid.UUID) Id {
	i := Id{
		varName:       "OSVC_EXEC_ID",
		parentVarName: "OSVC_PARENT_EXEC_ID",
	}
	for _, id := range ids {
		i.id = id
	}
	if i.id == uuid.Nil {
		i.id = uuid.New()
	}
	return i
}

// NewSid creates a new SessionId. If no uuid is given, assign a random one.
func NewSid(ids ...uuid.UUID) Id {
	i := Id{
		varName:       "OSVC_SESSION_ID",
		parentVarName: "OSVC_PARENT_SESSION_ID",
	}
	for _, id := range ids {
		i.id = id
	}
	if i.id == uuid.Nil {
		i.id = uuid.New()
	}
	return i
}

// NewOid creates a new OrchestrationId. If no uuid is given, assign a random one.
func NewOid(ids ...uuid.UUID) Id {
	i := Id{
		varName: "OSVC_ORCHESTRATION_ID",
	}
	for _, id := range ids {
		i.id = id
	}
	if i.id == uuid.Nil {
		i.id = uuid.New()
	}
	return i
}

// UUID returns the underlying uuid.UUID value.
func (t *Id) UUID() uuid.UUID {
	return t.id
}

func (t *Id) SetUUID(id uuid.UUID) {
	t.id = id
}

func (t *Id) Zero() {
	t.id = uuid.Nil
}

func (t *Id) IsZero() bool {
	return t.id == uuid.Nil
}

// String returns the string representation of the SessionId.
func (t *Id) String() string {
	return t.id.String()
}

// MarshalJSON implements json.Marshaler interface.
// It marshals the SessionId as a JSON string representation of the UUID.
// This method is also used by sigs.k8s.io/yaml for YAML marshaling.
func (t Id) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.id.String())
}

// UnmarshalJSON implements json.Unmarshaler interface.
// It unmarshals a JSON string into a SessionId.
// This method is also used by sigs.k8s.io/yaml for YAML unmarshaling.
func (t *Id) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	u, err := uuid.Parse(s)
	if err != nil {
		return err
	}
	t.id = u
	return nil
}

func (t *Id) setenvArg(s string) string {
	var buff strings.Builder
	buff.WriteString(s)
	buff.WriteString("=")
	buff.WriteString(t.String())
	return buff.String()
}

// Var returns the session id as a OSVC_SESSION_ID=<sid> environment variable setter string.
func (t *Id) Var() string {
	return t.setenvArg(t.varName)
}

// ParentVar returns the session id as a OSVC_PARENT_SESSION_ID=<sid> environment variable setter string.
// Used by the daemon to provide its session id to the executed crm commands.
func (t *Id) ParentVar() string {
	return t.setenvArg(t.parentVarName)
}

func (t *Id) Load() {
	s := os.Getenv(t.varName)
	if u, err := uuid.Parse(s); err == nil {
		t.id = u
	}
}

// initID wraps init so it can be tested.
func initID() {
	eid = NewEid()
	eid.Load()
	oid = NewOid()
	oid.Load()
	sid = NewSid()
	sid.Load()
}

func init() {
	initID()
}

// Eid returns the Exec ID
func Eid() *Id {
	return &eid
}

// Sid returns the Session ID
func Sid() *Id {
	return &sid
}

// Oid returns the Orchestration ID
func Oid() *Id {
	return &oid
}

// ResetSid is for go test
func ResetSid(id Id) {
	sid = id
}

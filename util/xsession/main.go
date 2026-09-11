// xsession is a package managing the ExecID, SessionID and OrchestrationID
// lifecycles.
//
// ExecID identifies every om process spawned.
//
// SessionID identifies the command execution and all crm commands
// forked from this execution.
//
// OrchestrationID identifies the target state the daemons cooperate to
// reach, when the execution is a step of one.
//
// 1/ Who allocates a SessionID ?
//
//    OR user
//    OR daemon scheduler
//    OR daemon imon
//    OR daemon api (if no session_id query parameter)
//    OR crm (if no OSVC_SESSION_ID env variable)
//
// 2/ How a SessionID is propagated ?
//
//    OR query params: session_id
//    OR environement: OSVC_SESSION_ID
//
// 3/ When the SessionID is propagated:
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
//      => New SessionID created by the daemon
//      => export OSVC_SESSION_ID=xxx
//
//      Use-cases:
//      - scheduler
//      - imon
//      - nmon (drain)
//
//    - When the daemon is asked to exec the crm
//      => New SessionID created by the requester, passed by the `session_id` query parameter.
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

type ID struct {
	id            uuid.UUID
	varName       string
	parentVarName string
}

var (
	//
	// sessionID is an uuid identifying the command execution and all crm
	// commands forked from this execution.
	//
	// This uuid is embedded in the logs so it's easy to retrieve
	// the logs of an execution.
	//
	// Asynchronous commands posted on the API return a ID,
	// so logs can be streamed for this execution after posting.
	//
	// The opensvc daemon forges an ID and exports it in
	// the CRM commands it executes as a OSVC_SESSION_ID environment
	// variable.
	//
	// The ID is also used as a caching session. Spawned
	// subprocesses using the "cache" package store and retrieve
	// their out, err, ret from the session cache identified by
	// the spawner ID.
	//
	sessionID       ID
	execID          ID
	orchestrationID ID
)

// NewExecID creates a new exec id. If no uuid is given, assign a random one.
func NewExecID(ids ...uuid.UUID) ID {
	i := ID{
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

// NewSessionID creates a new session id. If no uuid is given, assign a random one.
func NewSessionID(ids ...uuid.UUID) ID {
	i := ID{
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

// NewOrchestrationID creates an orchestration id carrying the id it is given,
// and carrying none when that id is nil.
//
// Unlike NewSessionID and NewExecID, a nil id is not answered with a fresh
// random one: an exec naming itself in its own logs wants an id whatever
// happens, where a caller reporting which orchestration something belongs to
// wants the truth, and a random one would report belonging to an
// orchestration that never existed.
func NewOrchestrationID(id uuid.UUID) ID {
	return ID{
		varName: "OSVC_ORCHESTRATION_ID",
		id:      id,
	}
}

// UUID returns the underlying uuid.UUID value.
func (t *ID) UUID() uuid.UUID {
	return t.id
}

func (t *ID) SetUUID(id uuid.UUID) {
	t.id = id
}

func (t *ID) Zero() {
	t.id = uuid.Nil
}

func (t *ID) IsZero() bool {
	return t.id == uuid.Nil
}

// String returns the string representation of the id.
func (t *ID) String() string {
	return t.id.String()
}

// MarshalJSON implements json.Marshaler interface.
// It marshals the id as a JSON string representation of the UUID.
// This method is also used by sigs.k8s.io/yaml for YAML marshaling.
func (t ID) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.id.String())
}

// UnmarshalJSON implements json.Unmarshaler interface.
// It unmarshals a JSON string into an id.
// This method is also used by sigs.k8s.io/yaml for YAML unmarshaling.
func (t *ID) UnmarshalJSON(b []byte) error {
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

func (t *ID) setenvArg(s string) string {
	var buff strings.Builder
	buff.WriteString(s)
	buff.WriteString("=")
	buff.WriteString(t.String())
	return buff.String()
}

// Var returns the id as a OSVC_SESSION_ID=<session id> environment variable
// setter string, and the empty string when there is no id to set, so a caller
// exporting an orchestration id it may not have can hand what it gets to the
// environment without naming an orchestration that never existed.
func (t *ID) Var() string {
	if t.IsZero() {
		return ""
	}
	return t.setenvArg(t.varName)
}

// ParentVar returns the id as a OSVC_PARENT_SESSION_ID=<session id>
// environment variable setter string, and the empty string when there is no
// id to set.
// Used by the daemon to provide its session id to the executed crm commands.
func (t *ID) ParentVar() string {
	if t.IsZero() {
		return ""
	}
	return t.setenvArg(t.parentVarName)
}

func (t *ID) Load() {
	s := os.Getenv(t.varName)
	if u, err := uuid.Parse(s); err == nil {
		t.id = u
	}
}

// initID wraps init so it can be tested.
func initID() {
	execID = NewExecID()
	execID.Load()

	// The orchestration id is loaded, never minted: a command is a step of an
	// orchestration only when the daemon that forked it said so.
	orchestrationID = NewOrchestrationID(uuid.Nil)
	orchestrationID.Load()

	sessionID = NewSessionID()
	sessionID.Load()
}

func init() {
	initID()
}

// ExecID returns the exec id of this process.
func ExecID() *ID {
	return &execID
}

// SessionID returns the session id of this process.
func SessionID() *ID {
	return &sessionID
}

// OrchestrationID returns the orchestration id this process is a step of, and
// a zero id when it is a step of none.
func OrchestrationID() *ID {
	return &orchestrationID
}

// ResetSessionID is for go test
func ResetSessionID(id ID) {
	sessionID = id
}

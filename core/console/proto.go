package console

import "encoding/json"

// A console session is a websocket. Its binary messages are the bytes of
// the terminal, in both directions. Its text messages are the control
// messages below, json encoded.
const (
	// MsgResize is sent by the client when its terminal changes size.
	MsgResize = "resize"

	// MsgExit is sent by the node when the session ends, before it closes
	// the websocket.
	MsgExit = "exit"
)

// Reasons a session ends for.
const (
	// ReasonExited is the command of the session ending by itself.
	ReasonExited = "exited"

	// ReasonError is the session failing to start or to run.
	ReasonError = "error"
)

// Message is a control message of a console session.
type Message struct {
	Type string `json:"type"`

	// Cols and Rows are the terminal size of a resize.
	Cols uint16 `json:"cols,omitempty"`
	Rows uint16 `json:"rows,omitempty"`

	// Code is the exit code of the command, Reason why the session
	// ended, and Text what to tell the user about it.
	Code   int    `json:"code,omitempty"`
	Reason string `json:"reason,omitempty"`
	Text   string `json:"text,omitempty"`
}

// Encode returns the message as a text websocket payload.
func (m Message) Encode() []byte {
	b, _ := json.Marshal(m)
	return b
}

// DecodeMessage parses a text websocket payload.
func DecodeMessage(b []byte) (Message, error) {
	var m Message
	err := json.Unmarshal(b, &m)
	return m, err
}

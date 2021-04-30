package trigger

type (
	Blocking int
	Hook     int
	Action   int
)

const (
	Block Blocking = iota
	NoBlock
)

const (
	Pre Hook = iota
	Post
)

const (
	Start Action = iota
	Stop
	Provision
	Unprovision
	Startstandby
	Shutdown
	SyncNodes
	SyncDRP
	SyncAll
	SyncResync
	SyncUpdate
	SyncRestore
	Run
	OnError // tasks use that as an action
	Command // tasks use that as an action
)

var (
	blockingToString = map[Blocking]string{
		Block:   "blocking",
		NoBlock: "non-blocking",
	}
	hookToString = map[Hook]string{
		Pre:  "pre",
		Post: "post",
	}
	actionToString = map[Action]string{
		Start:        "start",
		Stop:         "stop",
		Provision:    "provision",
		Unprovision:  "unprovision",
		Startstandby: "startstandby",
		Shutdown:     "shutdown",
		SyncNodes:    "syncnodes",
		SyncDRP:      "syncdrp",
		SyncAll:      "syncall",
		SyncResync:   "syncresync",
		SyncUpdate:   "syncupdate",
		SyncRestore:  "syncrestore",
		Run:          "run",
		OnError:      "on-error",
		Command:      "command",
	}
)

func (t Blocking) String() string {
	s, ok := blockingToString[t]
	if ok {
		return s
	}
	return "unknown blocking"
}
func (t Hook) String() string {
	s, ok := hookToString[t]
	if ok {
		return s
	}
	return "unknown hook"
}
func (t Action) String() string {
	s, ok := actionToString[t]
	if ok {
		return s
	}
	return "unknown action"
}

package routeraw

type (
	rawToHttp struct {
		method string
		path   string
	}
)

var actionToPath = map[string]rawToHttp{
	"daemon_running":    {"GET", "/daemon/running"},
	"daemon_stop":       {"POST", "/daemon/stop"},
	"daemon_eventsdemo": {"GET", "/daemon/eventsdemo"},

	"daemon/running":    {"GET", "/daemon/running"},
	"daemon/stop":       {"POST", "/daemon/stop"},
	"daemon/eventsdemo": {"GET", "/daemon/eventsdemo"},
}

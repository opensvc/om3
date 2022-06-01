package routeraw

type (
	rawToHttp struct {
		method string
		path   string
	}
)

var actionToPath = map[string]rawToHttp{
	"daemon_running":    {"GET", "/daemon/running"},
	"daemon_status":     {"GET", "/daemon/status"},
	"daemon_stop":       {"POST", "/daemon/stop"},
	"daemon_eventsdemo": {"GET", "/daemon/eventsdemo"},

	"daemon/running":    {"GET", "/daemon/running"},
	"daemon/stop":       {"POST", "/daemon/stop"},
	"daemon/eventsdemo": {"GET", "/daemon/eventsdemo"},

	"object/config":   {"GET", "/object/config"},
	"object_config":   {"GET", "/object/config"},
	"object/selector": {"GET", "/object/selector"},
	"object_selector": {"GET", "/object/selector"},
	"object/status":   {"POST", "/object/status"},
	"object_status":   {"POST", "/object/status"},
}

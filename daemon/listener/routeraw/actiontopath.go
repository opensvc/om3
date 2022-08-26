package routeraw

type (
	rawToHttp struct {
		method string
		path   string
	}
)

var actionToPath = map[string]rawToHttp{
	"node_log":     {"GET", "/node/log"},
	"node/log":     {"GET", "/node/log"},
	"node_backlog": {"GET", "/node/backlog"},
	"node/backlog": {"GET", "/node/backlog"},

	"daemon_running": {"GET", "/daemon/running"},
	"daemon_status":  {"GET", "/daemon/status"},
	"daemon_stop":    {"POST", "/daemon/stop"},
	"daemon_events":  {"GET", "/daemon/events"},

	"daemon/running": {"GET", "/daemon/running"},
	"daemon/stop":    {"POST", "/daemon/stop"},
	"daemon/events":  {"GET", "/daemon/events"},

	"object/config":   {"GET", "/object/config"},
	"object_config":   {"GET", "/object/config"},
	"object/monitor":  {"POST", "/object/monitor"},
	"object_monitor":  {"POST", "/object/monitor"},
	"object/selector": {"GET", "/object/selector"},
	"object_selector": {"GET", "/object/selector"},
	"object/status":   {"POST", "/object/status"},
	"object_status":   {"POST", "/object/status"},
}

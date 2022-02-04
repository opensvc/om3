package rawmux

type (
	rawToHttp struct {
		method string
		path   string
	}
)

var actionToPath = map[string]rawToHttp{
	"daemon_running": {"GET", "/daemon/running"},
	"daemon_stop":    {"POST", "/daemon/stop"},
}

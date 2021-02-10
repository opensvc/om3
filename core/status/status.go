package status

type StatusType string
var (
	UP			StatusType = "up"
	DOWN			StatusType = "down"
	WARN			StatusType = "warn"
	NA			StatusType = "n/a"
	UNDEF			StatusType = "undef"
	STDBY_UP		StatusType = "stdby up"
	STDBY_DOWN		StatusType = "stdby down"
	STDBY_UP_WITH_UP	StatusType = "up"
	STDBY_UP_WITH_DOWN	StatusType = "stdby up"
)


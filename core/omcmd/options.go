package omcmd

type (
	// OptsGlobal contains options accepted by all actions
	OptsGlobal struct {
		Color          string
		Output         string
		ObjectSelector string
		Quiet          bool
		Debug          bool
	}
)

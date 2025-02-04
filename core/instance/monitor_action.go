package instance

var (
	// MonitorActionNone: represents the no-operation monitor action
	MonitorActionNone MonitorAction = "none"

	// MonitorActionCrash represents the monitor action that will try system crash/panic
	MonitorActionCrash MonitorAction = "crash"

	// MonitorActionFreezeStop represents the monitor action that will try freeze and subsequently stop
	// the monitored instance.
	MonitorActionFreezeStop MonitorAction = "freezestop"

	// MonitorActionReboot represents the monitor action that will reboot the system.
	MonitorActionReboot MonitorAction = "reboot"

	// MonitorActionSwitch represents the monitor action that will stop instance stop to allow
	// any other cluster nodes to takeover instance.
	MonitorActionSwitch MonitorAction = "switch"
)

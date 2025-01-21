package instance

var (
	// MonitorActionNone: monitor action is disabled.
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

	// MonitorActionNoOp represents the no-operation behavior while setting the state to 'evicted'.
	// This can be useful for demonstration purposes or cases where no action is required.
	MonitorActionNoOp MonitorAction = "no-op"
)

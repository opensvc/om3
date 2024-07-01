package dsubsystem

type (
	// Dns describes the OpenSVC daemon dns thread, which is
	// responsible for janitoring and serving the cluster Dns zone. This
	// zone is dynamically populated by ip address allocated for the
	// services (frontend and backend).
	Dns struct {
		DaemonSubsystemStatus
	}
)

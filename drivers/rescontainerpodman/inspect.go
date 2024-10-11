package rescontainerpodman

type (
	InspectData struct {
		Id    string
		State struct {
			Pid     int
			Running bool
		}
		HostConfig struct {
			AutoRemove bool
		}
		NetworkSettings struct {
			SandboxKey string
		}
	}
)

func (i *InspectData) HostConfigAutoRemove() bool {
	return i.HostConfig.AutoRemove
}

func (i *InspectData) Running() bool {
	return i.State.Running
}

func (i *InspectData) PID() int {
	return i.State.Pid
}

func (i *InspectData) ID() string {
	return i.Id
}

func (i *InspectData) SandboxKey() string {
	return i.NetworkSettings.SandboxKey
}

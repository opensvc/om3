package rescontainerocibase

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

func (i *InspectData) Defined() bool {
	if i!=nil {
		return true
	}
	return false
}

func (i *InspectData) HostConfigAutoRemove() bool  {
	if i==nil {
		return false
	}
	return i.HostConfig.AutoRemove
}

func (i *InspectData) Running() bool {
	if i==nil {
		return false
	}
	return i.State.Running
}

func (i *InspectData) PID() int {
	if i==nil {
		return 0
	}
	return i.State.Pid
}

func (i *InspectData) ID() string {
	if i==nil {
		return ""
	}
	return i.Id
}

func (i *InspectData) SandboxKey() string {
	if i==nil {
		return ""
	}
	return i.NetworkSettings.SandboxKey
}

package san

const (
	FC    = "fc"
	FCOE  = "fcoe"
	ISCSI = "iscsi"
)

type (
	Path struct {
		HostBusAdapter HostBusAdapter
		TargetPort     TargetPort
	}
	TargetPort struct {
		ID string `json:"tgt_id"`
	}
	HostBusAdapter struct {
		ID   string `json:"hba_id"`
		Type string `json:"hba_type"`
		Host string `json:"host"`
	}
)

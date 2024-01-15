package limits

import "time"

type (
	T struct {
		LimitAS      int64         `json:"limit_as"`
		LimitCPU     time.Duration `json:"limit_cpu"`
		LimitCore    int64         `json:"limit_core"`
		LimitData    int64         `json:"limit_data"`
		LimitFSize   int64         `json:"limit_fsize"`
		LimitMemLock int64         `json:"limit_memlock"`
		LimitNoFile  int64         `json:"limit_nofile"`
		LimitNProc   int64         `json:"limit_nproc"`
		LimitRSS     int64         `json:"limit_rss"`
		LimitStack   int64         `json:"limit_stack"`
		LimitVMem    int64         `json:"limit_vmem"`
	}
)

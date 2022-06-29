package object

import (
	"opensvc.com/opensvc/util/sysreport"
)

type (
	// OptsNodeSysreport is the options of the Sysreport function.
	OptsNodeSysreport struct {
		OptForce
	}

	// SysreportReq structures the POST /register request body
	SysreportReq struct {
		Nodename string `json:"nodename"`
		App      string `json:"app,omitempty"`
	}

	// SysreportRes structures the POST /register response body
	SysreportRes struct {
		Data  SysreportResData `json:"data"`
		Info  string           `json:"info"`
		Error string           `json:"error"`
	}
	SysreportResData struct {
		UUID string `json:"uuid"`
	}
)

// Sysreport sends an archive of modified files the agent is configured
// to track, and the list of files deleted since the last call.
//
// The collector is in charge of versioning this information and of
// reporting on changes.
func (t Node) Sysreport(options OptsNodeSysreport) error {
	client, err := t.collectorFeedClient()
	if err != nil {
		return err
	}
	sr := sysreport.New()
	sr.SetCollectorClient(client)
	sr.SetForce(options.Force)
	return sr.Do()
}

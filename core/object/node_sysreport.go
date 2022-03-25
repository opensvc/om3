package object

import (
	"strings"

	"github.com/pkg/errors"
	"opensvc.com/opensvc/util/sysreport"
)

type (
	// OptsNodeSysreport is the options of the Sysreport function.
	OptsNodeSysreport struct {
		Global OptsGlobal
		Force  OptForce
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
	client, err := t.collectorClient()
	if err != nil {
		return err
	}
	sr := sysreport.New()
	sr.SetCollectorClient(client)
	sr.SetForce(options.Force.Force)
	return sr.Do()
}

func (t Node) sendSysreport(archive string, deletedFiles []string) error {
	client, err := t.collectorClient()
	if err != nil {
		return err
	}
	if response, err := client.Call("send_sysreport", archive, deletedFiles); err != nil {
		return err
	} else if response.Error != nil {
		return errors.Errorf("rpc: %s: %s", response.Error.Message, response.Error.Data)
	} else if response.Result != nil {
		switch v := response.Result.(type) {
		case []interface{}:
			for _, e := range v {
				s, ok := e.(string)
				if !ok {
					continue
				}
				if strings.Contains(s, "already") {
					t.Log().Info().Msg(s)
				} else {
					return errors.New(s)
				}
			}
		case string:
			return t.writeUUID(v)
		default:
			return errors.Errorf("unknown response result type: %+v", v)
		}
	} else {
		return errors.Errorf("unexpected rpc response: %+v", response)
	}
	return nil
}

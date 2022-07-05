package object

import (
	"opensvc.com/opensvc/util/sysreport"
)

// Sysreport sends an archive of modified files the agent is configured
// to track, and the list of files deleted since the last call.
//
// The collector is in charge of versioning this information and of
// reporting on changes.
func (t Node) NewSysreport() (*sysreport.T, error) {
	client, err := t.collectorFeedClient()
	if err != nil {
		return nil, err
	}
	sr := sysreport.New()
	sr.SetCollectorClient(client)
	return sr, nil
}

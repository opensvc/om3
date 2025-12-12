package object

import "github.com/opensvc/om3/v3/util/sysreport"

// Sysreport sends an archive of modified files the agent is configured
// to track, and the list of files deleted since the last call.
//
// The collector is in charge of versioning this information and of
// reporting on changes.
func (t Node) Sysreport() error {
	sr, err := t.newSysreport()
	if err != nil {
		return err
	}
	return sr.Do()
}

func (t Node) ForceSysreport() error {
	sr, err := t.newSysreport()
	if err != nil {
		return err
	}
	sr.SetForce(true)
	return sr.Do()
}

func (t Node) newSysreport() (*sysreport.T, error) {
	client, err := t.CollectorFeedClient()
	if err != nil {
		return nil, err
	}
	sr := sysreport.New()
	sr.SetCollectorClient(client)
	return sr, nil
}

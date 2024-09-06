package restaskhost

import (
	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/manifest"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/drivers/resapp"
	"github.com/opensvc/om3/drivers/restask"
)

var (
	drvID = driver.NewID(driver.GroupTask, "host")
)

func init() {
	driver.Register(drvID, New)
}

// Manifest ...
func (t T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc, naming.KindVol)
	m.Add(
		manifest.ContextObjectPath,
		manifest.ContextNodes,
		manifest.ContextObjectID,
		resapp.BaseKeywordTimeout,
		resapp.BaseKeywordStopTimeout,
		resapp.BaseKeywordSecretsEnv,
		resapp.BaseKeywordConfigsEnv,
		resapp.BaseKeywordEnv,
		resapp.BaseKeywordRetCodes,
		resapp.BaseKeywordUmask,
		resapp.UnixKeywordStopCmd,
		resapp.UnixKeywordCwd,
		resapp.UnixKeywordUser,
		resapp.UnixKeywordGroup,
		resapp.UnixKeywordLimitCPU,
		resapp.UnixKeywordLimitCore,
		resapp.UnixKeywordLimitData,
		resapp.UnixKeywordLimitFSize,
		resapp.UnixKeywordLimitMemLock,
		resapp.UnixKeywordLimitNoFile,
		resapp.UnixKeywordLimitNProc,
		resapp.UnixKeywordLimitRSS,
		resapp.UnixKeywordLimitStack,
		resapp.UnixKeywordLimitVmem,
		resapp.UnixKeywordLimitAS,
	)
	m.AddKeywords(restask.Keywords...)
	m.AddKeywords(Keywords...)
	return m
}

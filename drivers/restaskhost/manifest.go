package restaskhost

import (
	"embed"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/manifest"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/drivers/resapp"
	"github.com/opensvc/om3/v3/drivers/restask"
)

var (
	drvID = driver.NewID(driver.GroupTask, "host")

	//go:embed text
	fs embed.FS

	kws = []*keywords.Keyword{
		{
			Option:   "command",
			Attr:     "RunCmd",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/command"),
		},
	}
)

func init() {
	driver.Register(drvID, New)
}

// Manifest ...
func (t *T) Manifest() *manifest.T {
	m := manifest.New(drvID, t)
	m.Kinds.Or(naming.KindSvc, naming.KindVol)
	m.Add(
		manifest.ContextObjectPath,
		manifest.ContextNodes,
		manifest.ContextObjectID,
	)
	m.AddKeywords(
		&resapp.BaseKeywordTimeout,
		&resapp.BaseKeywordStopTimeout,
		&resapp.BaseKeywordSecretsEnv,
		&resapp.BaseKeywordConfigsEnv,
		&resapp.BaseKeywordEnv,
		&resapp.BaseKeywordRetCodes,
		&resapp.BaseKeywordUmask,
		&resapp.UnixKeywordStopCmd,
		&resapp.UnixKeywordCwd,
		&resapp.UnixKeywordUser,
		&resapp.UnixKeywordGroup,
		&resapp.UnixKeywordLimitCPU,
		&resapp.UnixKeywordLimitCore,
		&resapp.UnixKeywordLimitData,
		&resapp.UnixKeywordLimitFSize,
		&resapp.UnixKeywordLimitMemLock,
		&resapp.UnixKeywordLimitNoFile,
		&resapp.UnixKeywordLimitNProc,
		&resapp.UnixKeywordLimitRSS,
		&resapp.UnixKeywordLimitStack,
		&resapp.UnixKeywordLimitVmem,
		&resapp.UnixKeywordLimitAS,
	)
	m.AddKeywords(restask.Keywords...)
	m.AddKeywords(kws...)
	return m
}

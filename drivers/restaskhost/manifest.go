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
			Group:    GroupProcess.ID,
			Attr:     "RunCmd",
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/command"),
		},
		{
			Option:     "container",
			Group:      GroupProcess.ID,
			Attr:       "Container",
			Example:    "container#1",
			References: &keywords.References{Groups: []string{"container"}, Shape: keywords.ReferencesRID},
			Scopable:   true,
			Text:       keywords.NewText(fs, "text/kw/container"),
		},
		{
			Attr:     "StopCmd",
			Example:  "/usr/bin/rm -f /tmp/{fqdn}.{rindex}",
			Option:   "stop",
			Group:    GroupProcess.ID,
			Scopable: true,
			Text:     keywords.NewText(fs, "text/kw/stop"),
		},
	}
)

func init() {
	driver.Register(drvID, New)
	driver.SetDoc(drvID, "Runs a command on the node, as a task.")
}

func (t *T) DriverID() driver.ID {
	return drvID
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
	// The keywords borrowed from the app drivers name their groups by the
	// ids a host task declares: the timeouts and status of every task, its
	// own process, and the environment and limits of the app drivers.
	m.AddKeywords(
		&resapp.BaseKeywordTimeout,
		&resapp.BaseKeywordStopTimeout,
		&resapp.BaseKeywordSecretsEnv,
		&resapp.BaseKeywordConfigsEnv,
		&resapp.BaseKeywordEnv,
		&resapp.BaseKeywordRetCodes,
		&resapp.BaseKeywordUmask,
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
	m.AddGroups(GroupProcess, resapp.GroupEnvironment, resapp.GroupLimits)
	m.AddGroups(restask.Groups...)
	return m
}

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
	m.AddKeywords(grouped(restask.GroupTimeouts,
		&resapp.BaseKeywordTimeout,
		&resapp.BaseKeywordStopTimeout,
	)...)
	m.AddKeywords(grouped(GroupEnvironment,
		&resapp.BaseKeywordSecretsEnv,
		&resapp.BaseKeywordConfigsEnv,
		&resapp.BaseKeywordEnv,
	)...)
	m.AddKeywords(grouped(restask.GroupStatus,
		&resapp.BaseKeywordRetCodes,
	)...)
	m.AddKeywords(grouped(GroupProcess,
		&resapp.BaseKeywordUmask,
		&resapp.UnixKeywordCwd,
		&resapp.UnixKeywordUser,
		&resapp.UnixKeywordGroup,
	)...)
	m.AddKeywords(grouped(GroupLimits,
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
	)...)
	m.AddKeywords(restask.Keywords...)
	m.AddKeywords(kws...)
	m.AddGroups(GroupProcess, GroupEnvironment, GroupLimits)
	m.AddGroups(restask.Groups...)
	return m
}

// grouped returns copies of keywords the app drivers share, in the group of
// a host task they are in: the app drivers group none of their keywords yet,
// and a keyword naming a group its driver does not declare is in none.
func grouped(group keywords.Group, l ...*keywords.Keyword) []*keywords.Keyword {
	copies := make([]*keywords.Keyword, len(l))
	for i, kw := range l {
		c := *kw
		c.Group = group.ID
		copies[i] = &c
	}
	return copies
}

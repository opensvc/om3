package restaskacme

import (
	"github.com/opensvc/om3/v3/core/keywords"
)

// GroupCertificates holds the certificates the task renews: the secs it
// names, the domains it reads from files and the secs it creates for them,
// the ACME directory and account of these, and where the http-01 challenge
// is answered from.
//
// A task renews the secs it names, the secs of the domains it reads, or
// both: the two strategies exclude nothing of each other, as a run renews
// both, with an account at a directory they share. The group is required,
// as a run with neither fails with "no sec to renew", and the domains are
// read with both domains_from and domains_to, which checkDomains refuses
// alone: the validation says so when the configuration is written.
var GroupCertificates = keywords.Group{
	ID:       "certificates",
	Name:     "Certificates",
	Text:     keywords.NewText(fs, "text/group/certificates"),
	Order:    20,
	Required: true,
	Strategies: []keywords.Strategy{
		{
			ID:   "secs",
			Name: "Secs named",
			Text: keywords.NewText(fs, "text/strategy/certificates/secs"),
			Uses: []string{"secs"},
		},
		{
			ID:       "domains",
			Name:     "Domains read from files",
			Text:     keywords.NewText(fs, "text/strategy/certificates/domains"),
			Uses:     []string{"domains_from", "domains_to"},
			Requires: []string{"domains_from", "domains_to"},
		},
	},
}

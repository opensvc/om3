package restaskacme

import (
	"github.com/opensvc/om3/v3/core/keywords"
)

// GroupCertificates holds the certificates the task renews: the secs it
// names, the domains it reads from files and the secs it creates for them,
// the ACME directory and account of these, and where the http-01 challenge
// is answered from.
//
// The secs named and the domains read are no strategies: a run renews both,
// and the account at a directory they share. The ones of the domains are
// read with both domains_from and domains_to, which checkDomains refuses
// alone, and a run with neither the secs nor the domains fails.
var GroupCertificates = keywords.Group{
	ID:    "certificates",
	Name:  "Certificates",
	Text:  keywords.NewText(fs, "text/group/certificates"),
	Order: 20,
}

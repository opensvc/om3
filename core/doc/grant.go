package doc

import (
	"fmt"
	"strings"

	"github.com/opensvc/om3/v3/core/keyoprbac"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/daemon/rbac"
)

// AddObjectKeywordGrants says, on each keyword of an object of kind, the
// grant setting it through the api needs, as the keyword operation policy
// says it: the grant, the sentence the documentation shows, and whether the
// grant is needed whatever the value.
//
// A keyword of a driver, as the rcmd of a kvm container, also needs the grant
// choosing the driver does: a user who may not set the type of the section
// may not set any keyword of it, so a keyword whose drivers are all refused
// to that user is reported needing the grant whatever its value. A type
// refused or not by the rest of the section, as the ip ones, is no such
// driver: the keywords of a netns address are a namespace administrator's
// when the address is drawn from a cluster network.
func AddObjectKeywordGrants(items api.KeywordDefinitionItems, kind naming.Kind) {
	for i := range items {
		item := &items[i]
		item.Grant, item.GrantDoc, item.GrantAlways = keywordGrant(kind, item.Section, item.Option, item.Types)
	}
}

// KeywordGrantDoc returns the sentence the documentation of a keyword of an
// object of kind shows about the grant setting it needs, or an empty string
// when any user granted on the namespace may set it: the grantDoc of the api.
// types are the drivers the keyword belongs to.
func KeywordGrantDoc(kind naming.Kind, section, option string, types []string) string {
	_, doc, _ := keywordGrant(kind, section, option, types)
	return doc
}

func keywordGrant(kind naming.Kind, section, option string, types []string) (string, string, bool) {
	var (
		grant  string
		doc    string
		always bool
	)
	if rule, ok := keyoprbac.Lookup(kind, section, option); ok {
		grant = string(rule.Grant)
		doc = keyoprbac.Doc(kind, section, option)
		always = rule.Denies == nil && len(rule.Values) == 0
	}
	if always || len(types) == 0 || option == "type" {
		return grant, doc, always
	}
	typeRule, ok := keyoprbac.Lookup(kind, section, "type")
	if !ok || typeRule.Denies != nil {
		// A type allowed or not by the rest of the section, as a netns
		// address drawn from a cluster network, is not one this can tell
		// refused: the keyword is said by its own rule, and the type by
		// the sentence of its keyword.
		return grant, doc, always
	}
	for _, typ := range types {
		if keyoprbac.DeniedByDriver(rbac.Grants{}, kind, section, option, typ, nil) == nil {
			return grant, doc, always
		}
	}
	s := fmt.Sprintf("A keyword of the %s driver, which needs the %s grant to choose.", strings.Join(types, ", "), typeRule.Grant)
	if len(types) > 1 {
		s = fmt.Sprintf("A keyword of the %s drivers, which need the %s grant to choose.", strings.Join(types, ", "), typeRule.Grant)
	}
	return string(typeRule.Grant), s, true
}

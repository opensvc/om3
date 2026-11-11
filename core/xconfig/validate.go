package xconfig

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/resourceid"
	"github.com/opensvc/om3/v3/core/resourcereqs"
	"github.com/opensvc/om3/v3/util/capabilities"
	"github.com/opensvc/om3/v3/util/converters"
	"github.com/opensvc/om3/v3/util/key"
)

type (
	Alerts []Alert
	Alert  struct {
		Path    naming.Path `json:"path"`
		Level   AlertLevel  `json:"level"`
		Kind    AlertKind   `json:"kind"`
		Key     key.T       `json:"key"`
		Driver  driver.ID   `json:"driver"`
		Comment string      `json:"comment"`
	}
	AlertKind  int
	AlertLevel int
)

const (
	alertLevelWarn AlertLevel = iota
	alertLevelError

	alertKindScoping AlertKind = iota
	alertKindUnknown
	alertKindUnknownDriver
	alertKindUnknownSection
	alertKindEval
	alertKindCandidates
	alertKindDeprecated
	alertKindDeprecatedValue
	alertKindMoved
	alertKindCapabilities
	alertKindInvalidSection
	alertKindConfigure
	alertKindKeySource
	alertKindStrategies
	alertKindUnknownResource
	alertKindReferenceGroup
)

var (
	alertLevelWarnStr  = "warning"
	alertLevelErrorStr = "error"
	alertLevelNames    = map[AlertLevel]string{
		alertLevelWarn:  alertLevelWarnStr,
		alertLevelError: alertLevelErrorStr,
	}
	alertLevelFromNames = map[string]AlertLevel{
		alertLevelWarnStr:  alertLevelWarn,
		alertLevelErrorStr: alertLevelError,
	}
	alertKindUnknownDriverStr   = "unknown driver"
	alertKindUnknownSectionStr  = "unknown section"
	alertKindScopingStr         = "unscopable keyword"
	alertKindUnknownStr         = "unknown keyword"
	alertKindEvalStr            = "evaluation error"
	alertKindCandidatesStr      = "unsupported value"
	alertKindDeprecatedStr      = "deprecated keyword"
	alertKindDeprecatedValueStr = "deprecated value"
	alertKindMovedStr           = "moved keyword"
	alertKindCapabilitiesStr    = "unusable driver on this node"
	alertKindInvalidSectionStr  = "invalid section name"
	alertKindConfigureStr       = "resource configuration error"
	alertKindKeySourceStr       = "key source error"
	alertKindStrategiesStr      = "conflicting strategies"
	alertKindUnknownResourceStr = "unknown resource"
	alertKindReferenceGroupStr  = "unsupported resource group"
	alertKindNames              = map[AlertKind]string{
		alertKindScoping:         alertKindScopingStr,
		alertKindUnknown:         alertKindUnknownStr,
		alertKindUnknownDriver:   alertKindUnknownDriverStr,
		alertKindUnknownSection:  alertKindUnknownSectionStr,
		alertKindEval:            alertKindEvalStr,
		alertKindCandidates:      alertKindCandidatesStr,
		alertKindDeprecated:      alertKindDeprecatedStr,
		alertKindDeprecatedValue: alertKindDeprecatedValueStr,
		alertKindMoved:           alertKindMovedStr,
		alertKindCapabilities:    alertKindCapabilitiesStr,
		alertKindInvalidSection:  alertKindInvalidSectionStr,
		alertKindConfigure:       alertKindConfigureStr,
		alertKindKeySource:       alertKindKeySourceStr,
		alertKindStrategies:      alertKindStrategiesStr,
		alertKindUnknownResource: alertKindUnknownResourceStr,
		alertKindReferenceGroup:  alertKindReferenceGroupStr,
	}
	alertKindFromNames = map[string]AlertKind{
		alertKindScopingStr:         alertKindScoping,
		alertKindUnknownStr:         alertKindUnknown,
		alertKindUnknownDriverStr:   alertKindUnknownDriver,
		alertKindUnknownSectionStr:  alertKindUnknownSection,
		alertKindEvalStr:            alertKindEval,
		alertKindCandidatesStr:      alertKindCandidates,
		alertKindDeprecatedStr:      alertKindDeprecated,
		alertKindDeprecatedValueStr: alertKindDeprecatedValue,
		alertKindMovedStr:           alertKindMoved,
		alertKindCapabilitiesStr:    alertKindCapabilities,
		alertKindInvalidSectionStr:  alertKindInvalidSection,
		alertKindConfigureStr:       alertKindConfigure,
		alertKindKeySourceStr:       alertKindKeySource,
		alertKindStrategiesStr:      alertKindStrategies,
		alertKindUnknownResourceStr: alertKindUnknownResource,
		alertKindReferenceGroupStr:  alertKindReferenceGroup,
	}
)

func (t T) NewAlertScoping(k key.T, did driver.ID) Alert {
	return Alert{
		Path:   t.Path,
		Kind:   alertKindScoping,
		Level:  alertLevelError,
		Key:    k,
		Driver: did,
	}
}

func (t T) NewAlertUnknownDriver(k key.T, did driver.ID) Alert {
	return Alert{
		Path:   t.Path,
		Kind:   alertKindUnknownDriver,
		Level:  alertLevelWarn,
		Key:    k,
		Driver: did,
	}
}

// NewAlertUnsupportedSection says a section is a resource of a driver group
// this agent does not have, in the words the instance status says it: the
// status warns of a resource it ignores, and the validation of the
// configuration is where what to fix is looked for.
//
// The section has no driver to name in the alert, so the group it names is
// in the comment.
func (t T) NewAlertUnsupportedSection(section, sectionType string) Alert {
	drv, _, _ := strings.Cut(section, "#")
	if sectionType != "" {
		drv += "." + sectionType
	}
	return Alert{
		Path:    t.Path,
		Kind:    alertKindUnknownDriver,
		Level:   alertLevelWarn,
		Key:     key.T{Section: section},
		Comment: fmt.Sprintf("the %s driver is not supported by this agent: the resource is ignored", drv),
	}
}

// NewAlertUnknownSection says a section is neither a resource nor a section
// an object reads, so nothing uses it: the status has no resource to show it
// by, and the validation is the only place it is reported.
// NewAlertInvalidSection says a section is named with a character no resource
// id is made of. Its resource is not configured, and it is an error, so a write
// naming it is refused rather than ignored.
func (t T) NewAlertInvalidSection(section string, err error) Alert {
	return Alert{
		Path:    t.Path,
		Kind:    alertKindInvalidSection,
		Level:   alertLevelError,
		Key:     key.T{Section: section},
		Comment: err.Error(),
	}
}

// NewAlertConfigure says the resource of section refused its configuration
// when the object loaded it, by a check its driver makes beyond the keywords,
// as a network no cni configuration and no cluster network names: the
// resource is not configured, its status is undef, and its start is refused.
// A disabled resource is never started, so it is a warning.
func (t T) NewAlertConfigure(section string, did driver.ID, err error, disabled bool) Alert {
	level := alertLevelError
	if disabled {
		level = alertLevelWarn
	}
	return Alert{
		Path:    t.Path,
		Kind:    alertKindConfigure,
		Level:   level,
		Key:     key.T{Section: section},
		Driver:  did,
		Comment: err.Error(),
	}
}

// NewAlertKeySource says a keyword names a key of a datastore with a source
// the object can not seed it from: a key another keyword seeds from another
// source, or a key of a datastore of another namespace.
func (t T) NewAlertKeySource(k key.T, did driver.ID, comment string) Alert {
	return Alert{
		Path:    t.Path,
		Kind:    alertKindKeySource,
		Level:   alertLevelError,
		Key:     k,
		Driver:  did,
		Comment: comment,
	}
}

func (t T) NewAlertUnknownSection(section string) Alert {
	return Alert{
		Path:    t.Path,
		Kind:    alertKindUnknownSection,
		Level:   alertLevelWarn,
		Key:     key.T{Section: section},
		Comment: "the section is no resource and no section an object reads: it is ignored",
	}
}

func (t T) NewAlertUnknown(k key.T, did driver.ID, comment string) Alert {
	return Alert{
		Path:    t.Path,
		Kind:    alertKindUnknown,
		Level:   alertLevelWarn,
		Key:     k,
		Driver:  did,
		Comment: comment,
	}
}

func (t T) NewAlertCandidates(k key.T, did driver.ID, comment string) Alert {
	return Alert{
		Path:    t.Path,
		Kind:    alertKindCandidates,
		Level:   alertLevelError,
		Key:     k,
		Driver:  did,
		Comment: comment,
	}
}

func (t T) NewAlertEval(k key.T, did driver.ID, comment string) Alert {
	return Alert{
		Path:    t.Path,
		Kind:    alertKindEval,
		Level:   alertLevelError,
		Key:     k,
		Driver:  did,
		Comment: comment,
	}
}

// NewAlertDeprecatedValue says a keyword holds a value om still accepts and no
// longer recommends, and what to write instead.
//
// It is not the keyword that is deprecated, which is what the alert beside it
// says: the keyword is the one to use, and one of the forms it takes is on its
// way out.
func (t T) NewAlertDeprecatedValue(k key.T, did driver.ID, comment string) Alert {
	return Alert{
		Path:    t.Path,
		Kind:    alertKindDeprecatedValue,
		Level:   alertLevelWarn,
		Key:     k,
		Driver:  did,
		Comment: comment,
	}
}

// newAlertGone is what to say of a keyword the driver does not declare: where
// what it asked for is written now, when om knows, and that nothing knows it
// otherwise.
func (t T) newAlertGone(k key.T, did driver.ID, err error) Alert {
	if text, ok := keywords.Moved(did, k.BaseOption()); ok {
		return t.NewAlertMoved(k, did, text)
	}
	return t.NewAlertUnknown(k, did, err.Error())
}

// NewAlertMoved says a keyword is one a driver no longer reads, and what to
// write for om to do again what it did.
//
// It is not an unknown keyword, which is what the alert beside it says of a
// keyword nothing has ever heard of: this one was read once, and what it
// asked for is written elsewhere now.
func (t T) NewAlertMoved(k key.T, did driver.ID, comment string) Alert {
	return Alert{
		Path:    t.Path,
		Kind:    alertKindMoved,
		Level:   alertLevelWarn,
		Key:     k,
		Driver:  did,
		Comment: comment,
	}
}

// NewAlertStrategies says a section sets keywords of two strategies of a
// group of keywords that exclude each other, as an address and the network
// to draw one from: one of the two is not used as its writer expects.
//
// It is a warning, the resource doing with the keywords what its driver
// does.
func (t T) NewAlertStrategies(k key.T, did driver.ID, comment string) Alert {
	return Alert{
		Path:    t.Path,
		Kind:    alertKindStrategies,
		Level:   alertLevelWarn,
		Key:     k,
		Driver:  did,
		Comment: comment,
	}
}

// NewAlertUnknownResource says a keyword names a resource the configuration
// has no section of, as a netns naming a container not configured: the
// resource using it fails its action.
//
// It is a warning, as a configuration may be written in steps, the resource
// named added after the one naming it.
func (t T) NewAlertUnknownResource(k key.T, did driver.ID, comment string) Alert {
	return Alert{
		Path:    t.Path,
		Kind:    alertKindUnknownResource,
		Level:   alertLevelWarn,
		Key:     k,
		Driver:  did,
		Comment: comment,
	}
}

// NewAlertReferenceGroup says a keyword names a resource of a driver group
// it does not take, as a netns naming a fs: no section of that name can ever
// be what the keyword asks for.
func (t T) NewAlertReferenceGroup(k key.T, did driver.ID, comment string) Alert {
	return Alert{
		Path:    t.Path,
		Kind:    alertKindReferenceGroup,
		Level:   alertLevelError,
		Key:     k,
		Driver:  did,
		Comment: comment,
	}
}

func (t T) NewAlertDeprecated(k key.T, did driver.ID, release, replacedBy string) Alert {
	comment := fmt.Sprintf("since %s", release)
	if replacedBy != "" {
		comment += fmt.Sprintf(", replaced by %s", replacedBy)
	}
	return Alert{
		Path:    t.Path,
		Kind:    alertKindDeprecated,
		Level:   alertLevelWarn,
		Key:     k,
		Driver:  did,
		Comment: comment,
	}
}

func (t T) NewAlertCapabilities(k key.T, did driver.ID) Alert {
	return Alert{
		Path:   t.Path,
		Kind:   alertKindCapabilities,
		Level:  alertLevelWarn,
		Key:    k,
		Driver: did,
	}
}

func (t AlertKind) String() string {
	if s, ok := alertKindNames[t]; ok {
		return s
	} else {
		return ""
	}
}

func (t AlertLevel) String() string {
	if s, ok := alertLevelNames[t]; ok {
		return s
	} else {
		return ""
	}
}

func (t AlertLevel) MarshalJSON() ([]byte, error) {
	if s, ok := alertLevelNames[t]; ok {
		return json.Marshal(s)
	} else {
		return nil, fmt.Errorf("unknown validate alert level: %d", t)
	}
}

// UnmarshalJSON unmashals a quoted json string to the enum value
func (t *AlertLevel) UnmarshalJSON(b []byte) error {
	var j string
	err := json.Unmarshal(b, &j)
	if err != nil {
		return err
	}
	*t, _ = alertLevelFromNames[j]
	return nil
}

func (t AlertKind) MarshalJSON() ([]byte, error) {
	if s, ok := alertKindNames[t]; ok {
		return json.Marshal(s)
	} else {
		return nil, fmt.Errorf("unknown validate alert kind: %d", t)
	}
}

// UnmarshalJSON unmashals a quoted json string to the enum value
func (t *AlertKind) UnmarshalJSON(b []byte) error {
	var j string
	err := json.Unmarshal(b, &j)
	if err != nil {
		return err
	}
	*t, _ = alertKindFromNames[j]
	return nil
}

func (t Alerts) Error() string {
	return t.String()
}

func (t Alerts) String() string {
	l := make([]string, len(t))
	for i, alert := range t {
		l[i] = alert.String()
	}
	return strings.Join(l, "\n")
}

func (t Alerts) StringWithoutMeta() string {
	l := make([]string, len(t))
	for i, alert := range t {
		l[i] = alert.StringWithoutMeta()
	}
	return strings.Join(l, "\n")
}

// Errors are the alerts that make a configuration invalid, as opposed to the
// warnings that only have something to say about it.
func (t Alerts) Errors() Alerts {
	l := make(Alerts, 0, len(t))
	for _, alert := range t {
		if alert.Level == alertLevelError {
			l = append(l, alert)
		}
	}
	return l
}

// Warns are the alerts that only have something to say about a configuration,
// as opposed to the errors that make it invalid.
func (t Alerts) Warns() Alerts {
	l := make(Alerts, 0, len(t))
	for _, alert := range t {
		if alert.Level == alertLevelWarn {
			l = append(l, alert)
		}
	}
	return l
}

func (t Alerts) HasError() bool {
	return t.has(alertLevelError)
}

func (t Alerts) HasWarn() bool {
	return t.has(alertLevelWarn)
}

func (t Alerts) has(lvl AlertLevel) bool {
	for _, alert := range t {
		if alert.Level == lvl {
			return true
		}
	}
	return false
}

func (t T) Validate() (Alerts, error) {
	alerts := make(Alerts, 0)
	// sections are the names of the sections of the configuration, the
	// resources a keyword value may name.
	sections := make(map[string]bool)
	for _, s := range t.file.Sections() {
		sections[s.Name()] = true
	}
	for _, s := range t.file.Sections() {
		var did driver.ID
		section := s.Name()
		sectionType := t.GetString(key.New(section, "type"))
		if !strings.HasPrefix(section, "subset#") {
			if err := resourceid.CheckName(section); err != nil {
				alerts = append(alerts, t.NewAlertInvalidSection(section, err))
				continue
			}
		}
		if rid, err := resourceid.Parse(section); err == nil {
			if rid.DriverGroup() == driver.GroupUnknown && t.hasResourceSections() {
				if rid.IsIndexed() {
					alerts = append(alerts, t.NewAlertUnsupportedSection(section, sectionType))
				} else {
					alerts = append(alerts, t.NewAlertUnknownSection(section))
				}
				continue
			}
			did = driver.NewID(rid.DriverGroup(), sectionType)
			if did.Name != "" {
				if sectionType == "" {
					sectionType = did.Name
				}
				if rid.DriverGroup() == driver.GroupVolume {
					poolDid := driver.NewID(driver.GroupPool, sectionType)
					if !driver.Exists(poolDid) {
						alerts = append(alerts, t.NewAlertUnknownDriver(key.T{Section: section}, poolDid))
						continue
					}
				} else if !driver.Exists(did) {
					alerts = append(alerts, t.NewAlertUnknownDriver(key.T{Section: section}, did))
					continue
				}
				if !capabilities.Has(did.Cap()) {
					alerts = append(alerts, t.NewAlertCapabilities(key.T{Section: section}, did))
					continue
				}
			}
		}
		// written are the options of the keywords the section sets, in
		// any scope, by their name or an alias, each with the name it is
		// written by.
		written := make(map[string]string)
		for option, raw := range s.KeysHash() {
			k, err := key.ParseStrict(section + "." + option)
			if err != nil {
				alerts = append(alerts, t.NewAlertUnknown(k, did, err.Error()))
				continue
			} else if k.BaseOption() == "type" {
				continue
			}
			kw, err := getKeyword(k, sectionType, t.Referrer)
			if err != nil {
				if k.Section == "DEFAULT" {
					// if a DEFAULT.<option> is not declared as a keyword, don't
					// raise an issue if a keyword exists in drivers, as it
					// may be the default value for this keyword.
					relaxedKey := key.T{Section: "*", Option: k.Option}
					var err1 error
					kw, err1 = getKeyword(relaxedKey, sectionType, t.Referrer)
					if err1 != nil {
						alerts = append(alerts, t.newAlertGone(k, did, err))
						continue
					}
				} else {
					alerts = append(alerts, t.newAlertGone(k, did, err))
					continue
				}
			}
			if _, ok := written[kw.Option]; !ok || k.BaseOption() == kw.Option {
				written[kw.Option] = k.BaseOption()
			}
			if strings.Contains(k.Option, "@") && !kw.Scopable {
				alerts = append(alerts, t.NewAlertScoping(k, did))
			}
			v, err := t.evalStringAs(k, kw, "", false, newDereferenceTrace())
			if err != nil {
				alerts = append(alerts, t.NewAlertEval(k, did, fmt.Sprint(err)))
				continue
			}
			refused := false
			if kw.Validate != nil {
				if err := t.validateLine(k, kw, raw); err != nil {
					alerts = append(alerts, t.NewAlertCandidates(k, did, err.Error()))
					refused = true
				}
			}
			if kw.References != nil && !refused {
				alerts = append(alerts, t.referenceAlerts(k, kw, did, raw, sections)...)
			}
			if kw.Deprecated != "" {
				alerts = append(alerts, t.NewAlertDeprecated(k, did, kw.Deprecated, kw.ReplacedBy))
			}
			if kw.DeprecatedValue != "" && v != "" {
				if ok, err := regexp.MatchString(kw.DeprecatedValue, v); err == nil && ok {
					alerts = append(alerts, t.NewAlertDeprecatedValue(k, did, kw.DeprecatedValueText))
				}
			}
			if len(kw.Candidates) > 0 {
				// A keyword naming resources takes a resource id
				// besides its candidates, which the references check.
				accepted := func(s string) bool {
					return slices.Contains(kw.Candidates, s) || kw.References != nil && isRIDShape(s)
				}
				switch kw.Converter {
				case nil, converters.String, converters.Int:
					if !accepted(v) {
						alerts = append(alerts, t.NewAlertCandidates(k, did, v))
					}
				case converters.List:
					for _, e := range strings.Fields(v) {
						if !accepted(e) {
							alerts = append(alerts, t.NewAlertCandidates(k, did, e))
						}
					}
				}
			}
		}
		if did.Group.IsResource() {
			alerts = append(alerts, t.strategyAlerts(section, did, written)...)
		}
	}
	if i, ok := t.Referrer.(sectionsValidator); ok {
		alerts = append(alerts, i.ValidateSections(&t)...)
	}
	return alerts, nil
}

// sectionsValidator is a referrer checking the sections of its configuration
// against one another, which no keyword can do of its value alone: two
// networks of the cluster handing out the same addresses.
type sectionsValidator interface {
	ValidateSections(*T) Alerts
}

// keywordGrouper is a referrer knowing the groups of the keywords of a
// resource driver, whose strategies say which keywords are ways to configure
// a group that exclude each other.
type keywordGrouper interface {
	DriverKeywordGroups(driver.ID) []keywords.Group
}

// strategyAlerts warns of the keywords a resource section sets that belong to
// strategies excluding each other: a keyword a strategy excludes set next to
// one the strategy uses, as an address and the network to draw one from.
//
// The keywords judged are the ones the section writes, in any scope: a value
// inherited from the DEFAULT section is not the resource's choice. A pair of
// keywords is warned of once, whichever of its two strategies finds it.
//
// A keyword a strategy uses is set next to the keyword of another strategy
// when that other strategy does not exclude it: the netmask of a static
// address also overrides the one of the network an address is drawn from.
func (t T) strategyAlerts(section string, did driver.ID, written map[string]string) Alerts {
	i, ok := t.Referrer.(keywordGrouper)
	if !ok {
		return nil
	}
	var alerts Alerts
	reported := make(map[[2]string]bool)
	// active says the selector values a strategy is chosen by hold in the
	// section, as quiesce=pause: a strategy chosen by none always does.
	active := func(s keywords.Strategy) bool {
		for _, w := range s.When {
			w.Key = key.New(section, w.Key.Option)
			if !t.HasKeyMatchingOp(w) {
				return false
			}
		}
		return true
	}
	for _, group := range i.DriverKeywordGroups(did) {
		for _, strategy := range group.Strategies {
			if !active(strategy) {
				continue
			}
			for _, used := range strategy.Uses {
				usedAs, ok := written[used]
				if !ok {
					continue
				}
				for _, excluded := range strategy.Excludes {
					excludedAs, ok := written[excluded]
					if !ok {
						continue
					}
					// A strategy using the keyword excluded, not chosen
					// in this section, makes it a keyword the strategy
					// chosen does not read.
					other, hasOther := group.StrategyUsing(excluded)
					if hasOther && !active(other) {
						hasOther = false
					}
					if hasOther && !slices.Contains(other.Excludes, used) {
						continue
					}
					pair := [2]string{used, excluded}
					if pair[0] > pair[1] {
						pair[0], pair[1] = pair[1], pair[0]
					}
					if reported[pair] {
						continue
					}
					reported[pair] = true
					var comment string
					if hasOther {
						comment = fmt.Sprintf("%s and %s are both set, but %s is the %s %s strategy and %s the %s one", usedAs, excludedAs, usedAs, strategy.ID, group.ID, excludedAs, other.ID)
					} else {
						comment = fmt.Sprintf("%s and %s are both set, but the %s %s strategy uses %s and excludes %s", usedAs, excludedAs, strategy.ID, group.ID, usedAs, excludedAs)
					}
					alerts = append(alerts, t.NewAlertStrategies(key.T{Section: section, Option: excludedAs}, did, comment))
				}
			}
		}
	}
	return alerts
}

// hasResourceSections reports whether the sections of the configuration are
// resources, so that one naming no driver group is a resource ignored rather
// than a section of another purpose.
func (t T) hasResourceSections() bool {
	i, ok := t.Referrer.(resourceSectioner)
	return ok && i.HasResourceSections()
}

// referenceAlerts says of the resources a line of the configuration names,
// by a keyword whose value names resources, the ones of a driver group the
// keyword does not take, an error, and the ones no section of the
// configuration is, a warning.
//
// The value read is the one of the line, as validateLine does. A value
// naming no resource, empty, a candidate or a value the engine takes as it
// is, as the podman userns modes, is not checked: a resource id has a '#'.
//
// The DEFAULT section is not the section of a resource: the keyword a value
// set there is inherited by is the one of each resource not setting it, of
// any driver, so the groups are not judged there, only that the resources
// named exist.
func (t *T) referenceAlerts(k key.T, kw *keywords.Keyword, did driver.ID, raw string, sections map[string]bool) Alerts {
	v, ok, err := t.lineValue(k, kw, raw)
	if err != nil || !ok {
		return nil
	}
	var alerts Alerts
	for _, rid := range referencedRIDs(*kw.References, v, kw.Candidates) {
		group, _, _ := strings.Cut(rid, "#")
		if k.Section != "DEFAULT" && !kw.References.HasGroup(group) {
			comment := fmt.Sprintf("%s is no %s resource", rid, kw.References.GroupsText())
			alerts = append(alerts, t.NewAlertReferenceGroup(k, did, comment))
			continue
		}
		if !sections[rid] {
			comment := fmt.Sprintf("%s names no section of the configuration", rid)
			alerts = append(alerts, t.NewAlertUnknownResource(k, did, comment))
		}
	}
	return alerts
}

// referencedRIDs returns the ids of the resources a value names, read in the
// shape of the references, leaving out the candidates of the keyword and the
// words that are no resource id, having no '#'.
//
// None of the shapes takes a pattern: the resources named are looked up by
// their id, so a pattern names a section only if one is named so.
func referencedRIDs(refs keywords.References, value string, candidates []string) []string {
	var words []string
	switch refs.Shape {
	case keywords.ReferencesRID:
		words = []string{strings.TrimSpace(value)}
	case keywords.ReferencesRIDs:
		words = strings.Fields(value)
	case keywords.ReferencesRequires:
		words = resourcereqs.RIDs(value)
	}
	l := make([]string, 0, len(words))
	for _, word := range words {
		if !strings.Contains(word, "#") || slices.Contains(candidates, word) {
			continue
		}
		l = append(l, word)
	}
	return l
}

// isRIDShape reports whether s is written as the id of a resource,
// <group>#<name>, which a keyword naming resources takes besides its
// candidates.
func isRIDShape(s string) bool {
	group, name, ok := strings.Cut(s, "#")
	return ok && group != "" && name != "" && resourceid.CheckName(s) == nil
}

// lineValue returns the value a line of the configuration holds, evaluated,
// and false when it names something not built yet, which is not the value
// it will be.
//
// It is the value of the line, and not the one the key evaluates to: a
// scoped line is the value of the nodes it names, which the local node may
// not be, and a line the local node does not see is still what those nodes
// will use. It is evaluated as the node the scope names, if it names one, so
// the references it holds are the ones that node resolves.
func (t *T) lineValue(k key.T, kw *keywords.Keyword, raw string) (string, bool, error) {
	var impersonate string
	switch scope := k.Scope(); scope {
	case "", "nodes", "drpnodes", "encapnodes":
	default:
		impersonate = scope
	}
	v, err := t.replaceReferences(raw, k.Section, impersonate, false, newDereferenceTrace())
	if isPostponedRef(err) {
		return "", false, nil
	} else if err != nil {
		return "", false, err
	}
	if v, err = evalArithmetic(v, kw); err != nil {
		return "", false, err
	}
	return v, true, nil
}

// validateLine says whether the value a line of the configuration holds is
// one its keyword takes, the value lineValue reads. A value naming something
// not built yet is not the value it will be, so it is checked when it is.
func (t *T) validateLine(k key.T, kw *keywords.Keyword, raw string) error {
	v, ok, err := t.lineValue(k, kw, raw)
	if err != nil {
		return err
	}
	if !ok || v == "" {
		return nil
	}
	return kw.Validate(v)
}

func ValidateFile(p string, ref Referrer) (Alerts, error) {
	cfg, err := NewObject(p, p)
	if err != nil {
		return nil, err
	}
	cfg.Referrer = ref
	return cfg.Validate()
}

func (t Alert) String() string {
	if t.Path.IsZero() {
		return fmt.Sprintf("%s: %s", t.Level, t.StringWithoutMeta())
	} else {
		return fmt.Sprintf("%s: %s: %s", t.Path, t.Level, t.StringWithoutMeta())
	}
}

func (t Alert) StringWithoutMeta() string {
	buff := fmt.Sprintf("key %s: %s", t.Key, t.Kind)
	if t.Comment != "" {
		buff += ": " + t.Comment
	}
	return buff
}

func ValidateReferrer(dst string, referrer Referrer) error {
	alerts, err := ValidateFile(dst, referrer)
	if err != nil {
		return err
	}
	if alerts.HasError() {
		return fmt.Errorf("%s", alerts)
	}
	return nil
}

func (t Alerts) GetItems() any {
	return t
}

func (t Alert) Icon() string {
	switch t.Level {
	case alertLevelWarn:
		return "warning"
	case alertLevelError:
		return "error"
	default:
		return "info"
	}
}

func (t Alert) Unstructured() map[string]any {
	return map[string]any{
		"icon":    t.Icon(),
		"path":    t.Path.String(),
		"level":   t.Level.String(),
		"key":     t.Key.String(),
		"kind":    t.Kind.String(),
		"driver":  t.Driver.String(),
		"comment": t.Comment,
	}
}

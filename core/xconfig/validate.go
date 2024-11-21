package xconfig

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/core/resourceid"
	"github.com/opensvc/om3/util/capabilities"
	"github.com/opensvc/om3/util/converters"
	"github.com/opensvc/om3/util/key"
	"github.com/opensvc/om3/util/render/tree"
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
	alertKindEval
	alertKindCandidates
	alertKindDeprecated
	alertKindCapabilities
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
	alertKindUnknownDriverStr = "unknown driver"
	alertKindScopingStr       = "unscopable keyword"
	alertKindUnknownStr       = "unknown keyword"
	alertKindEvalStr          = "evaluation error"
	alertKindCandidatesStr    = "unsupported value"
	alertKindDeprecatedStr    = "deprecated keyword"
	alertKindCapabilitiesStr  = "unusable driver on this node"
	alertKindNames            = map[AlertKind]string{
		alertKindScoping:       alertKindScopingStr,
		alertKindUnknown:       alertKindUnknownStr,
		alertKindUnknownDriver: alertKindUnknownDriverStr,
		alertKindEval:          alertKindEvalStr,
		alertKindCandidates:    alertKindCandidatesStr,
		alertKindDeprecated:    alertKindDeprecatedStr,
		alertKindCapabilities:  alertKindCapabilitiesStr,
	}
	alertKindFromNames = map[string]AlertKind{
		alertKindScopingStr:       alertKindScoping,
		alertKindUnknownStr:       alertKindUnknown,
		alertKindUnknownDriverStr: alertKindUnknownDriver,
		alertKindEvalStr:          alertKindEval,
		alertKindCandidatesStr:    alertKindCandidates,
		alertKindDeprecatedStr:    alertKindDeprecated,
		alertKindCapabilitiesStr:  alertKindCapabilities,
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

func (t T) NewAlertUnknown(k key.T, did driver.ID) Alert {
	return Alert{
		Path:   t.Path,
		Kind:   alertKindUnknown,
		Level:  alertLevelWarn,
		Key:    k,
		Driver: did,
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

func (t T) NewAlertDeprecated(k key.T, did driver.ID, release, replacedBy string) Alert {
	comment := fmt.Sprintf("since %s", release)
	if replacedBy != "" {
		comment += fmt.Sprintf("replaced by %s", replacedBy)
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

func (t Alerts) Render() string {
	tr := t.Tree()
	return tr.Render()
}

func (t Alerts) Tree() *tree.Tree {
	tr := tree.New()
	if len(t) == 0 {
		return tr
	}
	node := tr.AddNode()
	node.AddColumn().AddText("alert level").SetColor(rawconfig.Color.Secondary)
	node.AddColumn().AddText("key").SetColor(rawconfig.Color.Secondary)
	node.AddColumn().AddText("driver").SetColor(rawconfig.Color.Secondary)
	node.AddColumn().AddText("kind").SetColor(rawconfig.Color.Secondary)
	node.AddColumn().AddText("comment").SetColor(rawconfig.Color.Secondary)
	for _, alert := range t {
		n := tr.AddNode()
		color := rawconfig.Color.Warning
		if alert.Level == alertLevelError {
			color = rawconfig.Color.Error
		}
		driver := alert.Driver.String()
		if driver == "" {
			driver = "-"
		}
		comment := alert.Comment
		if comment == "" {
			comment = "-"
		}
		n.AddColumn().AddText(alert.Level.String()).SetColor(color)
		n.AddColumn().AddText(alert.Key.String())
		n.AddColumn().AddText(driver)
		n.AddColumn().AddText(alert.Kind.String())
		n.AddColumn().AddText(comment)
	}
	return tr
}

func (t T) Validate() (Alerts, error) {
	alerts := make(Alerts, 0)
	for _, s := range t.file.Sections() {
		var did driver.ID
		section := s.Name()
		sectionType := t.GetString(key.New(section, "type"))
		if rid, err := resourceid.Parse(section); err == nil {
			did = driver.NewID(rid.DriverGroup(), sectionType)
			if did.Name != "" {
				if sectionType == "" {
					sectionType = did.Name
				}
				if !driver.Exists(did) {
					alerts = append(alerts, t.NewAlertUnknownDriver(key.T{Section: section}, did))
					continue
				}
				if !capabilities.Has(did.Cap()) {
					alerts = append(alerts, t.NewAlertCapabilities(key.T{Section: section}, did))
					continue
				}
			}
		}
		for option := range s.KeysHash() {
			k := key.Parse(section + "." + option)
			if k.BaseOption() == "type" {
				continue
			}
			kw, err := getKeyword(k, sectionType, t.Referrer)
			if err != nil {
				if k.Section == "DEFAULT" {
					// if a DEFAULT.<option> is not declared as a keyword, don't
					// raise an issue if a keyword exists in drivers, as it
					// may be the default value for this keyword.
					relaxedKey := key.T{Section: "*", Option: k.Option}
					kw, err = getKeyword(relaxedKey, sectionType, t.Referrer)
					if err != nil {
						alerts = append(alerts, t.NewAlertUnknown(k, did))
						continue
					}
				}
			}
			if strings.Contains(k.Option, "@") && !kw.Scopable {
				alerts = append(alerts, t.NewAlertScoping(k, did))
			}
			v, err := t.evalStringAs(k, kw, "")
			if err != nil {
				alerts = append(alerts, t.NewAlertEval(k, did, fmt.Sprint(err)))
				continue
			}
			if kw.Deprecated != "" {
				alerts = append(alerts, t.NewAlertDeprecated(k, did, kw.Deprecated, kw.ReplacedBy))
			}
			if len(kw.Candidates) > 0 {
				switch kw.Converter {
				case nil:
					if !slices.Contains(kw.Candidates, v) {

						alerts = append(alerts, t.NewAlertCandidates(k, did, v))
					}
				case converters.List:
					for _, e := range strings.Fields(v) {
						if !slices.Contains(kw.Candidates, e) {
							alerts = append(alerts, t.NewAlertCandidates(k, did, e))
						}
					}
				}
			}
		}
	}
	return alerts, nil
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

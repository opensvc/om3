package xconfig

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"opensvc.com/opensvc/core/driver"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/core/resourceid"
	"opensvc.com/opensvc/util/key"
	"opensvc.com/opensvc/util/render/tree"
	"opensvc.com/opensvc/util/stringslice"
)

type (
	ValidateAlerts []ValidateAlert
	ValidateAlert  struct {
		Path    path.T             `json:"path"`
		Level   ValidateAlertLevel `json:"level"`
		Kind    ValidateAlertKind  `json:"kind"`
		Key     key.T              `json:"key"`
		Type    string             `json:"type"`
		Comment string             `json:"comment"`
	}
	ValidateAlertKind  int
	ValidateAlertLevel int
)

const (
	validateAlertLevelWarn ValidateAlertLevel = iota
	validateAlertLevelError

	validateAlertKindScoping ValidateAlertKind = iota
	validateAlertKindUnknown
	validateAlertKindUnknownDriver
	validateAlertKindEval
	validateAlertKindCandidates
	validateAlertKindDeprecated
)

var (
	validateAlertLevelWarnStr  = "warning"
	validateAlertLevelErrorStr = "error"
	validateAlertLevelNames    = map[ValidateAlertLevel]string{
		validateAlertLevelWarn:  validateAlertLevelWarnStr,
		validateAlertLevelError: validateAlertLevelErrorStr,
	}
	validateAlertLevelFromNames = map[string]ValidateAlertLevel{
		validateAlertLevelWarnStr:  validateAlertLevelWarn,
		validateAlertLevelErrorStr: validateAlertLevelError,
	}
	validateAlertKindUnknownDriverStr = "driver does not exist"
	validateAlertKindScopingStr       = "keyword does not support scoping"
	validateAlertKindUnknownStr       = "keyword does not exist"
	validateAlertKindEvalStr          = "keyword does not evaluate"
	validateAlertKindCandidatesStr    = "keyword value is not in allowed candidates"
	validateAlertKindDeprecatedStr    = "keyword is deprecated"
	validateAlertKindNames            = map[ValidateAlertKind]string{
		validateAlertKindScoping:       validateAlertKindScopingStr,
		validateAlertKindUnknown:       validateAlertKindUnknownStr,
		validateAlertKindUnknownDriver: validateAlertKindUnknownDriverStr,
		validateAlertKindEval:          validateAlertKindEvalStr,
		validateAlertKindCandidates:    validateAlertKindCandidatesStr,
		validateAlertKindDeprecated:    validateAlertKindDeprecatedStr,
	}
	validateAlertKindFromNames = map[string]ValidateAlertKind{
		validateAlertKindScopingStr:       validateAlertKindScoping,
		validateAlertKindUnknownStr:       validateAlertKindUnknown,
		validateAlertKindUnknownDriverStr: validateAlertKindUnknownDriver,
		validateAlertKindEvalStr:          validateAlertKindEval,
		validateAlertKindCandidatesStr:    validateAlertKindCandidates,
		validateAlertKindDeprecatedStr:    validateAlertKindDeprecated,
	}
)

func (t T) NewValidateAlertScoping(k key.T, st string) ValidateAlert {
	return ValidateAlert{
		Path:  t.Path,
		Kind:  validateAlertKindScoping,
		Level: validateAlertLevelError,
		Key:   k,
		Type:  st,
	}
}

func (t T) NewValidateAlertUnknownDriver(k key.T, st string) ValidateAlert {
	return ValidateAlert{
		Path:  t.Path,
		Kind:  validateAlertKindUnknownDriver,
		Level: validateAlertLevelWarn,
		Key:   k,
		Type:  st,
	}
}

func (t T) NewValidateAlertUnknown(k key.T, st string) ValidateAlert {
	return ValidateAlert{
		Path:  t.Path,
		Kind:  validateAlertKindUnknown,
		Level: validateAlertLevelWarn,
		Key:   k,
		Type:  st,
	}
}

func (t T) NewValidateAlertCandidates(k key.T, st string) ValidateAlert {
	return ValidateAlert{
		Path:  t.Path,
		Kind:  validateAlertKindCandidates,
		Level: validateAlertLevelError,
		Key:   k,
		Type:  st,
	}
}

func (t T) NewValidateAlertEval(k key.T, st, comment string) ValidateAlert {
	return ValidateAlert{
		Path:    t.Path,
		Kind:    validateAlertKindEval,
		Level:   validateAlertLevelError,
		Key:     k,
		Type:    st,
		Comment: comment,
	}
}

func (t T) NewValidateAlertDeprecated(k key.T, st, release, replacedBy string) ValidateAlert {
	comment := fmt.Sprintf("since %s", release)
	if replacedBy != "" {
		comment += fmt.Sprintf("replaced by %s", replacedBy)
	}
	return ValidateAlert{
		Path:    t.Path,
		Kind:    validateAlertKindDeprecated,
		Level:   validateAlertLevelWarn,
		Key:     k,
		Type:    st,
		Comment: comment,
	}
}

func (t ValidateAlertKind) String() string {
	if s, ok := validateAlertKindNames[t]; ok {
		return s
	} else {
		return ""
	}
}

func (t ValidateAlertLevel) String() string {
	if s, ok := validateAlertLevelNames[t]; ok {
		return s
	} else {
		return ""
	}
}

func (t ValidateAlertLevel) MarshalJSON() ([]byte, error) {
	if s, ok := validateAlertLevelNames[t]; ok {
		return json.Marshal(s)
	} else {
		return nil, errors.Errorf("unknown validate alert level: %d", t)
	}
}

// UnmarshalJSON unmashals a quoted json string to the enum value
func (t *ValidateAlertLevel) UnmarshalJSON(b []byte) error {
	var j string
	err := json.Unmarshal(b, &j)
	if err != nil {
		return err
	}
	*t, _ = validateAlertLevelFromNames[j]
	return nil
}

func (t ValidateAlertKind) MarshalJSON() ([]byte, error) {
	if s, ok := validateAlertKindNames[t]; ok {
		return json.Marshal(s)
	} else {
		return nil, errors.Errorf("unknown validate alert kind: %d", t)
	}
}

// UnmarshalJSON unmashals a quoted json string to the enum value
func (t *ValidateAlertKind) UnmarshalJSON(b []byte) error {
	var j string
	err := json.Unmarshal(b, &j)
	if err != nil {
		return err
	}
	*t, _ = validateAlertKindFromNames[j]
	return nil
}

func (t ValidateAlerts) Render() string {
	tree := tree.New()
	node := tree.AddNode()
	t.LoadTreeNode(node)
	return tree.Render()
}

func (t ValidateAlerts) LoadTreeNode(node *tree.Node) {
	if len(t) == 0 {
		return
	}
	node.AddColumn().AddText("alert level").SetColor(rawconfig.Node.Color.Secondary)
	node.AddColumn().AddText("key").SetColor(rawconfig.Node.Color.Secondary)
	node.AddColumn().AddText("type").SetColor(rawconfig.Node.Color.Secondary)
	node.AddColumn().AddText("kind").SetColor(rawconfig.Node.Color.Secondary)
	node.AddColumn().AddText("comment").SetColor(rawconfig.Node.Color.Secondary)
	for _, alert := range t {
		n := node.AddNode()
		color := rawconfig.Node.Color.Warning
		if alert.Level == validateAlertLevelError {
			color = rawconfig.Node.Color.Error
		}
		n.AddColumn().AddText(alert.Level.String()).SetColor(color)
		n.AddColumn().AddText(alert.Key.String())
		n.AddColumn().AddText(alert.Type)
		n.AddColumn().AddText(alert.Kind.String())
		n.AddColumn().AddText(alert.Comment)
	}
}

func (t ValidateAlerts) HasError() bool {
	for _, alert := range t {
		if alert.Level == validateAlertLevelError {
			return true
		}
	}
	return false
}

func (t T) Validate() (ValidateAlerts, error) {
	alerts := make(ValidateAlerts, 0)
	for _, s := range t.file.Sections() {
		section := s.Name()
		sectionType := t.GetString(key.New(section, "type"))
		if rid, err := resourceid.Parse(section); err == nil {
			did := driver.NewID(rid.DriverGroup(), sectionType)
			if (did.Name != "") && !driver.Exists(*did) {
				alerts = append(alerts, t.NewValidateAlertUnknownDriver(key.T{Section: section}, sectionType))
				continue
			}
		}
		for option, _ := range s.KeysHash() {
			k := key.Parse(section + "." + option)
			if k.BaseOption() == "type" {
				continue
			}
			kw, err := getKeyword(k, sectionType, t.Referrer)
			if err != nil {
				alerts = append(alerts, t.NewValidateAlertUnknown(k, sectionType))
				continue
			}
			if strings.Contains(k.Option, "@") && !kw.Scopable {
				alerts = append(alerts, t.NewValidateAlertScoping(k, sectionType))
			}
			v, err := t.evalStringAs(k, kw, "")
			if err != nil {
				alerts = append(alerts, t.NewValidateAlertEval(k, sectionType, fmt.Sprint(err)))
				continue
			}
			if kw.Deprecated != "" {
				alerts = append(alerts, t.NewValidateAlertDeprecated(k, sectionType, kw.Deprecated, kw.ReplacedBy))
			}
			if (len(kw.Candidates) > 0) && !stringslice.Has(v, kw.Candidates) {
				alerts = append(alerts, t.NewValidateAlertCandidates(k, sectionType))
			}
		}
	}
	if alerts.HasError() {
		return alerts, errors.New("")
	}
	return alerts, nil
}

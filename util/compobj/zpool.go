package main

type (
	CompZpools struct {
		CompZprops
	}
)

var compZpoolInfo = ObjInfo{
	DefaultPrefix: "OSVC_COMP_ZPOOL_",
	ExampleValue: []CompZprop{
		{
			Name:  "rpool",
			Prop:  "failmode",
			Op:    "=",
			Value: "continue",
		}, {
			Name:  "rpool",
			Prop:  "dedupditto",
			Op:    "<",
			Value: 1,
		}, {
			Name:  "rpool",
			Prop:  "dedupditto",
			Op:    ">",
			Value: 0,
		}, {
			Name:  "dedupditto",
			Prop:  "copies",
			Op:    "<=",
			Value: 1,
		}, {
			Name:  "rpool",
			Prop:  "dedupditto",
			Op:    ">=",
			Value: 1,
		},
	},
	Description: `* Check the properties values against their target and operator
* The collector provides the format with wildcards.
* The module replace the wildcards with contextual values.
* In the 'fix' the zpool property is set.
`,
	FormDefinition: `Desc: |
  A rule to set a list of zpool properties.
Css: comp48

Outputs:
  -
    Dest: compliance variable
    Type: json
    Format: list of dict
    Class: zpool

Inputs:
  -
    Id: name
    Label: Pool Name
    DisplayModeLabel: poolname
    LabelCss: hd16
    Mandatory: Yes
    Type: string
    Help: The zpool name whose property to check.
  -
    Id: prop
    Label: Property
    DisplayModeLabel: property
    LabelCss: action16
    Mandatory: Yes
    Type: string
    Help: The zpool property to check.
    Candidates:
      - readonly
      - autoexpand
      - autoreplace
      - bootfs
      - cachefile
      - dedupditto
      - delegation
      - failmode
      - listshares
      - listsnapshots
      - version

  -
    Id: op_s
    Key: op
    Label: Comparison operator
    DisplayModeLabel: op
    LabelCss: action16
    Type: info
    Default: "="
    ReadOnly: yes
    Help: The comparison operator to use to check the property current value.
    Condition: "#prop IN readonly,autoexpand,autoreplace,bootfs,cachefile,delegation,failmode,listshares,listsnapshots"
  -
    Id: op_n
    Key: op
    Label: Comparison operator
    DisplayModeLabel: op
    LabelCss: action16
    Mandatory: Yes
    Type: string
    Default: "="
    StrictCandidates: yes
    Candidates:
      - "="
      - ">"
      - ">="
      - "<"
      - "<="
    Help: The comparison operator to use to check the property current value.
    Condition: "#prop IN version,dedupditto"

  -
    Id: value_readonly
    Key: value
    Label: Value
    DisplayModeLabel: value
    LabelCss: action16
    Mandatory: Yes
    Type: string
    Help: The zpool property target value.
    Condition: "#prop == readonly"
    StrictCandidates: yes
    Candidates:
      - "on"
      - "off"
  -
    Id: value_autoexpand
    Key: value
    Label: Value
    DisplayModeLabel: value
    LabelCss: action16
    Mandatory: Yes
    Type: string
    Help: The zpool property target value.
    Condition: "#prop == autoexpand"
    StrictCandidates: yes
    Candidates:
      - "on"
      - "off"
  -
    Id: value_autoreplace
    Key: value
    Label: Value
    DisplayModeLabel: value
    LabelCss: action16
    Mandatory: Yes
    Type: string
    Help: The zpool property target value.
    Condition: "#prop == autoreplace"
    StrictCandidates: yes
    Candidates:
      - "on"
      - "off"
  -
    Id: value_delegation
    Key: value
    Label: Value
    DisplayModeLabel: value
    LabelCss: action16
    Mandatory: Yes
    Type: string
    Help: The zpool property target value.
    Condition: "#prop == delegation"
    StrictCandidates: yes
    Candidates:
      - "on"
      - "off"
  -
    Id: value_listshares
    Key: value
    Label: Value
    DisplayModeLabel: value
    LabelCss: action16
    Mandatory: Yes
    Type: string
    Help: The zpool property target value.
    Condition: "#prop == listshares"
    StrictCandidates: yes
    Candidates:
      - "on"
      - "off"
  -
    Id: value_listsnapshots
    Key: value
    Label: Value
    DisplayModeLabel: value
    LabelCss: action16
    Mandatory: Yes
    Type: string
    Help: The zpool property target value.
    Condition: "#prop == listsnapshots"
    StrictCandidates: yes
    Candidates:
      - "on"
      - "off"
  -
    Id: value_failmode
    Key: value
    Label: Value
    DisplayModeLabel: value
    LabelCss: action16
    Mandatory: Yes
    Type: string
    Help: The zpool property target value.
    Condition: "#prop == failmode"
    StrictCandidates: yes
    Candidates:
      - "continue"
      - "wait"
      - "panic"
  -
    Id: value_bootfs
    Key: value
    Label: Value
    DisplayModeLabel: value
    LabelCss: action16
    Mandatory: Yes
    Type: string
    Help: The zpool property target value.
    Condition: "#prop == bootfs"
  -
    Id: value_cachefile
    Key: value
    Label: Value
    DisplayModeLabel: value
    LabelCss: action16
    Mandatory: Yes
    Type: string
    Help: The zpool property target value.
    Condition: "#prop == cachefile"
  -
    Id: value_dedupditto
    Key: value
    Label: Value
    DisplayModeLabel: value
    LabelCss: action16
    Mandatory: Yes
    Type: integer
    Help: The zpool property target value.
    Condition: "#prop == dedupditto"
  -
    Id: value_version
    Key: value
    Label: Value
    DisplayModeLabel: value
    LabelCss: action16
    Mandatory: Yes
    Type: integer
    Help: The zpool property target value.
    Condition: "#prop == version"
`,
}

func init() {
	m["zpool"] = NewCompZpools
}

func NewCompZpools() interface{} {
	return &CompZpools{
		CompZprops{NewObj()},
	}
}

func (t CompZpools) Add(s string) error {
	zpropZbin = "zpool"
	return t.add(s)
}

func (t CompZpools) Info() ObjInfo {
	return compZpoolInfo
}

package main

type (
	CompZfss struct {
		CompZprops
	}
)

var compZfsInfo = ObjInfo{
	DefaultPrefix: "OSVC_COMP_ZFS_",
	ExampleValue: []CompZprop{
		{
			Name:  "rpool/swap",
			Prop:  "aclmode",
			Op:    "=",
			Value: "discard",
		}, {
			Name:  "rpool/swap",
			Prop:  "copies",
			Op:    "<",
			Value: 1,
		}, {
			Name:  "rpool/swap",
			Prop:  "copies",
			Op:    ">",
			Value: 0,
		}, {
			Name:  "rpool/swap",
			Prop:  "copies",
			Op:    "<=",
			Value: 1,
		}, {
			Name:  "rpool/swap",
			Prop:  "copies",
			Op:    ">=",
			Value: 1,
		},
	},
	Description: `* Check the properties values against their target and operator
* The collector provides the format with wildcards.
* The module replace the wildcards with contextual values.
* In the 'fix' the zfs dataset property is set.
`,
	FormDefinition: `Desc: |
  A rule to set a list of zfs properties.
Css: comp48

Outputs:
  -
    Dest: compliance variable
    Type: json
    Format: list of dict
    Class: zfs dataset

Inputs:
  -
    Id: name
    Label: Dataset Name
    DisplayModeLabel: dsname
    LabelCss: hd16
    Mandatory: Yes
    Type: string
    Help: The zfs dataset name whose property to check.
  -
    Id: prop
    Label: Property
    DisplayModeLabel: property
    LabelCss: action16
    Mandatory: Yes
    Type: string
    Help: The zfs dataset property to check.
    Candidates:
      - aclinherit
      - aclmode
      - atime
      - canmount
      - checksum
      - compression
      - copies
      - dedup
      - devices
      - exec
      - keychangedate
      - keysource
      - logbias
      - mountpoint
      - nbmand
      - primarycache
      - quota
      - readonly
      - recordsize
      - refquota
      - refreservation
      - rekeydate
      - reservation
      - rstchown
      - secondarycache
      - setuid
      - share.*
      - snapdir
      - sync
      - vscan
      - xattr
      - zoned
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
    Condition: "#prop != copies"
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
      - ">="
      - "<="
    Help: The comparison operator to use to check the property current value.
    Condition: "#prop == copies"
  -
    Id: value_on_off
    Key: value
    Label: Value
    DisplayModeLabel: value
    LabelCss: action16
    Mandatory: Yes
    Type: string
    Help: The zfs dataset property target value.
    Condition: "#prop IN sharenfs,sharesmb"
    StrictCandidates: yes
    Candidates:
      - "on"
      - "off"
  -
    Id: value_on_off_strict
    Key: value
    Label: Value
    DisplayModeLabel: value
    LabelCss: action16
    Mandatory: Yes
    Type: string
    Help: The zfs dataset property target value.
    Condition: "#prop IN canmount,atime,readonly,exec,devices,setuid,vscan,xattr,jailed,utf8only"
    StrictCandidates: yes
    Candidates:
      - "on"
      - "off"
  -
    Id: value_n
    Key: value
    Label: Value
    DisplayModeLabel: value
    LabelCss: action16
    Mandatory: Yes
    Type: integer
    Help: The zfs dataset property target value.
    Condition: "#prop IN copies,recordsize,volsize"
  -
    Id: value_s
    Key: value
    Label: Value
    DisplayModeLabel: value
    LabelCss: action16
    Mandatory: Yes
    Type: string
    Help: The zfs dataset property target value.
    Condition: "#prop NOT IN normalization,casesensitivity,sync,volmode,logbias,snapdir,dedup,primarycache,secondarycache,redundant_metadata,checksum,compression,aclinherit,aclmode,copies,recordsize,volsize,canmount,atime,readonly,exec,devices,setuid,vscan,xattr,jailed,utf8only,sharenfs,sharesmb"
  -
    Id: value_aclinherit
    Key: value
    Label: Value
    DisplayModeLabel: value
    LabelCss: action16
    Mandatory: Yes
    Type: string
    Help: The zfs dataset property target value.
    Condition: "#prop == aclinherit"
    StrictCandidates: yes
    Candidates:
      - "discard"
      - "noallow"
      - "restricted"
      - "passthrough"
      - "passthrough-x"
  -
    Id: value_aclmode
    Key: value
    Label: Value
    DisplayModeLabel: value
    LabelCss: action16
    Mandatory: Yes
    Type: string
    Help: The zfs dataset property target value.
    Condition: "#prop == aclmode"
    StrictCandidates: yes
    Candidates:
      - "discard"
      - "groupmask"
      - "passthrough"
      - "restricted"
  -
    Id: value_checksum
    Key: value
    Label: Value
    DisplayModeLabel: value
    LabelCss: action16
    Mandatory: Yes
    Type: string
    Help: The zfs dataset property target value.
    Condition: "#prop == checksum"
    StrictCandidates: yes
    Candidates:
      - "on"
      - "off"
      - "fletcher2"
      - "fletcher4"
      - "sha256"
      - "noparity"
  -
    Id: value_compression
    Key: value
    Label: Value
    DisplayModeLabel: value
    LabelCss: action16
    Mandatory: Yes
    Type: string
    Help: The zfs dataset property target value.
    Condition: "#prop == compression"
    StrictCandidates: yes
    Candidates:
      - "on"
      - "off"
      - "lzjb"
      - "gzip"
      - "gzip-1"
      - "gzip-2"
      - "gzip-3"
      - "gzip-4"
      - "gzip-5"
      - "gzip-6"
      - "gzip-7"
      - "gzip-8"
      - "gzip-9"
      - "zle"
      - "lz4"
  -
    Id: value_dedup
    Key: value
    Label: Value
    DisplayModeLabel: value
    LabelCss: action16
    Mandatory: Yes
    Type: string
    Help: The zfs dataset property target value.
    Condition: "#prop == dedup"
    StrictCandidates: yes
    Candidates:
      - "on"
      - "off"
      - "verify"
      - "sha256"
      - "sha256,verify"
  -
    Id: value_primarycache
    Key: value
    Label: Value
    DisplayModeLabel: value
    LabelCss: action16
    Mandatory: Yes
    Type: string
    Help: The zfs dataset property target value.
    Condition: "#prop IN primarycache,secondarycache"
    StrictCandidates: yes
    Candidates:
      - "all"
      - "none"
      - "metadata"
  -
    Id: value_redundant_metadata
    Key: value
    Label: Value
    DisplayModeLabel: value
    LabelCss: action16
    Mandatory: Yes
    Type: string
    Help: The zfs dataset property target value.
    Condition: "#prop == redundant_metadata"
    StrictCandidates: yes
    Candidates:
      - "all"
      - "most"
  -
    Id: value_logbias
    Key: value
    Label: Value
    DisplayModeLabel: value
    LabelCss: action16
    Mandatory: Yes
    Type: string
    Help: The zfs dataset property target value.
    Condition: "#prop == logbias"
    StrictCandidates: yes
    Candidates:
      - "latency"
      - "throughput"
  -
    Id: value_snapdir
    Key: value
    Label: Value
    DisplayModeLabel: value
    LabelCss: action16
    Mandatory: Yes
    Type: string
    Help: The zfs dataset property target value.
    Condition: "#prop == snapdir"
    StrictCandidates: yes
    Candidates:
      - "hidden"
      - "visible"
  -
    Id: value_sync
    Key: value
    Label: Value
    DisplayModeLabel: value
    LabelCss: action16
    Mandatory: Yes
    Type: string
    Help: The zfs dataset property target value.
    Condition: "#prop == sync"
    StrictCandidates: yes
    Candidates:
      - "standard"
      - "always"
      - "disabled"
  -
    Id: value_volmode
    Key: value
    Label: Value
    DisplayModeLabel: value
    LabelCss: action16
    Mandatory: Yes
    Type: string
    Help: The zfs dataset property target value.
    Condition: "#prop == volmode"
    StrictCandidates: yes
    Candidates:
      - "default"
      - "geom"
      - "dev"
      - "none"
  -
    Id: value_casesensitivity
    Key: value
    Label: Value
    DisplayModeLabel: value
    LabelCss: action16
    Mandatory: Yes
    Type: string
    Help: The zfs dataset property target value.
    Condition: "#prop == casesensitivity"
    StrictCandidates: yes
    Candidates:
      - "sensitive"
      - "insensitive"
      - "mixed"
  -
    Id: value_normalization
    Key: value
    Label: Value
    DisplayModeLabel: value
    LabelCss: action16
    Mandatory: Yes
    Type: string
    Help: The zfs dataset property target value.
    Condition: "#prop == normalization"
    StrictCandidates: yes
    Candidates:
      - "none"
      - "formC"
      - "formD"
      - "formKC"
      - "formKD"
`,
}

func init() {
	m["zfs"] = NewCompZfss
}

func NewCompZfss() interface{} {
	return &CompZfss{
		CompZprops{NewObj()},
	}
}

func (t CompZfss) Add(s string) error {
	zpropZbin = "/usr/sbin/zfs"
	return t.add(s)
}

func (t CompZfss) Info() ObjInfo {
	return compZfsInfo
}

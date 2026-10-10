package arrayfreenas

import (
	"fmt"
	"slices"
	"strings"
)

// CreateISCSIExtentParams defines model for CreateISCSIExtentParams.
type CreateISCSIExtentParams struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	InsecureTPC bool   `json:"insecure_tpc"`
	Blocksize   int    `json:"blocksize"`
	Disk        string `json:"disk"`
}

// CreateISCSIInitiatorParams defines model for CreateISCSIInitiatorParams.
type CreateISCSIInitiatorParams struct {
	Initiators  []string `json:"initiators"`
	AuthNetwork []string `json:"auth_network,omitempty"`
	Comment     string   `json:"comment,omitempty"`
}

// CreateISCSITargetExtentParams defines model for CreateISCSITargetExtentParams.
type CreateISCSITargetExtentParams struct {
	Target int  `json:"target"`
	Extent int  `json:"extent"`
	LunId  *int `json:"lunid"`
}

// CreateISCSITargetParams defines model for CreateISCSITargetParams.
type CreateISCSITargetParams struct {
	Name  string `json:"name"`
	Alias string `json:"alias,omitempty"`
}

// UpdateISCSITargetParams defines model for UpdateISCSITargetParams.
type UpdateISCSITargetParams struct {
	Name   string            `json:"name"`
	Alias  string            `json:"alias,omitempty"`
	Mode   string            `json:"mode"`
	Groups ISCSITargetGroups `json:"groups,omitempty"`
}

type ISCSIPortal struct {
	Id                  int                   `json:"id,omitempty"`
	Comment             string                `json:"comment,omitempty"`
	DiscoveryAuthMethod string                `json:"discovery_authmethod,omitempty"`
	DiscoveryAuthGroup  int                   `json:"discovery_authgroup,omitempty"`
	Listen              []ISCSIPortalListenIp `json:"listen"`
}

type CreateISCSIPortalParams struct {
	Comment             string                `json:"comment,omitempty"`
	DiscoveryAuthMethod string                `json:"discovery_authmethod,omitempty"`
	DiscoveryAuthGroup  int                   `json:"discovery_authgroup,omitempty"`
	Listen              []ISCSIPortalListenIp `json:"listen"`
}

type ISCSIPortalListenIp struct {
	Ip   string `json:"ip"`
	Port int    `json:"port"`
}

// ISCSIExtent defines model for ISCSIExtent.
//
//	{
//	  "id": 218,
//	  "name": "c28_disk_md4",
//	  "serial": "08002734c651217",
//	  "type": "DISK",
//	  "path": "zvol/osvcdata/c28_disk_md4",
//	  "filesize": "0",
//	  "blocksize": 512,
//	  "pblocksize": false,
//	  "avail_threshold": null,
//	  "comment": "",
//	  "naa": "0x6589cfc000000f487531b4688113d131",
//	  "insecure_tpc": true,
//	  "xen": false,
//	  "rpm": "SSD",
//	  "ro": false,
//	  "enabled": true,
//	  "vendor": "TrueNAS",
//	  "disk": "zvol/osvcdata/c28_disk_md4",
//	  "locked": false
//	}
type ISCSIExtent struct {
	Id             int     `json:"id"`
	Name           string  `json:"name"`
	Serial         string  `json:"serial"`
	Type           string  `json:"type"`
	Path           string  `json:"path"`
	Filesize       any     `json:"filesize"`
	Blocksize      uint64  `json:"blocksize"`
	PBlocksize     bool    `json:"pblocksize"`
	AvailThreshold *uint64 `json:"avail_threshold"`
	Comment        string  `json:"comment"`
	NAA            string  `json:"naa"`
	InsecureTPC    bool    `json:"insecure_tpc"`
	Xen            bool    `json:"xen"`
	RPM            string  `json:"rpm"`
	RO             bool    `json:"ro"`
	Enabled        bool    `json:"enabled"`
	Vendor         string  `json:"vendor"`
	Disk           string  `json:"disk"`
	Locked         bool    `json:"locked"`
}

type ISCSIExtents []ISCSIExtent

// ISCSIExtentsResponse defines model for ISCSIExtentsResponse.
type ISCSIExtentsResponse = []ISCSIExtent

// GetISCSIExtentsParams defines parameters for GetISCSIExtents.
type GetISCSIExtentsParams struct {
	Limit  *int    `form:"limit,omitempty" json:"limit,omitempty"`
	Offset *int    `form:"offset,omitempty" json:"offset,omitempty"`
	Count  *bool   `form:"count,omitempty" json:"count,omitempty"`
	Sort   *string `form:"sort,omitempty" json:"sort,omitempty"`
}

func (t ISCSIExtents) WithType(s string) ISCSIExtents {
	l := make(ISCSIExtents, 0)
	for _, e := range t {
		if e.Type == s {
			l = append(l, e)
		}
	}
	return l
}

func (t ISCSIExtents) WithPath(s string) ISCSIExtents {
	l := make(ISCSIExtents, 0)
	for _, e := range t {
		if e.Path == s {
			l = append(l, e)
		}
	}
	return l
}

func (t ISCSIExtents) GetByName(name string) *ISCSIExtent {
	for _, e := range t {
		if e.Name == name {
			return &e
		}
	}
	return nil
}

func (t ISCSIExtents) GetById(s int) *ISCSIExtent {
	for _, e := range t {
		if e.Id == s {
			return &e
		}
	}
	return nil
}

func (t ISCSIExtents) GetByPath(s string) *ISCSIExtent {
	for _, e := range t {
		if e.Path == s {
			return &e
		}
	}
	return nil
}

// GetByNAA returns the extent of a naa, written with or without its 0x
// prefix, as v2 accepts it.
func (t ISCSIExtents) GetByNAA(naa string) *ISCSIExtent {
	want := strings.TrimPrefix(strings.ToLower(naa), "0x")
	if want == "" {
		return nil
	}
	for _, e := range t {
		if strings.TrimPrefix(strings.ToLower(e.NAA), "0x") == want {
			return &e
		}
	}
	return nil
}

// WithZvol returns the extents exporting a zvol.
func (t ISCSIExtents) WithZvol(name string) ISCSIExtents {
	l := make(ISCSIExtents, 0)
	for _, e := range t {
		if s, ok := e.zvol(); ok && s == name {
			l = append(l, e)
		}
	}
	return l
}

// Names returns the extents as an error message names them.
func (t ISCSIExtents) Names() string {
	l := make([]string, len(t))
	for i, e := range t {
		l[i] = fmt.Sprintf("%d (%s)", e.Id, e.Name)
	}
	return strings.Join(l, ", ")
}

// checkFree returns an error when an extent is named name, or exports disk:
// the array refuses the first, and the second exports the zvol twice.
func (t ISCSIExtents) checkFree(name, disk string) error {
	if e := t.GetByName(name); e != nil {
		return fmt.Errorf("extent %s already exists (id %d, exporting %s)", name, e.Id, e.diskPath())
	}
	for _, e := range t {
		if e.diskPath() == disk {
			return fmt.Errorf("%s is already exported by extent %d (%s)", disk, e.Id, e.Name)
		}
	}
	return nil
}

// diskPath returns what the extent exports. The api says it in "disk" for
// a DISK extent and in "path" for a FILE one, and some versions say it in
// both.
func (t ISCSIExtent) diskPath() string {
	if t.Disk != "" {
		return t.Disk
	}
	return t.Path
}

// zvol returns the name of the zvol the extent exports, the whole of it, as
// in "pool/dir/name". v2 read its first component only, which is the pool
// of a zvol in a subdirectory.
func (t ISCSIExtent) zvol() (string, bool) {
	if t.Type != "DISK" {
		return "", false
	}
	name, ok := strings.CutPrefix(t.diskPath(), "zvol/")
	return name, ok && name != ""
}

// ISCSIInitiator defines model for ISCSIInitiator.
//
//	{
//	    "id": 40,
//	    "initiators": [
//	        "iqn.2009-11.com.opensvc.srv:qau22c13n3.storage.initiator"
//	    ],
//	    "auth_network": [],
//	    "comment": ""
//	}
type ISCSIInitiator struct {
	Id         int      `json:"id"`
	Initiators []string `json:"initiators"`
	Comment    string   `json:"comment"`
}

type ISCSIInitiators []ISCSIInitiator

// anyInitiator is the hba id a mapping report gives the initiators of a
// target group that admits any initiator.
const anyInitiator = "*"

// groupInitiators returns the initiators a target group admits, and true
// when it admits any: the group names no initiator group, which the array
// answers as a null id decoded as 0, or names one listing no initiator.
func groupInitiators(group ISCSITargetGroup, initiators ISCSIInitiators) ([]string, bool, error) {
	if group.InitiatorId == 0 {
		return nil, true, nil
	}
	initiator, ok := initiators.GetById(group.InitiatorId)
	if !ok {
		return nil, false, fmt.Errorf("initiator group id %d not found", group.InitiatorId)
	}
	if len(initiator.Initiators) == 0 {
		return nil, true, nil
	}
	return initiator.Initiators, false, nil
}

// targetAdmits returns true when a group of the target admits the initiator.
func targetAdmits(target ISCSITarget, hba string, initiators ISCSIInitiators) (bool, error) {
	for _, group := range target.Groups {
		names, any, err := groupInitiators(group, initiators)
		if err != nil {
			return false, fmt.Errorf("target %s: %w", target.Name, err)
		}
		if any || slices.Contains(names, hba) {
			return true, nil
		}
	}
	return false, nil
}

// GetISCSIInitiatorsParams defines parameters for GetISCSIInitiators.
type GetISCSIInitiatorsParams struct {
	Limit  *int    `form:"limit,omitempty" json:"limit,omitempty"`
	Offset *int    `form:"offset,omitempty" json:"offset,omitempty"`
	Count  *bool   `form:"count,omitempty" json:"count,omitempty"`
	Sort   *string `form:"sort,omitempty" json:"sort,omitempty"`
}

func (t ISCSIInitiators) WithName(s string) ISCSIInitiators {
	l := make(ISCSIInitiators, 0)
	for _, e := range t {
		for _, name := range e.Initiators {
			if name == s {
				l = append(l, e)
				break
			}
		}
	}
	return l
}

func (t ISCSIInitiators) GetById(id int) (ISCSIInitiator, bool) {
	for _, e := range t {
		if e.Id == id {
			return e, true
		}
	}
	return ISCSIInitiator{}, false
}

// ISCSITargetExtent defines model for ISCSITargetExtent.
//
//	 {
//	    "id": 1463,
//	    "lunid": 42,
//	    "extent": 211,
//	    "target": 76
//	}
type ISCSITargetExtent struct {
	Id       int `json:"id"`
	LunId    int `json:"lunid"`
	ExtentId int `json:"extent"`
	TargetId int `json:"target"`
}

type ISCSITargetExtents []ISCSITargetExtent

// GetISCSITargetExtentsParams defines parameters for GetISCSITargetExtents.
type GetISCSITargetExtentsParams struct {
	Limit  *int    `form:"limit,omitempty" json:"limit,omitempty"`
	Offset *int    `form:"offset,omitempty" json:"offset,omitempty"`
	Count  *bool   `form:"count,omitempty" json:"count,omitempty"`
	Sort   *string `form:"sort,omitempty" json:"sort,omitempty"`
}

func (t ISCSITargetExtents) WithExtent(extent ISCSIExtent) ISCSITargetExtents {
	l := make(ISCSITargetExtents, 0)
	for _, one := range t {
		if one.ExtentId == extent.Id {
			l = append(l, one)
		}
	}
	return l
}

func (t ISCSITargetExtents) WithTarget(target ISCSITarget) ISCSITargetExtents {
	l := make(ISCSITargetExtents, 0)
	for _, one := range t {
		if one.TargetId == target.Id {
			l = append(l, one)
		}
	}
	return l
}

// ISCSITarget defines model for ISCSITarget.
//
//	{
//	 "id": 79,
//	 "name": "iqn.2009-11.com.opensvc.srv:qau20c26n3.storage.target.1",
//	 "alias": null,
//	 "mode": "ISCSI",
//	 "groups": [
//	  {
//	   "portal": 1,
//	   "initiator": 43,
//	   "auth": null,
//	   "authmethod": "NONE"
//	  }
//	 ]
//	},
type ISCSITarget struct {
	Id     int               `json:"id"`
	Name   string            `json:"name"`
	Alias  *string           `json:"alias,omitempty"`
	Mode   string            `json:"mode"`
	Groups ISCSITargetGroups `json:"groups"`
}

type ISCSITargets []ISCSITarget

type ISCSITargetGroups []ISCSITargetGroup

type ISCSITargetGroup struct {
	PortalId    int     `json:"portal"`
	InitiatorId int     `json:"initiator"`
	Auth        *string `json:"auth"`
	AuthMethod  string  `json:"authmethod"`
}

// ISCSITargetsResponse defines model for ISCSITargetsResponse.
type ISCSITargetsResponse = []ISCSITarget

// GetISCSITargetsParams defines parameters for GetISCSITargets.
type GetISCSITargetsParams struct {
	Limit  *int    `form:"limit,omitempty" json:"limit,omitempty"`
	Offset *int    `form:"offset,omitempty" json:"offset,omitempty"`
	Count  *bool   `form:"count,omitempty" json:"count,omitempty"`
	Sort   *string `form:"sort,omitempty" json:"sort,omitempty"`
}

func (t ISCSITargets) GetById(id int) (ISCSITarget, bool) {
	for _, e := range t {
		if e.Id == id {

			return e, true
		}
	}
	return ISCSITarget{}, false
}

func (t ISCSITargets) GetByName(name string) (ISCSITarget, bool) {
	for _, e := range t {
		if e.Name == name {

			return e, true
		}
	}
	return ISCSITarget{}, false
}

func (t ISCSITargets) WithName(s string) ISCSITargets {
	l := make(ISCSITargets, 0)
	for _, e := range t {
		if e.Name == s {
			l = append(l, e)
		}
	}
	return l
}

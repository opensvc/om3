package arrayfreenas

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

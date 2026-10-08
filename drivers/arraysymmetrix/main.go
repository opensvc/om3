package arraysymmetrix

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"golang.org/x/exp/maps"

	"github.com/opensvc/om3/v3/core/array"
	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/resourceid"
	"github.com/opensvc/om3/v3/util/nullable"
	"github.com/opensvc/om3/v3/util/plog"
)

type (
	Array struct {
		*array.Array
		log *plog.Logger

		// runner runs the symcli commands in place of the programs
		// installed under symcli_path, when a test set it.
		runner runFunc
	}

	// The counts of the listings below are strings: symaccess prints N/A
	// for a count it does not know, which a number field fails to decode,
	// and a listing failing to decode fails the action reading it.

	//
	XSymAccessListPort struct {
		XMLName   xml.Name                    `xml:"SymCLI_ML" json:"-"`
		Symmetrix XSymAccessListPortSymmetrix `xml:"Symmetrix" json:"Symmetrix"`
	}
	XSymAccessListPortSymmetrix struct {
		XMLName    xml.Name      `xml:"Symmetrix" json:"-"`
		SymmInfo   SymmInfoShort `xml:"Symm_Info" json:"Symm_Info"`
		PortGroups []PortGroup   `xml:"Port_Group" json:"Port_Group"`
	}
	PortGroup struct {
		XMLName   xml.Name      `xml:"Port_Group" json:"-"`
		GroupInfo PortGroupInfo `xml:"Group_Info" json:"Group_Info"`
	}
	PortGroupInfo struct {
		XMLName       xml.Name      `xml:"Group_Info" json:"-"`
		GroupName     string        `xml:"group_name" json:"group_name"`
		PortCount     string        `xml:"port_count" json:"port_count"`
		ViewCount     string        `xml:"view_count" json:"view_count"`
		LastUpdated   string        `xml:"last_updated" json:"last_updated"`
		MaskViewNames MaskViewNames `xml:"Mask_View_Names" json:"Mask_View_Names"`
	}

	//
	XSymAccessListDevInitiator struct {
		XMLName   xml.Name                            `xml:"SymCLI_ML" json:"-"`
		Symmetrix XSymAccessListDevInitiatorSymmetrix `xml:"Symmetrix" json:"Symmetrix"`
	}
	XSymAccessListDevInitiatorSymmetrix struct {
		XMLName         xml.Name         `xml:"Symmetrix" json:"-"`
		SymmInfo        SymmInfoShort    `xml:"Symm_Info" json:"Symm_Info"`
		InitiatorGroups []InitiatorGroup `xml:"Initiator_Group" json:"Initiator_Group"`
	}
	InitiatorGroup struct {
		XMLName   xml.Name           `xml:"Initiator_Group" json:"-"`
		GroupInfo InitiatorGroupInfo `xml:"Group_Info" json:"Group_Info"`
	}
	InitiatorGroupInfo struct {
		XMLName       xml.Name      `xml:"Group_Info" json:"-"`
		GroupName     string        `xml:"group_name" json:"group_name"`
		ConsistentLUN string        `xml:"consistent_lun" json:"consistent_lun"`
		DevCount      string        `xml:"dev_count" json:"dev_count"`
		SGCount       string        `xml:"sg_count" json:"sg_count"`
		ViewCount     string        `xml:"view_count" json:"view_count"`
		LastUpdated   string        `xml:"last_updated" json:"last_updated"`
		MaskViewNames MaskViewNames `xml:"Mask_View_Names" json:"Mask_View_Names"`
		Status        string        `xml:"status" json:"status"`
	}

	//
	XSymAccessListDevStorage struct {
		XMLName   xml.Name                          `xml:"SymCLI_ML" json:"-"`
		Symmetrix XSymAccessListDevStorageSymmetrix `xml:"Symmetrix" json:"Symmetrix"`
	}
	XSymAccessListDevStorageSymmetrix struct {
		XMLName       xml.Name       `xml:"Symmetrix" json:"-"`
		SymmInfo      SymmInfoShort  `xml:"Symm_Info" json:"Symm_Info"`
		StorageGroups []StorageGroup `xml:"Storage_Group" json:"Storage_Group"`
	}
	StorageGroup struct {
		XMLName   xml.Name         `xml:"Storage_Group" json:"-"`
		GroupInfo StorageGroupInfo `xml:"Group_Info" json:"Group_Info"`
	}
	StorageGroupInfo struct {
		XMLName           xml.Name          `xml:"Group_Info" json:"-"`
		GroupName         string            `xml:"group_name" json:"group_name"`
		DevCount          string            `xml:"dev_count" json:"dev_count"`
		SGCount           string            `xml:"sg_count" json:"sg_count"`
		ViewCount         string            `xml:"view_count" json:"view_count"`
		LastUpdated       string            `xml:"last_updated" json:"last_updated"`
		MaskViewNames     MaskViewNames     `xml:"Mask_View_Names" json:"Mask_View_Names"`
		CascadedViewNames CascadedViewNames `xml:"Cascaded_View_Names" json:"Cascaded_View_Names"`

		// Status is IsParent for a storage group of storage groups, which
		// symaccess spells capitalized, as v2 read it.
		Status string `xml:"Status" json:"status"`
	}
	CascadedViewNames struct {
		XMLName   xml.Name `xml:"Cascaded_View_Names" json:"-"`
		ViewCount string   `xml:"view_count" json:"view_count"`
		ViewNames []string `xml:"view_name" json:"view_name"`
	}
	MaskViewNames struct {
		XMLName   xml.Name `xml:"Mask_View_Names" json:"-"`
		ViewCount string   `xml:"view_count" json:"view_count"`
		ViewNames []string `xml:"view_name" json:"view_name"`
	}

	//
	XSymDevListThinDevs struct {
		XMLName   xml.Name                     `xml:"SymCLI_ML" json:"-"`
		Symmetrix XSymDevListThinDevsSymmetrix `xml:"Symmetrix" json:"Symmetrix"`
	}
	XSymDevListThinDevsSymmetrix struct {
		XMLName  xml.Name      `xml:"Symmetrix" json:"-"`
		SymmInfo SymmInfoShort `xml:"Symm_Info" json:"Symm_Info"`
		ThinDevs []ThinDev     `xml:"ThinDevs>Device" json:"ThinDevs"`
	}
	// ThinDev holds strings: symcfg prints FALSE, NONE or N/A in the
	// columns it has no number for.
	ThinDev struct {
		XMLName              xml.Name `xml:"Device" json:"-"`
		DevName              string   `xml:"dev_name" json:"dev_name"`
		DevEmul              string   `xml:"dev_emul" json:"dev_emul"`
		MultiPool            string   `xml:"multi_pool" json:"multi_pool"`
		SharedTracks         string   `xml:"shared_tracks" json:"shared_tracks"`
		PersistTracks        string   `xml:"persist_tracks" json:"persist_tracks"`
		TotalTracks          string   `xml:"total_tracks" json:"total_tracks"`
		AllocTracks          string   `xml:"alloc_tracks" json:"alloc_tracks"`
		UnreducibleTracks    string   `xml:"unreducible_tracks" json:"unreducible_tracks"`
		WrittenTracks        string   `xml:"written_tracks" json:"written_tracks"`
		CompressedTracks     string   `xml:"compressed_tracks" json:"compressed_tracks"`
		ExclusiveAllocTracks string   `xml:"exclusive_alloc_tracks" json:"exclusive_alloc_tracks"`
	}

	//
	XSymAccessListViewDetail struct {
		XMLName   xml.Name                          `xml:"SymCLI_ML" json:"-"`
		Symmetrix XSymAccessListViewDetailSymmetrix `xml:"Symmetrix" json:"Symmetrix"`
	}
	XSymAccessListViewDetailSymmetrix struct {
		XMLName      xml.Name      `xml:"Symmetrix" json:"-"`
		SymmInfo     SymmInfoShort `xml:"Symm_Info" json:"Symm_Info"`
		MaskingViews []MaskingView `xml:"Masking_View" json:"Masking_View"`
	}
	MaskingView struct {
		XMLName  xml.Name `xml:"Masking_View" json:"-"`
		ViewInfo ViewInfo `xml:"View_Info" json:"View_Info"`
	}
	ViewInfo struct {
		XMLName       xml.Name      `xml:"View_Info" json:"-"`
		Name          string        `xml:"view_name" json:"view_name"`
		LastUpdated   string        `xml:"last_updated" json:"last_updated"`
		InitGrpName   string        `xml:"init_grpname" json:"init_grpname"`
		PortGrpName   string        `xml:"port_grpname" json:"port_grpname"`
		StorGrpName   string        `xml:"stor_grpname" json:"stor_grpname"`
		PortInfo      PortInfo      `xml:"port_info" json:"port_info"`
		SGChildInfo   SGChildInfo   `xml:"SG_Child_info" json:"SG_Child_info"`
		InitiatorList InitiatorList `xml:"Initiator_List" json:"Initiator_List"`
		Devices       []ViewDevice  `xml:"Device" json:"Device"`
	}
	ViewDevice struct {
		XMLName     xml.Name      `xml:"Device" json:"-"`
		DevName     string        `xml:"dev_name" json:"dev_name"`
		DevPortInfo []DevPortInfo `xml:"dev_port_info" json:"dev_port_info"`
	}
	DevPortInfo struct {
		XMLName xml.Name `xml:"dev_port_info" json:"-"`
		Port    int      `xml:"port" json:"port"`
		HostLUN string   `xml:"host_lun" json:"host_lun"`
	}
	InitiatorList struct {
		XMLName    xml.Name    `xml:"Initiator_List" json:"-"`
		Initiators []Initiator `xml:"Initiator" json:"Initiator"`
	}
	Initiator struct {
		XMLName      xml.Name `xml:"Initiator" json:"-"`
		WWN          *string  `xml:"wwn" json:"wwn"`
		UserPortName string   `xml:"user_port_name" json:"user_port_name"`
		UserNodeName string   `xml:"user_node_name" json:"user_node_name"`
	}
	PortInfo struct {
		XMLName                 xml.Name                 `xml:"port_info" json:"-"`
		DirectorIdentifications []DirectorIdentification `xml:"Director_Identification" json:"Director_Identification"`
	}
	DirectorIdentification struct {
		XMLName xml.Name `xml:"Director_Identification" json:"-"`
		Dir     string   `xml:"dir" json:"dir"`
		Port    int      `xml:"port" json:"port"`
		PortWWN string   `xml:"port_wwn" json:"port_wwn"`
	}
	SGChildInfo struct {
		XMLName    xml.Name  `xml:"SG_Child_info" json:"-"`
		ChildCount int       `xml:"child_count" json:"child_count"`
		SG         []SGShort `xml:"SG" json:"SG"`
	}
	SGShort struct {
		XMLName   xml.Name `xml:"SG" json:"-"`
		GroupName string   `xml:"group_name" json:"group_name"`
		Status    string   `xml:"Status" json:"Status"`
	}

	//
	XSymAccessShowPort struct {
		XMLName   xml.Name                    `xml:"SymCLI_ML" json:"-"`
		Symmetrix XSymAccessShowPortSymmetrix `xml:"Symmetrix" json:"Symmetrix"`
	}
	XSymAccessShowPortSymmetrix struct {
		XMLName   xml.Name      `xml:"Symmetrix" json:"-"`
		SymmInfo  SymmInfoShort `xml:"Symm_Info" json:"Symm_Info"`
		PortGroup ShowPortGroup `xml:"Port_Group" json:"Port_Group"`
	}
	ShowPortGroup struct {
		XMLName   xml.Name          `xml:"Port_Group" json:"-"`
		GroupInfo ShowPortGroupInfo `xml:"Group_Info" json:"Group_Info"`
	}
	ShowPortGroupInfo struct {
		XMLName                 xml.Name                 `xml:"Group_Info" json:"-"`
		GroupName               string                   `xml:"group_name" json:"group_name"`
		LastUpdated             string                   `xml:"last_updated" json:"last_updated"`
		DirectorIdentifications []DirectorIdentification `xml:"Director_Identification" json:"Director_Identification"`
	}

	//
	XSymDiskListDiskGroupSummary struct {
		XMLName   xml.Name                              `xml:"SymCLI_ML" json:"-"`
		Symmetrix XSymDiskListDiskGroupSummarySymmetrix `xml:"Symmetrix" json:"Symmetrix"`
	}
	XSymDiskListDiskGroupSummarySymmetrix struct {
		XMLName                xml.Name               `xml:"Symmetrix" json:"-"`
		DiskGroups             []DiskGroup            `xml:"Disk_Group" json:"Disk_Group"`
		SymmInfo               SymmInfoShort          `xml:"Symm_Info" json:"Symm_Info"`
		DiskGroupSummaryTotals DiskGroupSummaryTotals `xml:"Disk_Group_Summary_Totals" json:"Disk_Group_Summary_Totals"`
	}
	DiskGroup struct {
		XMLName         xml.Name        `xml:"Disk_Group" json:"-"`
		DiskGroupInfo   DiskGroupInfo   `xml:"Disk_Group_Info" json:"Disk_Group_Info"`
		DiskGroupTotals DiskGroupTotals `xml:"Disk_Group_Totals" json:"Disk_Group_Totals"`
	}
	DiskGroupSummaryTotals struct {
		XMLName xml.Name `xml:"Disk_Group_Summary_Totals" json:"-"`
		Units   string   `xml:"units" json:"units"`
		Total   int64    `xml:"total" json:"total"`
		Free    int64    `xml:"free" json:"free"`
		Actual  int64    `xml:"actual" json:"actual"`
	}
	DiskGroupTotals struct {
		XMLName xml.Name `xml:"Disk_Group_Totals" json:"-"`
		Units   string   `xml:"units" json:"units"`
		Total   int64    `xml:"total" json:"total"`
		Free    int64    `xml:"free" json:"free"`
		Actual  int64    `xml:"actual" json:"actual"`
	}
	DiskGroupInfo struct {
		XMLName                xml.Name `xml:"Disk_Group_Info" json:"-"`
		DiskGroupNumber        int      `xml:"disk_group_number" json:"disk_group_number"`
		DiskGroupName          string   `xml:"disk_group_name" json:"disk_group_name"`
		DiskLocation           string   `xml:"disk_location" json:"disk_location"`
		DisksSelected          int      `xml:"disks_selected" json:"disks_selected"`
		Technology             string   `xml:"technology" json:"technology"`
		Speed                  int      `xml:"speed" json:"speed"`
		FormFactor             string   `xml:"form_factor" json:"form_factor"`
		HyperSizeMegabytes     int64    `xml:"hyper_size_megabytes" json:"hyper_size_megabytes"`
		HyperSizeGigabytes     float64  `xml:"hyper_size_gigabytes" json:"hyper_size_gigabytes"`
		HyperSizeTerabytes     float64  `xml:"hyper_size_terabytes" json:"hyper_size_terabytes"`
		MaxHypersPerDisk       int      `xml:"max_hypers_per_disk" json:"max_hypers_per_disk"`
		DiskSizeMegabytes      int64    `xml:"disk_size_megabytes" json:"disk_size_megabytes"`
		DiskSizeGigabytes      float64  `xml:"disk_size_gigabytes" json:"disk_size_gigabytes"`
		DiskSizeTerabytes      float64  `xml:"disk_size_terabytes" json:"disk_size_terabytes"`
		RatedDiskSizeGigabytes int64    `xml:"rated_disk_size_gigabytes" json:"rated_disk_size_gigabytes"`
		RatedDiskSizeTerabytes float64  `xml:"rated_disk_size_terabytes" json:"rated_disk_size_terabytes"`
	}

	// RDF is the SRDF pairing of a device, as symdev show prints it.
	//
	// It is kept the way v2 read it, every element holding elements a map
	// of them and every other one its text, because that is what the
	// collector reads back from a resize or a delete: it joins the remote
	// device, the remote array and the group number into the commands it
	// chains, so each must be the string the array printed.
	RDF struct {
		raw map[string]any
	}

	//
	XSymDevShow struct {
		XMLName   xml.Name             `xml:"SymCLI_ML" json:"-"`
		Symmetrix XSymDevShowSymmetrix `xml:"Symmetrix" json:"Symmetrix"`
	}
	XSymDevShowSymmetrix struct {
		XMLName  xml.Name      `xml:"Symmetrix" json:"-"`
		Devices  []Device      `xml:"Device" json:"Device"`
		SymmInfo SymmInfoShort `xml:"Symm_Info" json:"Symm_Info"`
	}

	//
	XSymCfgSLOList struct {
		XMLName   xml.Name                `xml:"SymCLI_ML" json:"-"`
		Symmetrix XSymCfgSLOListSymmetrix `xml:"Symmetrix" json:"Symmetrix"`
	}
	XSymCfgSLOListSymmetrix struct {
		XMLName  xml.Name      `xml:"Symmetrix" json:"-"`
		SLOs     []SLO         `xml:"SLO" json:"SLO"`
		SymmInfo SymmInfoShort `xml:"Symm_Info" json:"Symm_Info"`
	}
	SLO struct {
		XMLName xml.Name `xml:"SLO" json:"-"`
		SLOInfo SLOInfo  `xml:"SLO_Info" json:"SLO_Info"`
	}
	SLOInfo struct {
		XMLName  xml.Name `xml:"SLO_Info" json:"-"`
		Name     string   `xml:"name" json:"name"`
		BaseName string   `xml:"base_name" json:"base_name"`
	}

	//
	XSymCfgSRPList struct {
		XMLName   xml.Name                `xml:"SymCLI_ML" json:"-"`
		Symmetrix XSymCfgSRPListSymmetrix `xml:"Symmetrix" json:"Symmetrix"`
	}
	XSymCfgSRPListSymmetrix struct {
		XMLName  xml.Name      `xml:"Symmetrix" json:"-"`
		SRPs     []SRP         `xml:"SRP" json:"SRP"`
		SymmInfo SymmInfoShort `xml:"Symm_Info" json:"Symm_Info"`
	}
	SRP struct {
		XMLName xml.Name `xml:"SRP" json:"-"`
		SRPInfo SRPInfo  `xml:"SRP_Info" json:"SRP_Info"`
	}

	SRPInfo struct {
		XMLName                           xml.Name           `xml:"SRP_Info" json:"-"`
		Name                              string             `xml:"name" json:"name"`
		DefaultSRP                        string             `xml:"default_SRP" json:"default_SRP"`
		EffectiveUsedCapacityPct          int                `xml:"effective_used_capacity_pct" json:"effective_used_capacity_pct"`
		UsedCapacityGigabytes             float64            `xml:"used_capacity_gigabytes" json:"used_capacity_gigabytes"`
		UsedCapacityTerabytes             float64            `xml:"used_capacity_terabytes" json:"used_capacity_terabytes"`
		AllocatedCapacityGigabytes        float64            `xml:"allocated_capacity_gigabytes" json:"allocated_capacity_gigabytes"`
		AllocatedCapacityTerabytes        float64            `xml:"allocated_capacity_terabytes" json:"allocated_capacity_terabytes"`
		FreeCapacityGigabytes             float64            `xml:"free_capacity_gigabytes" json:"free_capacity_gigabytes"`
		FreeCapacityTerabytes             float64            `xml:"free_capacity_terabytes" json:"free_capacity_terabytes"`
		UsableCapacityGigabytes           float64            `xml:"usable_capacity_gigabytes" json:"usable_capacity_gigabytes"`
		UsableCapacityTerabytes           float64            `xml:"usable_capacity_terabytes" json:"usable_capacity_terabytes"`
		SubscribedCapacityGigabytes       float64            `xml:"subscribed_capacity_gigabytes" json:"subscribed_capacity_gigabytes"`
		SubscribedCapacityTerabytes       float64            `xml:"subscribed_capacity_terabytes" json:"subscribed_capacity_terabytes"`
		UserSubscribedCapacityGigabytes   float64            `xml:"user_subscribed_capacity_gigabytes" json:"user_subscribed_capacity_gigabytes"`
		UserSubscribedCapacityTerabytes   float64            `xml:"user_subscribed_capacity_terabytes" json:"user_subscribed_capacity_terabytes"`
		SystemSubscribedCapacityGigabytes float64            `xml:"system_subscribed_capacity_gigabytes" json:"system_subscribed_capacity_gigabytes"`
		SystemSubscribedCapacityTerabytes float64            `xml:"system_subscribed_capacity_terabytes" json:"system_subscribed_capacity_terabytes"`
		SubscribedCapacityPct             nullable.Int       `xml:"subscribed_capacity_pct" json:"subscribed_capacity_pct"`
		ResvCap                           int                `xml:"resv_cap" json:"resv_cap"`
		DiskGroups                        []SRPInfoDiskGroup `xml:"DiskGroup" json:"disk_groups"`
	}

	SRPInfoDiskGroup struct {
		XMLName       xml.Name             `xml:"Disk_Group" json:"-"`
		DiskGroupInfo SRPInfoDiskGroupInfo `xml:"Disk_Group_Info" json:"Disk_Group_Info"`
	}
	SRPInfoDiskGroupInfo struct {
		XMLName                 xml.Name `xml:"Disk_Group_Info" json:"-"`
		DiskGroupNumber         int      `xml:"disk_group_number" json:"disk_group_number"`
		DiskGroupName           string   `xml:"disk_group_name" json:"disk_group_name"`
		DiskGroupStatus         string   `xml:"disk_group_status" json:"disk_group_status"`
		Technology              string   `xml:"technology" json:"technology"`
		DiskLocation            string   `xml:"disk_location" json:"disk_location"`
		Speed                   string   `xml:"speed" json:"speed"`
		FBAPct                  int      `xml:"fba_pct" json:"fba_pct"`
		CKDPct                  int      `xml:"ckd_pct" json:"ckd_pct"`
		UsableCapacityGigabytes float64  `xml:"usable_capacity_gigabytes" json:"usable_capacity_gigabytes"`
		UsableCapacityTerabytes float64  `xml:"usable_capacity_terabytes" json:"usable_capacity_terabytes"`
		Product                 string   `xml:"product" json:"product"`
		ArrayId                 string   `xml:"array_id" json:"array_id"`
	}

	//
	XSymDevList struct {
		XMLName   xml.Name             `xml:"SymCLI_ML" json:"-"`
		Symmetrix XSymDevListSymmetrix `xml:"Symmetrix" json:"Symmetrix"`
	}
	XSymDevListSymmetrix struct {
		XMLName  xml.Name      `xml:"Symmetrix" json:"-"`
		Devices  []Device      `xml:"Device" json:"devices"`
		SymmInfo SymmInfoShort `xml:"Symm_Info" json:"Symm_Info"`
	}
	Device struct {
		XMLName  xml.Name    `xml:"Device" json:"-"`
		DevInfo  DevInfo     `xml:"Dev_Info" json:"Dev_Info"`
		Flags    DevFlags    `xml:"Flags" json:"Flags"`
		Capacity DevCapacity `xml:"Capacity" json:"Capacity"`
		FrontEnd DevFrontEnd `xml:"Front_End" json:"Front_End"`
		BackEnd  DevBackEnd  `xml:"Back_End" json:"Back_End"`
		RDF      *RDF        `xml:"RDF" json:"RDF"`
		Product  *Product    `xml:"Product" json:"Product"`
	}
	DevFlags struct {
		XMLName       xml.Name      `xml:"Flags" json:"-"`
		WORMProtected nullable.Bool `xml:"worm_protected" json:"worm_protected"`
		ACLX          nullable.Bool `xml:"aclx" json:"aclx"`
		Meta          nullable.Bool `xml:"meta" json:"meta"`
	}
	DevCapacity struct {
		XMLName   xml.Name `xml:"Capacity" json:"-"`
		Cylinders int64    `xml:"cylinders" json:"cylinders"`
		Kilobytes int64    `xml:"kilobytes" json:"kilobytes"`
		Megabytes int64    `xml:"megabytes" json:"megabytes"`
		Gigabytes float32  `xml:"gigabytes" json:"gigabytes"`
		Terabytes float32  `xml:"terabytes" json:"terabytes"`
	}
	DevFrontEnd struct {
		XMLName xml.Name     `xml:"Front_End" json:"-"`
		Port    FrontEndPort `xml:"Port" json:"Port"`
	}
	FrontEndPort struct {
		XMLName  xml.Name     `xml:"Port" json:"-"`
		Name     string       `xml:"pd_name" json:"pd_name"`
		Director string       `xml:"director" json:"director"`
		Port     nullable.Int `xml:"port" json:"port"`
	}
	BackEndDisk struct {
		XMLName   xml.Name `xml:"Disk" json:"-"`
		Director  string   `xml:"director" json:"director"`
		Interface string   `xml:"interface" json:"interface"`
		TID       string   `xml:"tid" json:"tid"`
	}
	DevBackEnd struct {
		XMLName xml.Name `xml:"Back_End" json:"-"`
	}
	DevInfo struct {
		XMLName       xml.Name `xml:"Dev_Info" json:"-"`
		DevName       string   `xml:"dev_name" json:"dev_name"`
		SRPName       string   `xml:"SRP_name" json:"srp_name"`
		Configuration string   `xml:"configuration" json:"configuration"`
		Status        string   `xml:"status" json:"status"`

		// The snapvx flags are kept as printed, and compared to "True" as
		// v2 did, so a value other than a boolean does not fail the show
		// of a device about to be deleted.
		SnapvxSource string `xml:"snapvx_source" json:"snapvx_source"`
		SnapvxTarget string `xml:"snapvx_target" json:"snapvx_target"`
	}

	//
	XSymCfgDirList struct {
		XMLName   xml.Name                `xml:"SymCLI_ML" json:"-"`
		Symmetrix XSymCfgDirListSymmetrix `xml:"Symmetrix" json:"Symmetrix"`
	}
	XSymCfgDirListSymmetrix struct {
		XMLName   xml.Name   `xml:"Symmetrix" json:"-"`
		Directors []Director `xml:"Director" json:"directors"`
	}
	Director struct {
		XMLName xml.Name  `xml:"Director" json:"-"`
		DirInfo DirInfo   `xml:"Dir_Info" json:"Dir_Info"`
		Ports   []DirPort `xml:"Port" json:"ports"`
	}
	DirPort struct {
		XMLName  xml.Name    `xml:"Port" json:"-"`
		PortInfo DirPortInfo `xml:"Port_Info" json:"Port_Info"`
	}
	DirPortInfo struct {
		XMLName         xml.Name `xml:"Port_Info" json:"-"`
		Port            int      `xml:"port" json:"port"`
		PortWWN         string   `xml:"port_wwn" json:"port_wwn"`
		PortStatus      string   `xml:"port_status" json:"port_status"`
		NegociatedSpeed string   `xml:"negociated_speed" json:"negociated_speed"`
		MaximumSpeed    string   `xml:"maximum_speed" json:"maximum_speed"`
	}
	DirInfo struct {
		XMLName   xml.Name `xml:"Dir_Info" json:"-"`
		Id        string   `xml:"id" json:"id"`
		Type      string   `xml:"type" json:"type"`
		Status    string   `xml:"status" json:"status"`
		Cores     int      `xml:"cores" json:"cores"`
		EngineNum int      `xml:"engine_num" json:"engine_num"`
		Ports     int      `xml:"ports" json:"ports"`
		Number    int      `xml:"number" json:"number"`
		Slot      int      `xml:"slot" json:"slot"`
	}

	//
	XSymCfgRDFGList struct {
		XMLName   xml.Name                 `xml:"SymCLI_ML" json:"-"`
		Symmetrix XSymCfgRDFGListSymmetrix `xml:"Symmetrix" json:"Symmetrix"`
	}
	XSymCfgRDFGListSymmetrix struct {
		XMLName   xml.Name      `xml:"Symmetrix" json:"-"`
		RDFGroups []RDFGroup    `xml:"RdfGroup" json:"rdf_groups"`
		SymmInfo  SymmInfoShort `xml:"Symm_Info" json:"Symm_Info"`
	}
	RDFGroup struct {
		XMLName          xml.Name `xml:"RdfGroup" json:"-"`
		RAGroupNum       int      `xml:"ra_group_num" json:"ra_group_num"`
		RemoteRAGroupNum int      `xml:"remote_ra_group_num" json:"remote_ra_group_num"`
		RemoteSymId      string   `xml:"remote_symid" json:"remote_symid"`
		RDFMetro         string   `xml:"rdf_metro" json:"rdf_metro"`
		RDFGroupType     string   `xml:"rdf_group_type" json:"rdf_group_type"`
	}

	//
	XSymCfgPoolList struct {
		XMLName   xml.Name                 `xml:"SymCLI_ML" json:"-"`
		Symmetrix XSymCfgPoolListSymmetrix `xml:"Symmetrix" json:"Symmetrix"`
	}
	XSymCfgPoolListSymmetrix struct {
		XMLName     xml.Name      `xml:"Symmetrix" json:"-"`
		DevicePools []DevicePool  `xml:"DevicePool" json:"device_pools"`
		SymmInfo    SymmInfoShort `xml:"Symm_Info" json:"Symm_Info"`
	}
	SymmInfoShort struct {
		XMLName xml.Name `xml:"Symm_Info" json:"-"`
		SymId   string   `xml:"symid" json:"symid"`
	}
	DevicePoolTotals struct {
		XMLName           xml.Name `xml:"Totals" json:"-"`
		TotalTracks       int64    `xml:"total_tracks" json:"total_tracks"`
		TotalUsedTracks   int64    `xml:"total_used_tracks" json:"total_used_tracks"`
		TotalFreeTracks   int64    `xml:"total_free_tracks" json:"total_free_tracks"`
		TotalTracksMB     float32  `xml:"total_tracks_mb" json:"total_tracks_mb"`
		TotalUsedTracksMB float32  `xml:"total_used_tracks_mb" json:"total_used_tracks_mb"`
		TotalFreeTracksMB float32  `xml:"total_free_tracks_mb" json:"total_free_tracks_mb"`
		TotalTracksGB     float32  `xml:"total_tracks_gb" json:"total_tracks_gb"`
		TotalUsedTracksGB float32  `xml:"total_used_tracks_gb" json:"total_used_tracks_gb"`
		TotalFreeTracksGB float32  `xml:"total_free_tracks_gb" json:"total_free_tracks_gb"`
		TotalTracksTB     float32  `xml:"total_tracks_tb" json:"total_tracks_tb"`
		TotalUsedTracksTB float32  `xml:"total_used_tracks_tb" json:"total_used_tracks_tb"`
		TotalFreeTracksTB float32  `xml:"total_free_tracks_tb" json:"total_free_tracks_tb"`
		PercentFull       int      `xml:"percent_full" json:"percent_full"`
	}
	DevicePool struct {
		XMLName       xml.Name         `xml:"DevicePool" json:"-"`
		Name          string           `xml:"pool_name" json:"pool_name"`
		Type          string           `xml:"pool_type" json:"pool_type"`
		DiskLocation  string           `xml:"disk_location" json:"pool_location"`
		Technology    string           `xml:"technology" json:"technology"`
		DevEmulation  string           `xml:"dev_emulation" json:"dev_emulation"`
		Configuration string           `xml:"configuration" json:"configuration"`
		Devs          int              `xml:"pool_devs" json:"pool_devs"`
		State         string           `xml:"pool_state" json:"pool_state"`
		Totals        DevicePoolTotals `xml:"Totals" json:"totals"`
	}

	//
	XSymSGList struct {
		XMLName xml.Name `xml:"SymCLI_ML" json:"-"`
		SG      SG       `xml:"SG" json:"SG"`
	}
	SG struct {
		XMLName xml.Name `xml:"SG" json:"-"`
		SGInfos []SGInfo `xml:"SG_Info" json:"SG_Info"`
	}
	SGInfo struct {
		XMLName  xml.Name `xml:"SG_Info" json:"-"`
		Name     string   `xml:"name" json:"name"`
		SLOName  string   `xml:"SLO_name" json:"SLO_name"`
		SRPName  string   `xml:"SRP_name" json:"SRP_name"`
		SymID    string   `xml:"symid" json:"symid"`
		NumOfGKs int      `xml:"Num_of_GKS" json:"Num_of_GKS"`
	}

	//
	XSymCfgList struct {
		XMLName   xml.Name             `xml:"SymCLI_ML" json:"-"`
		Symmetrix XSymCfgListSymmetrix `xml:"Symmetrix" json:"Symmetrix"`
	}
	XSymCfgListSymmetrix struct {
		XMLName  xml.Name `xml:"Symmetrix" json:"-"`
		SymmInfo SymmInfo `xml:"Symm_Info" json:"Symm_Info"`
	}
	SymmInfo struct {
		XMLName          xml.Name `xml:"Symm_Info" json:"-"`
		SymId            string   `xml:"symid" json:"symid"`
		Attachment       string   `xml:"attachment" json:"attachment"`
		Model            string   `xml:"model" json:"model"`
		MicrocodeVersion string   `xml:"microcode_version" json:"microcode_version"`
		CacheMegabytes   int64    `xml:"cache_megabytes" json:"cache_megabytes"`
		PhysicalDevices  int      `xml:"physical_devices" json:"physical_devices"`
	}

	//
	Product struct {
		XMLName  xml.Name `xml:"Product" json:"-"`
		Vendor   string   `xml:"vendor" json:"vendor"`
		Name     string   `xml:"name" json:"name"`
		Revision string   `xml:"revision" json:"revision"`
		SerialId string   `xml:"serial_id" json:"serial_id"`
		SymId    string   `xml:"symid" json:"symid"`
		WWN      string   `xml:"wwn" json:"wwn"`
		DeviceId string   `xml:"device_id" json:"device_id"`
	}

	// mappingSG is a storage group a mapping reaches, through a view
	// presenting the devices of that group to the initiator on the target.
	mappingSG struct {
		name           string
		initiatorCount int
	}
)

var (
	// PromptReader, when a developer sets it, has every symcli command
	// confirmed on it before it runs, to try a dangerous command by hand.
	//
	// It is nil otherwise, and must stay nil for an action the collector
	// queues: the prompt is written on stdout, which the collector reads as
	// the json result of the action, and the action has no stdin to answer
	// it, so the prompt would loop on the end of file for ever.
	PromptReader *bufio.Reader

	ErrNotFree = errors.New("device is not free")

	// retryDelay is the wait between two tries of a device deletion the
	// array refused for the allocations the device still holds, and between
	// two checks of a free in progress. A test shortens it.
	retryDelay = 5 * time.Second
)

const (
	// cylinderKB is the size of a cylinder of the arrays this driver
	// handles, the vmax3, the vmax all flash and the powermax, whose
	// cylinders are 15 tracks of 128 KB. Sizes are sent to the array in
	// cylinders, as v2 sent them, so a resize compares like with like.
	cylinderKB = 1920

	// freeMaxTries bounds the wait for the free of a device, at retryDelay
	// per try: an hour at the default delay. v2 waited for ever.
	freeMaxTries = 720

	// deleteMaxTries is how many times a deletion refused for the
	// allocations the device holds is tried, as v2 tried it.
	deleteMaxTries = 5

	// defaultGKCount is the number of gatekeepers a masking plan gives a
	// storage group naming no count, as v2 gave it.
	defaultGKCount = 6
)

func init() {
	driver.Register(driver.NewID(driver.GroupArray, "symmetrix"), NewDriver)
}

func NewDriver() array.Driver {
	t := New()
	var i any = t
	return i.(array.Driver)
}

func New() *Array {
	t := &Array{
		Array: array.New(),
	}
	return t
}

func (t *Array) Log() *plog.Logger {
	if t.log == nil {
		t.log = plog.NewDefaultLogger().Attr("symid", t.kwSID()).Attr("driver", "array.symmetrix")
	}
	return t.log
}

// Run builds the command tree of this array and runs the arguments through
// it. What the tree holds is declared in Actions.
func (t *Array) Run(args []string) error {
	return array.RunActions(context.Background(), t.Actions(), args, os.Stdout)
}

// symcliVersion returns the version of the installed symcli, which the
// symcli program prints when run with no argument.
func (t *Array) symcliVersion(ctx context.Context) (int, int, error) {
	result, err := t.symResult(ctx, zerolog.TraceLevel, "symcli")
	if err != nil {
		return 0, 0, err
	}
	return t.parseSymcliVersion([]byte(result.Out))
}

func (t *Array) parseSymcliVersion(b []byte) (major int, minor int, err error) {
	pattern := regexp.MustCompile(`\(SYMCLI\)\sVersion V(\d+)\.(\d+)`)
	m := pattern.FindStringSubmatch(string(b))
	if len(m) != 3 {
		return 0, 0, fmt.Errorf("no symcli version found in: %s", string(b))
	}
	if major, err = strconv.Atoi(m[1]); err != nil {
		return
	}
	if minor, err = strconv.Atoi(m[2]); err != nil {
		return
	}
	return
}

// symcliNewerThan is true when the installed symcli is known to be newer
// than major.minor. A version that can not be read is not newer, which keeps
// the steps an older symcli needs, as v2 kept them.
func (t *Array) symcliNewerThan(ctx context.Context, major, minor int) bool {
	vMajor, vMinor, err := t.symcliVersion(ctx)
	if err != nil {
		t.Log().Infof("symcli version unknown: %s", err)
		return false
	}
	return vMajor > major || (vMajor == major && vMinor > minor)
}

func (t *Array) kwSID() string {
	if s := t.Config().GetString(t.Key("name")); s != "" {
		return s
	}
	rid, err := resourceid.Parse(t.Name())
	if err != nil {
		return ""
	}
	return rid.Index()
}

func (t *Array) kwSymcliPath() string {
	s := t.Config().GetString(t.Key("symcli_path"))
	if filepath.Base(s) != "bin" {
		s = filepath.Join(s, "bin")
	}
	return s
}

func (t *Array) kwSymcliConnect() string {
	return t.Config().GetString(t.Key("symcli_connect"))
}

func dump(data any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "    ")
	return enc.Encode(data)
}

func (t *Array) PrepareEnv() error {
	key := "SYMCLI_CONNECT"
	connectExpected := t.kwSymcliConnect()
	connectActual := os.Getenv(key)

	switch {
	case connectExpected == "" && connectActual != "":
		if err := os.Unsetenv(key); err != nil {
			return err
		}
	case connectExpected != "" && connectActual == "":
		if err := os.Setenv(key, connectExpected); err != nil {
			return err
		}
	}
	return nil
}

func (t *Array) MaskDBFile() (string, error) {
	s := os.Getenv("SYMCLI_DB_FILE")
	if s == "" {
		return "", nil
	}
	dir := filepath.Dir(s)
	sid := t.kwSID()
	if sid == "" {
		return "", fmt.Errorf("array name is required")
	}
	p := filepath.Join(dir, sid+".bin")
	_, err := os.Stat(p)
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		return "", err
	default:
		return p, nil
	}
	p = filepath.Join(dir, sid, "symmaskdb_backup.bin")
	switch {
	case err != nil:
		return "", err
	default:
		return p, nil
	}
}

func (t *Array) SymAccessShowViewDetail(ctx context.Context, sid, name string) ([]MaskingView, error) {
	b, err := t.symXML(ctx, "symaccess", sid, "show", "view", name, "-detail")
	if err != nil {
		return nil, err
	}
	return t.parseSymAccessListViewDetail(b)
}

func (t *Array) SymAccessListViewDetail(ctx context.Context) ([]MaskingView, error) {
	b, err := t.symXML(ctx, "symaccess", t.kwSID(), "list", "view", "-detail")
	if err != nil {
		return nil, err
	}
	return t.parseSymAccessListViewDetail(b)
}

func (t *Array) parseSymAccessListViewDetail(b []byte) ([]MaskingView, error) {
	var head XSymAccessListViewDetail
	if err := xml.Unmarshal(b, &head); err != nil {
		return nil, err
	}
	return head.Symmetrix.MaskingViews, nil
}

func (t *Array) SymCfgList(ctx context.Context, sid string) (SymmInfo, error) {
	if sid == "" {
		sid = t.kwSID()
	}
	b, err := t.symXML(ctx, "symcfg", sid, "list")
	if err != nil {
		return SymmInfo{}, err
	}
	return t.parseSymCfgList(b)
}

func (t *Array) parseSymCfgList(b []byte) (SymmInfo, error) {
	var head XSymCfgList
	if err := xml.Unmarshal(b, &head); err != nil {
		return SymmInfo{}, err
	}
	return head.Symmetrix.SymmInfo, nil
}

func (t *Array) SymCfgDirectorList(ctx context.Context, s string) ([]Director, error) {
	b, err := t.symXML(ctx, "symcfg", t.kwSID(), "-dir", s, "-v", "list")
	if err != nil {
		return nil, err
	}
	return t.parseSymCfgDirectorList(b)
}

func (t *Array) parseSymCfgDirectorList(b []byte) ([]Director, error) {
	var head XSymCfgDirList
	if err := xml.Unmarshal(b, &head); err != nil {
		return nil, err
	}
	return head.Symmetrix.Directors, nil
}

func (t *Array) SymCfgRDFGList(ctx context.Context, sid, s string) ([]RDFGroup, error) {
	if sid == "" {
		sid = t.kwSID()
	}
	b, err := t.symXML(ctx, "symcfg", sid, "-rdfg", s, "list")
	if err != nil {
		return nil, err
	}
	return t.parseSymCfgRDFGList(b)
}

func (t *Array) parseSymCfgRDFGList(b []byte) ([]RDFGroup, error) {
	var head XSymCfgRDFGList
	if err := xml.Unmarshal(b, &head); err != nil {
		return nil, err
	}
	return head.Symmetrix.RDFGroups, nil
}

func (t *Array) SymCfgPoolList(ctx context.Context) ([]DevicePool, error) {
	b, err := t.symXML(ctx, "symcfg", t.kwSID(), "-pool", "list", "-v")
	if err != nil {
		return nil, err
	}
	return t.parseSymCfgPoolList(b)
}

func (t *Array) parseSymCfgPoolList(b []byte) ([]DevicePool, error) {
	var head XSymCfgPoolList
	if err := xml.Unmarshal(b, &head); err != nil {
		return nil, err
	}
	return head.Symmetrix.DevicePools, nil
}

func (t *Array) SymCfgSLOList(ctx context.Context) ([]SLO, error) {
	b, err := t.symXML(ctx, "symcfg", t.kwSID(), "list", "-slo", "-detail", "-v")
	if err != nil {
		return nil, err
	}
	return t.parseSymCfgSLOList(b)
}

func (t *Array) parseSymCfgSLOList(b []byte) ([]SLO, error) {
	var head XSymCfgSLOList
	if err := xml.Unmarshal(b, &head); err != nil {
		return nil, err
	}
	return head.Symmetrix.SLOs, nil
}

func (t *Array) SymCfgSRPList(ctx context.Context) ([]SRP, error) {
	b, err := t.symXML(ctx, "symcfg", t.kwSID(), "list", "-srp", "-detail", "-v")
	if err != nil {
		return nil, err
	}
	return t.parseSymCfgSRPList(b)
}

func (t *Array) parseSymCfgSRPList(b []byte) ([]SRP, error) {
	var head XSymCfgSRPList
	if err := xml.Unmarshal(b, &head); err != nil {
		return nil, err
	}
	return head.Symmetrix.SRPs, nil
}

func (t *Array) SymDiskListDiskGroupSummary(ctx context.Context) ([]DiskGroup, error) {
	b, err := t.symXML(ctx, "symdisk", t.kwSID(), "list", "-dskgroup_summary")
	if err != nil {
		return nil, err
	}
	return t.parseSymDiskListDiskGroupSummary(b)
}

func (t *Array) parseSymDiskListDiskGroupSummary(b []byte) ([]DiskGroup, error) {
	var head XSymDiskListDiskGroupSummary
	if err := xml.Unmarshal(b, &head); err != nil {
		return nil, err
	}
	return head.Symmetrix.DiskGroups, nil
}

// SymCfgListThinDevs returns the allocations of a thin device, read as v2
// read them.
func (t *Array) SymCfgListThinDevs(ctx context.Context, sid, devId string) ([]ThinDev, error) {
	b, err := t.symXML(ctx, "symcfg", sid, "list", "-tdevs", "-devs", devId)
	if err != nil {
		return nil, err
	}
	return t.parseSymCfgListThinDevs(b)
}

func (t *Array) parseSymCfgListThinDevs(b []byte) ([]ThinDev, error) {
	var head XSymDevListThinDevs
	if err := xml.Unmarshal(b, &head); err != nil {
		return nil, err
	}
	return head.Symmetrix.ThinDevs, nil
}

func (t *Array) SymDevShow(ctx context.Context, sid, devId string) ([]Device, error) {
	b, err := t.symXML(ctx, "symdev", sid, "show", devId)
	if err != nil {
		return nil, err
	}
	return t.parseSymDevShow(b)
}

func (t *Array) parseSymDevShow(b []byte) ([]Device, error) {
	var head XSymDevShow
	if err := xml.Unmarshal(b, &head); err != nil {
		return nil, err
	}
	return head.Symmetrix.Devices, nil
}

func (t *Array) SymDevShowByWWN(ctx context.Context, sid, wwn string) ([]Device, error) {
	b, err := t.symXML(ctx, "symdev", sid, "show", "-wwn", wwn)
	if err != nil {
		return nil, err
	}
	return t.parseSymDevShow(b)
}

func (t *Array) SymDevList(ctx context.Context, sid string) ([]Device, error) {
	if sid == "" {
		sid = t.kwSID()
	}
	b, err := t.symXML(ctx, "symdev", sid, "list")
	if err != nil {
		return nil, err
	}
	return t.parseSymDevList(b)
}

func (t *Array) parseSymDevList(b []byte) ([]Device, error) {
	var head XSymDevList
	if err := xml.Unmarshal(b, &head); err != nil {
		return nil, err
	}
	return head.Symmetrix.Devices, nil
}

// devWWN returns the device as "symdev list -devs <dev> -wwn" prints it,
// and its wwn, which is what v2 returned as the disk id and the driver data
// of the device of a disk it added or mapped.
func (t *Array) devWWN(ctx context.Context, sid, devId string) (map[string]any, string, error) {
	b, err := t.symXML(ctx, "symdev", sid, "list", "-devs", devId, "-wwn")
	if err != nil {
		return nil, "", err
	}
	root, err := parseXMLNode(b)
	if err != nil {
		return nil, "", err
	}
	devs := root.findAll("Device")
	if len(devs) == 0 {
		return nil, "", fmt.Errorf("dev %s not found in the wwn listing of array %s", devId, sid)
	}
	data := devs[0].v2Map()
	wwn, _ := data["wwn"].(string)
	if wwn == "" {
		return nil, "", fmt.Errorf("dev %s has no wwn in the wwn listing of array %s", devId, sid)
	}
	return data, wwn, nil
}

// isPowerMax is true for a powermax array, which resizes a device in a SRDF
// pair without the pair being deleted first. v2 told the models apart the
// same way.
func (t *Array) isPowerMax(ctx context.Context, sid string) (bool, error) {
	info, err := t.SymCfgList(ctx, sid)
	if err != nil {
		return false, err
	}
	if info.Model == "" {
		return false, fmt.Errorf("array %s: no model in the symcfg list output", sid)
	}
	return strings.HasPrefix(info.Model, "PowerMax"), nil
}

func (t *Array) getSG(ctx context.Context, sid, name string) (SGInfo, error) {
	l, err := t.SymSGShow(ctx, sid, name)
	if err != nil {
		return SGInfo{}, err
	}
	if len(l) == 0 {
		return SGInfo{}, fmt.Errorf("storage group %s: %w", name, os.ErrNotExist)
	}
	return l[0], nil
}

func (t *Array) SymSGShow(ctx context.Context, sid, name string) ([]SGInfo, error) {
	b, err := t.symSGShow(ctx, sid, name)
	if err != nil {
		return nil, err
	}
	return t.parseSymSGList(b)
}

func (t *Array) symSGShow(ctx context.Context, sid, name string) ([]byte, error) {
	if sid == "" {
		sid = t.kwSID()
	}
	return t.symXML(ctx, "symsg", sid, "show", name)
}

func (t *Array) SymSGList(ctx context.Context, sid string) ([]SGInfo, error) {
	if sid == "" {
		sid = t.kwSID()
	}
	b, err := t.symXML(ctx, "symsg", sid, "list", "-v")
	if err != nil {
		return nil, err
	}
	return t.parseSymSGList(b)
}

// parseSymSGList returns the storage groups of a symsg list or show
// output, found at any depth, as v2 found them in both.
func (t *Array) parseSymSGList(b []byte) ([]SGInfo, error) {
	l := make([]SGInfo, 0)
	d := xml.NewDecoder(bytes.NewReader(b))
	for {
		tok, err := d.Token()
		if errors.Is(err, io.EOF) {
			return l, nil
		}
		if err != nil {
			return nil, err
		}
		e, ok := tok.(xml.StartElement)
		if !ok || e.Name.Local != "SG_Info" {
			continue
		}
		var info SGInfo
		if err := d.DecodeElement(&info, &e); err != nil {
			return nil, err
		}
		l = append(l, info)
	}
}

// getDev returns the device devId names, a device name or, longer than 6
// characters, a wwn, as v2 resolved it.
func (t *Array) getDev(ctx context.Context, sid, devId string) (Device, error) {
	var (
		devs []Device
		err  error
	)
	if devId == "" {
		return Device{}, fmt.Errorf("--dev is required")
	}
	if len(devId) > 6 {
		devs, err = t.SymDevShowByWWN(ctx, sid, devId)
	} else {
		devs, err = t.SymDevShow(ctx, sid, devId)
	}
	if err != nil {
		return Device{}, err
	}
	if len(devs) == 0 {
		return Device{}, fmt.Errorf("dev %s: %w", devId, os.ErrNotExist)
	}
	if devs[0].DevInfo.DevName == "" {
		return Device{}, fmt.Errorf("dev %s: no device name in the symdev show output", devId)
	}
	return devs[0], nil
}

func (t *Array) addThinDevToSG(ctx context.Context, sid, devId, sg string) error {
	if devId == "" {
		return fmt.Errorf("a dev id is required to add tdev to sg")
	}
	if sg == "" {
		return fmt.Errorf("a sg name is required to add tdev to sg")
	}
	_, err := t.symOn(ctx, "symaccess", sid, "-name", sg, "-type", "storage", "add", "dev", devId)
	return err
}

func (t *Array) removeThinDevFromSG(ctx context.Context, sid, devId, sg string) error {
	_, err := t.symOn(ctx, "symaccess", sid, "-name", sg, "-type", "storage", "remove", "dev", devId, "-unmap")
	return err
}

func (t *Array) SymAccessShowPort(ctx context.Context, sid, name string) (ShowPortGroup, error) {
	b, err := t.symXML(ctx, "symaccess", sid, "show", name, "-type", "port")
	if err != nil {
		return ShowPortGroup{}, err
	}
	return t.parseSymAccessShowPort(b)
}

func (t *Array) parseSymAccessShowPort(b []byte) (ShowPortGroup, error) {
	var head XSymAccessShowPort
	if err := xml.Unmarshal(b, &head); err != nil {
		return ShowPortGroup{}, err
	}
	return head.Symmetrix.PortGroup, nil
}

func (t *Array) SymAccessListPort(ctx context.Context, sid string) ([]PortGroup, error) {
	b, err := t.symXML(ctx, "symaccess", sid, "list", "-type", "port")
	if err != nil {
		return nil, err
	}
	return t.parseSymAccessListPort(b)
}

func (t *Array) parseSymAccessListPort(b []byte) ([]PortGroup, error) {
	var head XSymAccessListPort
	if err := xml.Unmarshal(b, &head); err != nil {
		return nil, err
	}
	return head.Symmetrix.PortGroups, nil
}

func (t *Array) SymAccessListDevInitiator(ctx context.Context, sid, wwn string) ([]InitiatorGroup, error) {
	b, err := t.symXML(ctx, "symaccess", sid, "list", "-type", "initiator", "-wwn", wwn)
	if err != nil {
		return nil, err
	}
	return t.parseSymAccessListDevInitiator(b)
}

func (t *Array) parseSymAccessListDevInitiator(b []byte) ([]InitiatorGroup, error) {
	var head XSymAccessListDevInitiator
	if err := xml.Unmarshal(b, &head); err != nil {
		return nil, err
	}
	return head.Symmetrix.InitiatorGroups, nil
}

// getInitiatorViewNames returns the names of the views presenting devices
// to the initiator, sorted so the views are walked in the same order from
// one run to the next.
func (t *Array) getInitiatorViewNames(ctx context.Context, sid, wwn string) ([]string, error) {
	igs, err := t.SymAccessListDevInitiator(ctx, sid, wwn)
	if err != nil {
		return nil, err
	}
	m := make(map[string]any)
	for _, ig := range igs {
		for _, name := range ig.GroupInfo.MaskViewNames.ViewNames {
			name = strings.TrimRight(name, " *")
			if name != "" {
				m[name] = nil
			}
		}
	}
	l := maps.Keys(m)
	sort.Strings(l)
	return l, nil
}

func (t *Array) getView(ctx context.Context, sid, name string) (MaskingView, error) {
	views, err := t.SymAccessShowViewDetail(ctx, sid, name)
	if err != nil {
		return MaskingView{}, err
	}
	if len(views) == 0 {
		return MaskingView{}, fmt.Errorf("masking view '%s' does not exist", name)
	}
	return views[0], nil
}

// viewCache keeps the views read during one action, which reads a view once
// per target of a mapping otherwise.
type viewCache map[string]MaskingView

func (t *Array) getCachedView(ctx context.Context, cache viewCache, sid, name string) (MaskingView, error) {
	if view, ok := cache[name]; ok {
		return view, nil
	}
	view, err := t.getView(ctx, sid, name)
	if err != nil {
		return view, err
	}
	cache[name] = view
	return view, nil
}

// hasPort is true when the view presents its devices on the target port.
func (t ViewInfo) hasPort(tgtId string) bool {
	for _, port := range t.PortInfo.DirectorIdentifications {
		if strings.EqualFold(port.PortWWN, tgtId) {
			return true
		}
	}
	return false
}

// initiatorCount is the number of initiators the view presents its devices
// to, which tells a storage group shared by many hosts from one dedicated to
// few.
func (t ViewInfo) initiatorCount() int {
	n := 0
	for _, initiator := range t.InitiatorList.Initiators {
		if initiator.WWN != nil {
			n++
		}
	}
	return n
}

// storageGroupNames returns the storage groups whose devices the view
// presents: the children of its storage group when it is a parent, or its
// storage group, as v2 read them.
func (t ViewInfo) storageGroupNames() []string {
	if len(t.SGChildInfo.SG) > 0 {
		l := make([]string, 0, len(t.SGChildInfo.SG))
		for _, sg := range t.SGChildInfo.SG {
			if sg.GroupName != "" {
				l = append(l, sg.GroupName)
			}
		}
		return l
	}
	if t.StorGrpName == "" {
		return nil
	}
	return []string{t.StorGrpName}
}

// bestSG returns the storage group to put a device into for it to be
// presented on every mapping, or "" when no mapping is asked for.
//
// It is a storage group every mapping reaches, of the requested pool and
// service level when they are given, and among those the one presented to
// the fewest initiators, so a device for one host does not end up presented
// to many. Groups presented to as many initiators are told apart by name,
// so the same request lands in the same group every time.
func (t *Array) bestSG(ctx context.Context, sid string, mappings array.Mappings, slo, srp string) (string, error) {
	if len(mappings) == 0 {
		return "", nil
	}
	m, err := t.getStorageGroupOfMappings(ctx, sid, mappings)
	if err != nil {
		return "", err
	}
	if len(m) == 0 {
		return "", fmt.Errorf("no storage group found for the requested mappings %s", strings.Join(mappingKeys(mappings), " "))
	}
	l := make([]mappingSG, 0, len(m))
	for _, sg := range m {
		l = append(l, sg)
	}
	sort.Slice(l, func(i, j int) bool { return l[i].name < l[j].name })
	if slo != "" || srp != "" {
		l, err = t.filterMappingsSGs(ctx, l, sid, slo, srp)
		if err != nil {
			return "", err
		}
		if len(l) == 0 {
			return "", fmt.Errorf("no storage group found for the requested mappings %s with srp '%s' and slo '%s'", strings.Join(mappingKeys(mappings), " "), srp, slo)
		}
	}
	sort.Slice(l, func(i, j int) bool {
		if l[i].initiatorCount != l[j].initiatorCount {
			return l[i].initiatorCount < l[j].initiatorCount
		}
		return l[i].name < l[j].name
	})
	names := make([]string, len(l))
	for i, sg := range l {
		names[i] = fmt.Sprintf("%s(%d initiators)", sg.name, sg.initiatorCount)
	}
	t.Log().Infof("candidates sgs: %s, retain: %s", strings.Join(names, " "), l[0].name)
	return l[0].name, nil
}

// mappingKeys returns the mappings as "<hba>:<tgt>", sorted.
func mappingKeys(mappings array.Mappings) []string {
	l := maps.Keys(mappings)
	sort.Strings(l)
	return l
}

func (t *Array) filterMappingsSGs(ctx context.Context, l []mappingSG, sid string, slo, srp string) ([]mappingSG, error) {
	filtered := make([]mappingSG, 0, len(l))
	for _, e := range l {
		sg, err := t.getSG(ctx, sid, e.name)
		if err != nil {
			return nil, err
		}
		if (srp != "") && (sg.SRPName != srp) {
			t.Log().Infof("discard sg %s (srp %s, required %s)", e.name, sg.SRPName, srp)
			continue
		}
		if (slo != "") && (sg.SLOName != slo) {
			t.Log().Infof("discard sg %s (slo %s, required %s)", e.name, sg.SLOName, slo)
			continue
		}
		filtered = append(filtered, e)
	}
	return filtered, nil
}

// getStorageGroupOfMappings returns the storage groups every mapping
// reaches.
func (t *Array) getStorageGroupOfMappings(ctx context.Context, sid string, mappings array.Mappings) (map[string]mappingSG, error) {
	var m map[string]mappingSG
	cache := make(viewCache)
	viewNames := make(map[string][]string)
	for _, k := range mappingKeys(mappings) {
		mapping := mappings[k]
		this, err := t.getStorageGroupOfMapping(ctx, cache, viewNames, sid, mapping.HBAID, mapping.TGTID)
		if err != nil {
			return nil, err
		}
		if m == nil {
			m = this
			continue
		}
		for name := range m {
			if _, ok := this[name]; !ok {
				delete(m, name)
			}
		}
	}
	return m, nil
}

// getStorageGroupOfMapping returns the storage groups whose devices a view
// presents to the initiator hbaId on the target tgtId.
//
// Each view is judged on its own ports: a view of the initiator not
// presenting on that target offers none of its storage groups, even when
// another view of the same initiator does present on it.
func (t *Array) getStorageGroupOfMapping(ctx context.Context, cache viewCache, viewNames map[string][]string, sid, hbaId, tgtId string) (map[string]mappingSG, error) {
	m := make(map[string]mappingSG)
	names, ok := viewNames[hbaId]
	if !ok {
		var err error
		names, err = t.getInitiatorViewNames(ctx, sid, hbaId)
		if err != nil {
			return nil, err
		}
		viewNames[hbaId] = names
	}
	for _, viewName := range names {
		view, err := t.getCachedView(ctx, cache, sid, viewName)
		if err != nil {
			return nil, err
		}
		if !view.ViewInfo.hasPort(tgtId) {
			continue
		}
		initiatorCount := view.ViewInfo.initiatorCount()
		for _, sgName := range view.ViewInfo.storageGroupNames() {
			if _, ok := m[sgName]; ok {
				continue
			}
			m[sgName] = mappingSG{
				name:           sgName,
				initiatorCount: initiatorCount,
			}
		}
	}
	return m, nil
}

// resolveSG returns the storage group a device is to be put into: the one
// named, once known to exist, or the best one for the mappings.
func (t *Array) resolveSG(ctx context.Context, sid, sg string, mappings array.Mappings, slo, srp string) (string, error) {
	if sg != "" {
		if _, err := t.getSG(ctx, sid, sg); err != nil {
			return "", err
		}
		return sg, nil
	}
	return t.bestSG(ctx, sid, mappings, slo, srp)
}

func (t *Array) SymAccessListDevStorage(ctx context.Context, sid, devId string) ([]StorageGroup, error) {
	b, err := t.symXML(ctx, "symaccess", sid, "list", "-type", "storage", "-devs", devId)
	if err != nil {
		return nil, err
	}
	return t.parseSymAccessListDevStorage(b)
}

func (t *Array) parseSymAccessListDevStorage(b []byte) ([]StorageGroup, error) {
	var head XSymAccessListDevStorage
	if err := xml.Unmarshal(b, &head); err != nil {
		return nil, err
	}
	return head.Symmetrix.StorageGroups, nil
}

func (t *Array) getDevSGs(ctx context.Context, sid, devId string) ([]StorageGroup, error) {
	sgs, err := t.SymAccessListDevStorage(ctx, sid, devId)
	if err != nil {
		return nil, err
	}
	l := make([]StorageGroup, 0)
	for _, sg := range sgs {
		if sg.GroupInfo.Status != "IsParent" {
			l = append(l, sg)
		}
	}
	return l, nil
}

// viewNames returns the views presenting the devices of the storage group,
// itself or through its parent, sorted.
func (t StorageGroupInfo) viewNames() []string {
	m := make(map[string]any)
	for _, l := range [][]string{t.MaskViewNames.ViewNames, t.CascadedViewNames.ViewNames} {
		for _, name := range l {
			name = strings.TrimRight(name, " *")
			if name != "" {
				m[name] = nil
			}
		}
	}
	l := maps.Keys(m)
	sort.Strings(l)
	return l
}

func (t *Array) RenameDisk(ctx context.Context, opt OptRenameDisk) (Device, error) {
	if opt.SID == "" {
		opt.SID = t.kwSID()
	}
	if opt.Name == "" {
		return Device{}, fmt.Errorf("--name is required")
	}
	dev, err := t.getDev(ctx, opt.SID, opt.Dev)
	if err != nil {
		return dev, err
	}
	if _, err := t.symOn(ctx, "symdev", opt.SID, "set", "dev", dev.DevInfo.DevName, "-attribute", "device_name="+opt.Name); err != nil {
		return dev, err
	}
	return t.getDev(ctx, opt.SID, dev.DevInfo.DevName)
}

// cylinders returns the number of cylinders a size in bytes is sent to the
// array as, at least one, rounded down as v2 rounded it.
func cylinders(bytes int64) int64 {
	n := bytes / (cylinderKB * 1024)
	if n < 1 {
		return 1
	}
	return n
}

// currentCylinders returns the size of the device in cylinders, once sure
// its cylinders are the size this driver sends sizes in: a resize computed
// in cylinders of another size would not be the size asked for.
func (t Device) currentCylinders() (int64, error) {
	c := t.Capacity
	if c.Cylinders <= 0 {
		return 0, fmt.Errorf("dev %s: no cylinder count in the symdev show output", t.DevInfo.DevName)
	}
	if c.Kilobytes != c.Cylinders*cylinderKB {
		return 0, fmt.Errorf("dev %s: %d KB in %d cylinders is not %d KB per cylinder, the cylinder size this driver resizes in", t.DevInfo.DevName, c.Kilobytes, c.Cylinders, cylinderKB)
	}
	return c.Cylinders, nil
}

type (
	// ResizeResult is what a resize returns, in the shape of v2, which the
	// collector reads to chain the resize of the remote device of a SRDF
	// pair and the recreation of the pair.
	ResizeResult struct {
		DriverData ResizeDriverData `json:"driver_data"`
	}
	ResizeDriverData struct {
		// PairDeleted is true when the SRDF pair of the device was deleted
		// for the resize, and is to be recreated once the remote device is
		// resized too.
		PairDeleted bool `json:"pair_deleted"`

		// RDF is the pairing of the device before the resize, when the
		// device is in a SRDF pair.
		RDF *RDF `json:"rdf,omitempty"`
	}
)

// ResizeDisk resizes a device, v2's way: the new size is given or added to
// the current size, a shrink is refused unless truncating is allowed, and
// the SRDF pair of the device is deleted first on an array that can not
// resize a paired device.
func (t *Array) ResizeDisk(ctx context.Context, opt OptResizeDisk) (ResizeResult, error) {
	var result ResizeResult
	if opt.SID == "" {
		opt.SID = t.kwSID()
	}
	if opt.Size == "" {
		return result, fmt.Errorf("--size is required")
	}
	size, err := array.ParseSize(opt.Size)
	if err != nil {
		return result, err
	}
	dev, err := t.getDev(ctx, opt.SID, opt.Dev)
	if err != nil {
		return result, err
	}
	devName := dev.DevInfo.DevName
	current, err := dev.currentCylinders()
	if err != nil {
		return result, err
	}
	var target int64
	switch {
	case size.Relative && size.Bytes == 0:
		// "+0" grows nothing: the one cylinder a size rounds up to is for a
		// growth asked for, not for none.
		target = current
	case size.Relative:
		target = current + cylinders(size.Bytes)
	default:
		target = cylinders(size.Bytes)
	}
	const cylinderBytes = cylinderKB * 1024
	if err := array.CheckResize(current*cylinderBytes, target*cylinderBytes, opt.Truncate); err != nil {
		return result, fmt.Errorf("dev %s: %d cylinders to %d: %w", devName, current, target, err)
	}
	rdf := dev.paired()
	result.DriverData.RDF = rdf
	if target == current {
		// Nothing to do, and the pair is left alone: deleting it to resize
		// nothing would leave a pair for the collector to recreate.
		t.Log().Infof("dev %s is already %d cylinders", devName, current)
		return result, nil
	}
	args := []string{"modify", devName, "-tdev", "-cap", fmt.Sprint(target), "-captype", "cyl", "-noprompt"}
	if rdf != nil {
		powerMax, err := t.isPowerMax(ctx, opt.SID)
		if err != nil {
			return result, err
		}
		if powerMax {
			rdfg := rdf.RAGroupNum()
			if rdfg == "" {
				return result, fmt.Errorf("dev %s: no rdf group in its srdf pairing", devName)
			}
			args = append(args, "-rdfg", rdfg)
		} else {
			if _, err := t.deletePair(ctx, opt.SID, dev); err != nil {
				return result, err
			}
			result.DriverData.PairDeleted = true
		}
	}
	if _, err := t.symOn(ctx, "symdev", opt.SID, args...); err != nil {
		if result.DriverData.PairDeleted {
			return result, fmt.Errorf("%w: the srdf pair of dev %s with dev %s of array %s in rdf group %s was deleted for the resize and is not recreated", err, devName, rdf.RemoteDev(), rdf.RemoteSID(), rdf.RAGroupNum())
		}
		return result, err
	}
	return result, nil
}

func (t *Array) IsThinDevFreed(ctx context.Context, sid, devId string) (bool, error) {
	devs, err := t.SymCfgListThinDevs(ctx, sid, devId)
	if err != nil {
		return false, err
	}
	if len(devs) == 0 {
		// v2 read a device absent from the listing as freed.
		return true, nil
	}
	t.Log().Infof("device %s has %s tracks allocated", devId, devs[0].AllocTracks)
	return strings.TrimSpace(devs[0].AllocTracks) == "0", nil
}

// FreeThinDev frees the allocations of a device, which a symcli up to 9.1
// requires before deleting it.
func (t *Array) FreeThinDev(ctx context.Context, opt OptFreeThinDev) error {
	if t.symcliNewerThan(ctx, 9, 1) {
		t.Log().Infof("skip tdev free: symcli is newer than 9.1")
		return nil
	}
	if opt.SID == "" {
		opt.SID = t.kwSID()
	}
	dev, err := t.getDev(ctx, opt.SID, opt.Dev)
	if err != nil {
		return err
	}
	return t.freeThinDev(ctx, opt.SID, dev.DevInfo.DevName)
}

// freeThinDev frees the allocations of a device and waits for the free to
// complete, as v2 did: the free is asked for again on each check, and its
// own outcome is only logged, the state of the device being what tells a
// free done.
func (t *Array) freeThinDev(ctx context.Context, sid, devId string) error {
	var lastErr error
	for i := 1; ; i++ {
		if _, err := t.symOn(ctx, "symdev", sid, "free", "-devs", devId, "-all", "-noprompt"); err != nil {
			t.Log().Infof("%s", err)
			lastErr = err
		}
		done, err := t.isThinDevFreeDone(ctx, sid, devId)
		if err != nil {
			return err
		}
		if done {
			return nil
		}
		if i >= freeMaxTries {
			return fmt.Errorf("dev %s is still not free of all allocations after %d tries: last free error: %v", devId, i, lastErr)
		}
		if err := sleep(ctx, retryDelay); err != nil {
			return err
		}
	}
}

func (t *Array) isThinDevFreeDone(ctx context.Context, sid, devId string) (bool, error) {
	if v, err := t.IsThinDevFreed(ctx, sid, devId); err != nil || !v {
		return false, err
	}
	if v, err := t.IsThinDevStatusDeallocating(ctx, sid, devId); err != nil || v {
		return false, err
	}
	if v, err := t.IsThinDevStatusFreeingAll(ctx, sid, devId); err != nil || v {
		return false, err
	}
	return true, nil
}

// sleep waits d, or less when the context is done.
func sleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (t *Array) IsThinDevStatusDeallocating(ctx context.Context, sid, devId string) (bool, error) {
	return t.SymCfgVerifyThinDevStatus(ctx, sid, devId, "-deallocating")
}

func (t *Array) IsThinDevStatusFreeingAll(ctx context.Context, sid, devId string) (bool, error) {
	return t.SymCfgVerifyThinDevStatus(ctx, sid, devId, "-freeingall")
}

// SymCfgVerifyThinDevStatus is true when the device is in the status.
//
// The output says it, not the exit code, as v2 read it: symcfg verify
// answers "None" when no device is in the status.
func (t *Array) SymCfgVerifyThinDevStatus(ctx context.Context, sid, devId, status string) (bool, error) {
	if sid == "" {
		return false, errNoSID
	}
	result, err := t.symResult(ctx, zerolog.TraceLevel, "symcfg", "-sid", sid, "verify", "-tdevs", "-devs", devId, status)
	if err != nil {
		return false, err
	}
	l := strings.Fields(result.Out)
	if len(l) == 0 {
		return false, fmt.Errorf("unexpected verify output: %s", strings.TrimSpace(result.Out+result.Err))
	}
	if l[0] == "None" {
		t.Log().Infof("device %s is not %s", devId, status)
		return false, nil
	}
	t.Log().Infof("device %s is %s", devId, status)
	return true, nil
}

func (t *Array) setDevRO(ctx context.Context, sid, devId string) error {
	_, err := t.symOn(ctx, "symdev", sid, "write_disable", devId, "-noprompt")
	return err
}

func (t *Array) SetSRDFMode(ctx context.Context, opt OptSetSRDFMode) error {
	if opt.SID == "" {
		opt.SID = t.kwSID()
	}
	if opt.SRDFMode == "" {
		return fmt.Errorf("--srdf-mode is required")
	}
	dev, err := t.getDev(ctx, opt.SID, opt.Dev)
	if err != nil {
		return err
	}
	rdf := dev.paired()
	if rdf == nil {
		return fmt.Errorf("dev %s is not in a RDF relation", dev.DevInfo.DevName)
	}
	rdfg, dst := rdf.RAGroupNum(), rdf.RemoteDev()
	if rdfg == "" || dst == "" {
		return fmt.Errorf("dev %s: no rdf group or remote device in its srdf pairing", dev.DevInfo.DevName)
	}
	pairFile, err := t.writePairFile(dev.DevInfo.DevName, dst)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(pairFile) }()
	_, err = t.symOn(ctx, "symrdf", opt.SID, "-f", pairFile, "-rdfg", rdfg, "set", "mode", opt.SRDFMode, "-noprompt")
	return err
}

// createdDevs is the devices an action created, named in its error when it
// fails after creating them.
//
// A device created is never deleted on a failure: deleting is what loses
// data when the device turns out to be in use after all, so the operator is
// told what exists and decides.
type createdDevs struct {
	sid  string
	r1   string
	rsid string
	r2   string
}

func (t createdDevs) wrap(err error) error {
	if err == nil || t.r1 == "" {
		return err
	}
	s := fmt.Sprintf("dev %s on array %s", t.r1, t.sid)
	if t.r2 != "" {
		s = fmt.Sprintf("R1 dev %s on array %s and R2 dev %s on array %s", t.r1, t.sid, t.r2, t.rsid)
	}
	return fmt.Errorf("%w: created %s, left in place for the operator to clean up", err, s)
}

// thinDevPlan is a validated request to create a device, and its SRDF
// mirror when asked for.
type thinDevPlan struct {
	OptAddThinDev
	cylinders int64
	rsid      string
}

// planThinDev checks a request to create a device, and resolves the remote
// array of its mirror, before anything is created: a request that can not
// complete creates nothing.
func (t *Array) planThinDev(ctx context.Context, opt OptAddThinDev) (thinDevPlan, error) {
	plan := thinDevPlan{OptAddThinDev: opt}
	if opt.Name == "" {
		return plan, fmt.Errorf("--name is required")
	}
	if opt.Size == "" {
		return plan, fmt.Errorf("--size is required")
	}
	size, err := array.ParseSize(opt.Size)
	if err != nil {
		return plan, err
	}
	if size.Relative {
		return plan, fmt.Errorf("size %s: the size of a new device can not be relative", opt.Size)
	}
	plan.cylinders = cylinders(size.Bytes)
	if opt.SID == "" {
		plan.SID = t.kwSID()
	}
	if plan.SID == "" {
		return plan, errNoSID
	}
	if !opt.SRDF {
		return plan, nil
	}
	if opt.RDFG == "" {
		return plan, fmt.Errorf("--srdf is specified but --rdfg is not")
	}
	if opt.SRDFMode == "" {
		return plan, fmt.Errorf("--srdf is specified but --srdf-mode is not")
	}
	if opt.SRDFType == "" {
		return plan, fmt.Errorf("--srdf is specified but --srdf-type is not")
	}
	groups, err := t.SymCfgRDFGList(ctx, plan.SID, opt.RDFG)
	if err != nil {
		return plan, err
	}
	if len(groups) == 0 || groups[0].RemoteSymId == "" {
		return plan, fmt.Errorf("can't find remote sid of rdfg %s", opt.RDFG)
	}
	plan.rsid = groups[0].RemoteSymId
	return plan, nil
}

// addThinDev creates the device of a plan and, for a SRDF plan, its mirror
// on the remote array and the pair of the two.
func (t *Array) addThinDev(ctx context.Context, plan thinDevPlan) (createdDevs, error) {
	created := createdDevs{sid: plan.SID, rsid: plan.rsid}
	r1, err := t.createThinDev(ctx, plan.SID, plan.Name, plan.cylinders, plan.SG)
	if err != nil {
		return created, err
	}
	created.r1 = r1
	if err := t.checkCreatedSize(ctx, plan.SID, r1, plan.cylinders); err != nil {
		return created, created.wrap(err)
	}
	if !plan.SRDF {
		return created, nil
	}
	r2, err := t.createThinDev(ctx, plan.rsid, plan.Name, plan.cylinders, "")
	if err != nil {
		return created, created.wrap(err)
	}
	created.r2 = r2
	if err := t.checkCreatedSize(ctx, plan.rsid, r2, plan.cylinders); err != nil {
		return created, created.wrap(err)
	}
	err = t.CreatePair(ctx, OptCreatePair{
		Pair:     r1 + ":" + r2,
		RDFG:     plan.RDFG,
		SRDFMode: plan.SRDFMode,
		SRDFType: plan.SRDFType,
		SID:      plan.SID,
	})
	if err != nil {
		return created, created.wrap(err)
	}
	return created, nil
}

// checkCreatedSize returns an error when a device created with a number of
// cylinders is not that size.
//
// The sizes are sent in cylinders of 1920 KB, the cylinder of the arrays
// since the VMAX3, and the cylinder of an older array is half that: the
// device is then half the size asked for, which a success would hide from
// the collector recording the size asked for.
func (t *Array) checkCreatedSize(ctx context.Context, sid, devName string, cyl int64) error {
	dev, err := t.getDev(ctx, sid, devName)
	if err != nil {
		return fmt.Errorf("read the size of the new dev %s: %w", devName, err)
	}
	got, err := dev.currentCylinders()
	if err != nil {
		return err
	}
	if got != cyl {
		return fmt.Errorf("dev %s: %d cylinders asked for and %d created", devName, cyl, got)
	}
	return nil
}

// AddThinDev creates a device, unmapped unless a storage group is named,
// and its SRDF mirror when asked for.
func (t *Array) AddThinDev(ctx context.Context, opt OptAddThinDev) (Device, error) {
	plan, err := t.planThinDev(ctx, opt)
	if err != nil {
		return Device{}, err
	}
	if plan.SG != "" {
		if _, err := t.getSG(ctx, plan.SID, plan.SG); err != nil {
			return Device{}, err
		}
	}
	created, err := t.addThinDev(ctx, plan)
	if err != nil {
		return Device{}, err
	}
	dev, err := t.getDev(ctx, plan.SID, created.r1)
	return dev, created.wrap(err)
}

func (t *Array) getDevsFromCreateThinDevOutput(b []byte) ([]string, error) {
	reader := bytes.NewReader(b)
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		i := strings.Index(line, "devices created are")
		if i < 0 {
			continue
		}
		line = line[i:]
		begin := strings.Index(line, "[")
		end := strings.Index(line, "]")
		if begin < 0 || end < begin {
			return nil, fmt.Errorf("unexpected device list in 'symdev create -tdev' output: %s", line)
		}
		return strings.Fields(line[begin+1 : end]), nil
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("device not found in 'symdev create -tdev' output: %s", string(b))
}

// createThinDev creates one device of the size in cylinders, in the
// storage group when one is named, and returns its name.
func (t *Array) createThinDev(ctx context.Context, sid, name string, cyl int64, sg string) (string, error) {
	result, err := t.symOn(ctx, "symdev", sid, t.createThinDevArgs(name, cyl, sg)...)
	if err != nil {
		return "", err
	}
	devs, err := t.getDevsFromCreateThinDevOutput([]byte(result.Out))
	switch {
	case err != nil:
		return "", fmt.Errorf("%w: the create succeeded, so a device named %s may exist on array %s", err, name, sid)
	case len(devs) != 1:
		return "", fmt.Errorf("one device asked for and %d created on array %s: %s", len(devs), sid, strings.Join(devs, " "))
	}
	return devs[0], nil
}

func (t *Array) createThinDevArgs(name string, cyl int64, sg string) []string {
	args := []string{"create", "-tdev", "-N", "1", "-cap", fmt.Sprint(cyl), "-captype", "cyl"}
	if sg != "" {
		args = append(args, "-sg", sg)
	}
	return append(args, "-emulation", "FBA", "-device_name", name, "-noprompt", "-v")
}

// CreateThinDev creates one device and returns its name.
func (t *Array) CreateThinDev(ctx context.Context, opt OptAddThinDev) (string, error) {
	opt.SRDF = false
	plan, err := t.planThinDev(ctx, opt)
	if err != nil {
		return "", err
	}
	return t.createThinDev(ctx, plan.SID, plan.Name, plan.cylinders, plan.SG)
}

func (t *Array) DelThinDev(ctx context.Context, opt OptDelThinDev) (Device, error) {
	if opt.SID == "" {
		opt.SID = t.kwSID()
	}
	dev, err := t.getDev(ctx, opt.SID, opt.Dev)
	if err != nil {
		return Device{}, err
	}
	err = t.delThinDev(ctx, opt.SID, dev.DevInfo.DevName)
	if err != nil {
		return Device{}, err
	}
	return dev, nil
}

func (t *Array) delThinDev(ctx context.Context, sid, devId string) error {
	result, err := t.symOn(ctx, "symdev", sid, "delete", devId, "-noprompt")
	if err != nil {
		if strings.Contains(result.Err+result.Out, "A free of all allocations is required") {
			return fmt.Errorf("%w: %w", ErrNotFree, err)
		}
		return err
	}
	return nil
}

func (t *Array) writePairFile(src, dst string) (string, error) {
	f, err := os.CreateTemp("", "om.array.symmetrix.rdf.pair.*")
	if err != nil {
		return "", err
	}
	path := f.Name()
	if _, err := fmt.Fprintf(f, "%s %s\n", src, dst); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return "", err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	return path, nil
}

// CreatePair pairs a device of this array with a device of the array of
// the rdf group, as v2 paired them.
func (t *Array) CreatePair(ctx context.Context, opt OptCreatePair) error {
	if opt.Pair == "" {
		return fmt.Errorf("--pair is required")
	}
	if opt.RDFG == "" {
		return fmt.Errorf("--rdfg is required")
	}
	if opt.SRDFType == "" {
		return fmt.Errorf("--srdf-type is required")
	}
	if opt.SRDFMode == "" {
		return fmt.Errorf("--srdf-mode is required")
	}
	src, dst, ok := strings.Cut(opt.Pair, ":")
	if !ok || src == "" || dst == "" || strings.Contains(dst, ":") {
		return fmt.Errorf("misformatted pair %s: expect <dev>:<remote dev>", opt.Pair)
	}
	switch opt.Invalidate {
	case "", "R1", "R2":
	default:
		return fmt.Errorf("--invalidate %s: expect R1 or R2", opt.Invalidate)
	}
	if opt.SID == "" {
		opt.SID = t.kwSID()
	}
	dev, err := t.getDev(ctx, opt.SID, src)
	if err != nil {
		return err
	}
	if dev.paired() != nil {
		return fmt.Errorf("dev %s is already in a RDF relation", src)
	}
	pairFile, err := t.writePairFile(dev.DevInfo.DevName, dst)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(pairFile) }()
	args := []string{"-f", pairFile, "-rdfg", opt.RDFG, "createpair", "-noprompt", "-rdf_mode", opt.SRDFMode, "-type", opt.SRDFType}
	if opt.Invalidate != "" {
		args = append(args, "-invalidate", opt.Invalidate)
	} else {
		args = append(args, "-establish")
	}
	_, err = t.symOn(ctx, "symrdf", opt.SID, args...)
	return err
}

// deletePair deletes the SRDF pair of the device, suspending it first, and
// returns the pairing deleted, or nil when the device is not paired.
func (t *Array) deletePair(ctx context.Context, sid string, dev Device) (*RDF, error) {
	rdf := dev.paired()
	if rdf == nil {
		t.Log().Debugf("dev %s is not in a RDF relation", dev.DevInfo.DevName)
		return nil, nil
	}
	rdfg, dst := rdf.RAGroupNum(), rdf.RemoteDev()
	if rdfg == "" || dst == "" {
		return rdf, fmt.Errorf("dev %s: no rdf group or remote device in its srdf pairing", dev.DevInfo.DevName)
	}
	pairFile, err := t.writePairFile(dev.DevInfo.DevName, dst)
	if err != nil {
		return rdf, err
	}
	defer func() { _ = os.Remove(pairFile) }()
	suspended := false
	if rdf.PairState() != "Suspended" {
		if _, err := t.symOn(ctx, "symrdf", sid, "-f", pairFile, "-rdfg", rdfg, "suspend", "-noprompt"); err != nil {
			return rdf, err
		}
		suspended = true
	}
	if _, err := t.symOn(ctx, "symrdf", sid, "-f", pairFile, "-rdfg", rdfg, "deletepair", "-noprompt", "-force"); err != nil {
		if suspended {
			return rdf, fmt.Errorf("%w: the pair of dev %s with dev %s of array %s was suspended, and is left suspended", err, dev.DevInfo.DevName, dst, rdf.RemoteSID())
		}
		return rdf, err
	}
	return rdf, nil
}

func (t *Array) DeletePair(ctx context.Context, opt OptDeletePair) (*RDF, error) {
	if opt.SID == "" {
		opt.SID = t.kwSID()
	}
	dev, err := t.getDev(ctx, opt.SID, opt.Dev)
	if err != nil {
		return nil, err
	}
	return t.deletePair(ctx, opt.SID, dev)
}

type (
	// DiskResult is what an action on a disk returns, in the shape v2
	// returned it, which the collector stores as the result of its form.
	DiskResult struct {
		DiskID    string `json:"disk_id"`
		DiskDevID string `json:"disk_devid"`

		// DevID is the disk_devid of v2, under the name the first om3
		// releases gave it.
		DevID      string                 `json:"dev_id"`
		Mappings   map[string]DiskMapping `json:"mappings"`
		DriverData map[string]any         `json:"driver_data"`
	}

	// DiskMapping is a path a disk is presented on, indexed by
	// "<hba_id>:<tgt_id>" in the mappings of a DiskResult.
	DiskMapping struct {
		SG       string `json:"sg"`
		ViewName string `json:"view_name"`
		HBAID    string `json:"hba_id"`
		TGTID    string `json:"tgt_id"`

		// LUN is the host lun as symaccess prints it.
		LUN string `json:"lun"`
	}
)

// diskResult returns the result of an action adding or mapping a disk.
func (t *Array) diskResult(ctx context.Context, sid, devName string) (DiskResult, error) {
	var result DiskResult
	data, wwn, err := t.devWWN(ctx, sid, devName)
	if err != nil {
		return result, err
	}
	mappings, err := t.getMappings(ctx, sid, devName)
	if err != nil {
		return result, err
	}
	result.DiskID = wwn
	result.DiskDevID = devName
	result.DevID = devName
	result.Mappings = mappings
	result.DriverData = map[string]any{"dev": data}
	return result, nil
}

// MapDisk presents a device in the storage group named, or in the one
// reaching every mapping.
func (t *Array) MapDisk(ctx context.Context, opt OptMapDisk) (DiskResult, error) {
	var result DiskResult
	if opt.SID == "" {
		opt.SID = t.kwSID()
	}
	if len(opt.Mappings) == 0 && opt.SG == "" {
		return result, fmt.Errorf("--sg or --mappings is required")
	}
	dev, err := t.getDev(ctx, opt.SID, opt.Dev)
	if err != nil {
		return result, err
	}
	devName := dev.DevInfo.DevName
	sg, err := t.resolveSG(ctx, opt.SID, opt.SG, opt.Mappings, opt.SLO, opt.SRP)
	if err != nil {
		return result, err
	}
	if err := t.addThinDevToSG(ctx, opt.SID, devName, sg); err != nil {
		return result, err
	}
	return t.diskResult(ctx, opt.SID, devName)
}

// getMappings returns the paths the device is presented on, through the
// views of its storage groups, those presenting the group itself and those
// presenting it as the child of another.
func (t *Array) getMappings(ctx context.Context, sid, devId string) (map[string]DiskMapping, error) {
	sgs, err := t.getDevSGs(ctx, sid, devId)
	if err != nil {
		return nil, err
	}
	m := make(map[string]DiskMapping)
	cache := make(viewCache)
	for _, sg := range sgs {
		for _, viewName := range sg.GroupInfo.viewNames() {
			view, err := t.getCachedView(ctx, cache, sid, viewName)
			if err != nil {
				return nil, err
			}
			for _, portInfo := range view.ViewInfo.PortInfo.DirectorIdentifications {
				for _, initiator := range view.ViewInfo.InitiatorList.Initiators {
					if initiator.WWN == nil {
						continue
					}
					for _, device := range view.ViewInfo.Devices {
						if device.DevName != devId {
							continue
						}
						for _, devPortInfo := range device.DevPortInfo {
							if devPortInfo.Port != portInfo.Port {
								continue
							}
							m[*initiator.WWN+":"+portInfo.PortWWN] = DiskMapping{
								SG:       sg.GroupInfo.GroupName,
								ViewName: view.ViewInfo.Name,
								HBAID:    *initiator.WWN,
								TGTID:    portInfo.PortWWN,
								LUN:      devPortInfo.HostLUN,
							}
						}
					}
				}
			}
		}
	}
	return m, nil
}

// AddDisk creates a device, its SRDF mirror when asked for, and presents
// it, v2's way: the storage group is found before anything is created, so a
// request no storage group can serve creates nothing.
func (t *Array) AddDisk(ctx context.Context, opt OptAddDisk) (DiskResult, error) {
	var result DiskResult
	plan, err := t.planThinDev(ctx, OptAddThinDev{
		Name:     opt.Name,
		RDFG:     opt.RDFG,
		Size:     opt.Size,
		SRDF:     opt.SRDF,
		SRDFMode: opt.SRDFMode,
		SRDFType: opt.SRDFType,
		SID:      opt.SID,
	})
	if err != nil {
		return result, err
	}
	sg, err := t.resolveSG(ctx, plan.SID, opt.SG, opt.Mappings, opt.SLO, opt.SRP)
	if err != nil {
		return result, err
	}
	created, err := t.addThinDev(ctx, plan)
	if err != nil {
		return result, err
	}
	if sg != "" {
		if err := t.addThinDevToSG(ctx, plan.SID, created.r1, sg); err != nil {
			return result, created.wrap(err)
		}
	}
	result, err = t.diskResult(ctx, plan.SID, created.r1)
	if err != nil {
		return result, created.wrap(err)
	}
	return result, nil
}

func (t *Array) unmap(ctx context.Context, sid, devId string) error {
	sgs, err := t.getDevSGs(ctx, sid, devId)
	if err != nil {
		return err
	}
	for _, sg := range sgs {
		if err := t.removeThinDevFromSG(ctx, sid, devId, sg.GroupInfo.GroupName); err != nil {
			return err
		}
	}
	return nil
}

func (t *Array) UnmapDisk(ctx context.Context, opt OptUnmapDisk) (DiskResult, error) {
	var result DiskResult
	if opt.SID == "" {
		opt.SID = t.kwSID()
	}
	dev, err := t.getDev(ctx, opt.SID, opt.Dev)
	if err != nil {
		return result, err
	}
	if err := t.unmap(ctx, opt.SID, dev.DevInfo.DevName); err != nil {
		return result, err
	}
	result.DiskID = devWWNOf(dev)
	result.DiskDevID = dev.DevInfo.DevName
	result.DevID = dev.DevInfo.DevName
	result.DriverData = map[string]any{"dev": dev}
	return result, nil
}

// devWWNOf is the wwn of a device as symdev show prints it.
func devWWNOf(dev Device) string {
	if dev.Product == nil {
		return ""
	}
	return dev.Product.WWN
}

// DelDisk unpresents and deletes a device, v2's way: the device is made
// read-only, unmapped, its SRDF pair deleted, its allocations freed when
// the symcli needs it, and deleted.
//
// The result holds the pairing the device had, read before the pair was
// deleted, which the collector reads to delete the remote device too.
func (t *Array) DelDisk(ctx context.Context, opt OptDelDisk) (DiskResult, error) {
	var result DiskResult
	if opt.SID == "" {
		opt.SID = t.kwSID()
	}
	dev, err := t.getDev(ctx, opt.SID, opt.Dev)
	if err != nil {
		return result, err
	}
	devName := dev.DevInfo.DevName
	if dev.DevInfo.SnapvxSource == "True" {
		return result, fmt.Errorf("dev %s is a snapvx_source. can not delete", devName)
	}
	rdf := dev.paired()
	if rdf != nil && strings.EqualFold(rdf.LocalType(), "R2") {
		// The pairing of an R2 names its R1 as the remote device, which the
		// collector deletes next when the result reports it: deleting an R2
		// would delete the R1 the hosts use. The R1 is deleted instead,
		// which deletes the pair, and reports this R2 to delete next.
		return result, fmt.Errorf("dev %s is the R2 of a SRDF pair with dev %s of array %s: delete the R1, whose deletion deletes the pair and reports this R2 to delete next", devName, rdf.RemoteDev(), rdf.RemoteSID())
	}

	// done is what was done to the device, said in the error of a later
	// step, so the operator knows the state it is left in.
	var done []string
	failed := func(err error) error {
		if len(done) == 0 {
			return err
		}
		return fmt.Errorf("%w: dev %s was %s, and is not deleted", err, devName, strings.Join(done, ", "))
	}

	if err := t.setDevRO(ctx, opt.SID, devName); err != nil {
		// As v2: the write disable keeps hosts from writing to a device
		// being deleted, and fails on a device already write disabled, as
		// the R2 the collector deletes after its R1. The unmap and the
		// delete decide.
		t.Log().Warnf("dev %s: write disable: %s", devName, err)
	} else {
		done = append(done, "write disabled")
	}
	if err := t.unmap(ctx, opt.SID, devName); err != nil {
		return result, failed(err)
	}
	done = append(done, "unmapped")
	if _, err := t.deletePair(ctx, opt.SID, dev); err != nil {
		return result, failed(err)
	}
	if rdf != nil {
		done = append(done, fmt.Sprintf("unpaired from dev %s of array %s", rdf.RemoteDev(), rdf.RemoteSID()))
	}
	free := !t.symcliNewerThan(ctx, 9, 1)
	for i := 1; ; i++ {
		if free {
			if err := t.freeThinDev(ctx, opt.SID, devName); err != nil {
				return result, failed(err)
			}
			if i == 1 {
				done = append(done, "freed of all its allocations, its data gone")
			}
		}
		err := t.delThinDev(ctx, opt.SID, devName)
		if err == nil {
			break
		}
		if !errors.Is(err, ErrNotFree) {
			return result, failed(err)
		}
		if i >= deleteMaxTries {
			return result, failed(fmt.Errorf("dev %s is still not free of all allocations after %d tries", devName, i))
		}
		if err := sleep(ctx, retryDelay); err != nil {
			return result, failed(err)
		}
	}

	driverData := map[string]any{"dev": dev}
	if rdf != nil {
		driverData["rdf"] = rdf
	}
	result.DiskID = devWWNOf(dev)
	result.DiskDevID = devName
	result.DevID = devName
	result.DriverData = driverData
	return result, nil
}

// Dump/Restore of masking views
type (
	Result struct {
		Cmd []string `json:"cmd"`
		Ret int      `json:"ret"`
		Out string   `json:"out"`
		Err string   `json:"err"`
	}

	// MaskingDump is a masking plan, in v2's format, and once run the
	// outcome of each of its steps in the result of the step.
	MaskingDump struct {
		InitiatorGroups []MaskingDumpIG   `json:"ig,omitempty"`
		StorageGroups   []MaskingDumpSG   `json:"sg,omitempty"`
		Gatekeepers     []MaskingDumpGK   `json:"gk,omitempty"`
		Devices         []MaskingDumpDev  `json:"dev,omitempty"`
		Views           []MaskingDumpView `json:"mv,omitempty"`
	}
	MaskingDumpIG struct {
		Name            string   `json:"name"`
		HBAIds          []string `json:"hba_ids,omitempty"`
		InitiatorGroups []string `json:"ig,omitempty"`
		Consistent      *bool    `json:"consistent,omitempty"`
		Results         []Result `json:"result"`
	}
	MaskingDumpSG struct {
		Name          string   `json:"name"`
		SRP           string   `json:"srp,omitempty"`
		SLO           string   `json:"slo,omitempty"`
		StorageGroups []string `json:"sg,omitempty"`
		Results       []Result `json:"result"`
	}
	MaskingDumpGK struct {
		StorageGroup string   `json:"sg"`
		Count        *int     `json:"count,omitempty"`
		Results      []Result `json:"result"`
	}
	MaskingDumpDev struct {
		Name         string   `json:"name,omitempty"`
		Size         string   `json:"size"`
		StorageGroup string   `json:"sg"`
		Results      []Result `json:"result"`
	}
	MaskingDumpView struct {
		Name                string   `json:"name"`
		PortIds             []string `json:"pg"`
		StorageGroupNames   []string `json:"sg,omitempty"`
		InitiatorGroupNames []string `json:"ig,omitempty"`
		Results             []Result `json:"result"`
	}
)

// AddMasking runs a masking plan, v2's way: every step is run whatever the
// outcome of the others, its outcome recorded in its result, and the plan
// with its results is returned. Only a plan that can not be read is an
// error, and then nothing is run.
func (t *Array) AddMasking(ctx context.Context, b []byte) (MaskingDump, error) {
	data, err := parseMaskingDump(b)
	if err != nil {
		return data, err
	}
	sid := t.kwSID()
	if sid == "" {
		return data, errNoSID
	}
	return t.addMasking(ctx, sid, data), nil
}

// parseMaskingDump reads a masking plan.
//
// The plan is the --data of the collector, which carries keys of its own
// next to the steps, and these are ignored. The steps are read strictly: a
// key a step does not know is a misspelled one, and a plan run without it
// is not the plan asked for.
func parseMaskingDump(b []byte) (MaskingDump, error) {
	var data MaskingDump
	if len(bytes.TrimSpace(b)) == 0 {
		return data, fmt.Errorf("--data is required")
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(b, &top); err != nil {
		return data, fmt.Errorf("--data: %w", err)
	}
	for k, v := range map[string]any{
		"ig":  &data.InitiatorGroups,
		"sg":  &data.StorageGroups,
		"gk":  &data.Gatekeepers,
		"dev": &data.Devices,
		"mv":  &data.Views,
	} {
		raw, ok := top[k]
		if !ok {
			continue
		}
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		if err := dec.Decode(v); err != nil {
			return data, fmt.Errorf("--data: %s: %w", k, err)
		}
	}
	return data, data.validate()
}

func (t MaskingDump) validate() error {
	for i, e := range t.InitiatorGroups {
		if e.Name == "" {
			return fmt.Errorf("--data: ig[%d]: no name", i)
		}
	}
	for i, e := range t.StorageGroups {
		if e.Name == "" {
			return fmt.Errorf("--data: sg[%d]: no name", i)
		}
	}
	for i, e := range t.Gatekeepers {
		if e.StorageGroup == "" {
			return fmt.Errorf("--data: gk[%d]: no sg", i)
		}
		if e.Count != nil && *e.Count < 0 {
			return fmt.Errorf("--data: gk[%d]: negative count", i)
		}
	}
	for i, e := range t.Devices {
		if e.StorageGroup == "" {
			return fmt.Errorf("--data: dev[%d]: no sg", i)
		}
		size, err := array.ParseSize(e.Size)
		if err != nil {
			return fmt.Errorf("--data: dev[%d]: %w", i, err)
		}
		if size.Relative {
			return fmt.Errorf("--data: dev[%d]: size %s: the size of a new device can not be relative", i, e.Size)
		}
	}
	for i, e := range t.Views {
		if e.Name == "" {
			return fmt.Errorf("--data: mv[%d]: no name", i)
		}
		if len(e.PortIds) == 0 {
			return fmt.Errorf("--data: mv[%d]: no pg", i)
		}
	}
	return nil
}

func (t *Array) addMasking(ctx context.Context, sid string, data MaskingDump) MaskingDump {
	for i, e := range data.InitiatorGroups {
		data.InitiatorGroups[i].Results = t.addDumpInitiatorGroup(ctx, sid, e)
	}
	for i, e := range data.StorageGroups {
		data.StorageGroups[i].Results = t.addDumpStorageGroup(ctx, sid, e)
	}
	for i, e := range data.Gatekeepers {
		data.Gatekeepers[i].Results = t.addDumpGatekeeper(ctx, sid, e)
	}
	for i, e := range data.Devices {
		data.Devices[i].Results = t.addDumpDevice(ctx, sid, e)
	}
	pgs := make(portGroupCache)
	for i, e := range data.Views {
		data.Views[i].Results = t.addDumpView(ctx, sid, pgs, e)
	}
	return data
}

// step runs one symcli command of a masking plan and returns its outcome,
// which a command that could not run at all is too.
func (t *Array) step(ctx context.Context, bin string, args ...string) Result {
	result, _ := t.symResult(ctx, zerolog.InfoLevel, bin, args...)
	return result
}

// failedStep is the outcome of a step that ran no command, for an error
// found before running it.
func failedStep(cmd []string, err error) Result {
	if cmd == nil {
		cmd = []string{}
	}
	return Result{Cmd: cmd, Ret: 1, Err: err.Error()}
}

func (t *Array) addDumpInitiatorGroup(ctx context.Context, sid string, data MaskingDumpIG) []Result {
	consistent := true
	if data.Consistent != nil {
		consistent = *data.Consistent
	}
	args := []string{"-sid", sid, "-name", data.Name, "-type", "initiator"}
	if consistent {
		args = append(args, "-consistent_lun")
	}
	args = append(args, "create")
	results := []Result{t.step(ctx, "symaccess", args...)}
	for _, ig := range data.InitiatorGroups {
		results = append(results, t.step(ctx, "symaccess", "-sid", sid, "-name", data.Name, "-type", "initiator", "-ig", ig, "add"))
	}
	for _, hbaId := range data.HBAIds {
		results = append(results, t.step(ctx, "symaccess", "-sid", sid, "-name", data.Name, "-type", "initiator", "-wwn", hbaId, "add"))
	}
	return results
}

func (t *Array) addDumpStorageGroup(ctx context.Context, sid string, data MaskingDumpSG) []Result {
	args := []string{"-sid", sid, "create", data.Name}
	if data.SRP != "" {
		args = append(args, "-srp", data.SRP)
	}
	if data.SLO != "" {
		args = append(args, "-slo", data.SLO)
	}
	results := []Result{t.step(ctx, "symsg", args...)}
	for _, sg := range data.StorageGroups {
		result := t.step(ctx, "symsg", "-sid", sid, "-sg", data.Name, "add", "sg", sg)
		if result.Ret != 0 && strings.Contains(result.Err, "group is currently within device masking view") {
			// The child is already in the parent.
			result.Ret = 0
			result.Out = result.Err
			result.Err = ""
		}
		results = append(results, result)
	}
	return results
}

func (t *Array) addDumpGatekeeper(ctx context.Context, sid string, data MaskingDumpGK) []Result {
	count := defaultGKCount
	if data.Count != nil {
		count = *data.Count
	}
	sg, err := t.getSG(ctx, sid, data.StorageGroup)
	if err != nil {
		return []Result{failedStep(nil, err)}
	}
	missing := count - sg.NumOfGKs
	if missing <= 0 {
		return []Result{}
	}
	return []Result{t.step(ctx, "symdev", "-sid", sid, "create", "-gk", "-N", fmt.Sprint(missing), "-sg", data.StorageGroup, "-noprompt")}
}

// addDumpDevice creates the device of a storage group, unless the group
// already holds devices, which makes running a plan twice harmless.
func (t *Array) addDumpDevice(ctx context.Context, sid string, data MaskingDumpDev) []Result {
	b, err := t.symSGShow(ctx, sid, data.StorageGroup)
	if err != nil {
		// A device created without its storage group would be presented
		// to nobody, so none is.
		return []Result{failedStep(nil, err)}
	}
	n, err := countElements(b, "Device")
	if err != nil {
		return []Result{failedStep(nil, fmt.Errorf("storage group %s: %w", data.StorageGroup, err))}
	}
	if n > 0 {
		return []Result{}
	}
	size, err := array.ParseSize(data.Size)
	if err != nil {
		return []Result{failedStep(nil, err)}
	}
	name := data.Name
	if name == "" {
		name = "NONAME"
	}
	args := append([]string{"-sid", sid}, t.createThinDevArgs(name, cylinders(size.Bytes), data.StorageGroup)...)
	return []Result{t.step(ctx, "symdev", args...)}
}

// portGroupCache keeps the port groups of the array and their ports, read
// once for all the views of a plan.
type portGroupCache map[string][]string

func (t *Array) portGroupPorts(ctx context.Context, sid string, cache portGroupCache) (portGroupCache, error) {
	if len(cache) > 0 {
		return cache, nil
	}
	// The port groups and their ports are found at any depth of the
	// outputs, as v2 found them, there being no sample of these outputs
	// to pin their nesting on.
	b, err := t.symXML(ctx, "symaccess", sid, "list", "-type", "port")
	if err != nil {
		return cache, err
	}
	root, err := parseXMLNode(b)
	if err != nil {
		return cache, err
	}
	for _, pg := range root.findAll("Port_Group") {
		name := ""
		for _, info := range pg.findAll("Group_Info") {
			if name = info.childText("group_name"); name != "" {
				break
			}
		}
		if name == "" {
			continue
		}
		b, err := t.symXML(ctx, "symaccess", sid, "show", name, "-type", "port")
		if err != nil {
			return cache, err
		}
		show, err := parseXMLNode(b)
		if err != nil {
			return cache, err
		}
		ports := make([]string, 0)
		for _, id := range show.findAll("Director_Identification") {
			if wwn := id.childText("port_wwn"); wwn != "" {
				ports = append(ports, wwn)
			}
		}
		cache[name] = ports
	}
	return cache, nil
}

// findPG returns the port group holding exactly the ports, as v2 found it,
// the first by name when several do.
func (t *Array) findPG(ctx context.Context, sid string, cache portGroupCache, tgtIds []string) (string, error) {
	cache, err := t.portGroupPorts(ctx, sid, cache)
	if err != nil {
		return "", err
	}
	names := maps.Keys(cache)
	sort.Strings(names)
	for _, name := range names {
		if samePorts(cache[name], tgtIds) {
			return name, nil
		}
	}
	return "", nil
}

// samePorts is true when the two lists hold the same ports, whatever their
// order and case.
func samePorts(a, b []string) bool {
	set := func(l []string) map[string]any {
		m := make(map[string]any)
		for _, s := range l {
			m[strings.ToLower(s)] = nil
		}
		return m
	}
	ma, mb := set(a), set(b)
	if len(ma) != len(mb) {
		return false
	}
	for k := range ma {
		if _, ok := mb[k]; !ok {
			return false
		}
	}
	return true
}

func (t *Array) addDumpView(ctx context.Context, sid string, pgs portGroupCache, data MaskingDumpView) []Result {
	pg, err := t.findPG(ctx, sid, pgs, data.PortIds)
	if err != nil {
		return []Result{failedStep(nil, err)}
	}
	if pg == "" {
		return []Result{failedStep(nil, fmt.Errorf("can't create the '%s' masking view: no pg with port ids %v", data.Name, data.PortIds))}
	}
	args := []string{"-sid", sid, "create", "view", "-name", data.Name, "-pg", pg}
	if len(data.StorageGroupNames) > 0 {
		args = append(args, "-sg", strings.Join(data.StorageGroupNames, ","))
	}
	if len(data.InitiatorGroupNames) > 0 {
		args = append(args, "-ig", strings.Join(data.InitiatorGroupNames, ","))
	}
	return []Result{t.step(ctx, "symaccess", args...)}
}

func (t ShowPortGroup) HasPort(tgtId string) bool {
	for _, directorId := range t.GroupInfo.DirectorIdentifications {
		if strings.EqualFold(directorId.PortWWN, tgtId) {
			return true
		}
	}
	return false
}

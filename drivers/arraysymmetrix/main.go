package arraysymmetrix

import (
	"bufio"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"golang.org/x/exp/maps"

	"github.com/opensvc/om3/core/array"
	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/resourceid"
	"github.com/opensvc/om3/util/command"
	"github.com/opensvc/om3/util/nullable"
	"github.com/opensvc/om3/util/plog"
	"github.com/opensvc/om3/util/sizeconv"
)

type (
	Array struct {
		*array.Array
		log *plog.Logger
	}

	resizeMethod int

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
		PortCount     int           `xml:"port_count" json:"port_count"`
		ViewCount     int           `xml:"view_count" json:"view_count"`
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
		ConsistentLUN bool          `xml:"consistent_lun" json:"consistent_lun"`
		DevCount      int           `xml:"dev_count" json:"dev_count"`
		SGCount       int           `xml:"sg_count" json:"sg_count"`
		ViewCount     int           `xml:"view_count" json:"view_count"`
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
		XMLName           xml.Name      `xml:"Group_Info" json:"-"`
		GroupName         string        `xml:"group_name" json:"group_name"`
		DevCount          int           `xml:"dev_count" json:"dev_count"`
		SGCount           int           `xml:"sg_count" json:"sg_count"`
		ViewCount         int           `xml:"view_count" json:"view_count"`
		LastUpdated       string        `xml:"last_updated" json:"last_updated"`
		MaskViewNames     MaskViewNames `xml:"Mask_View_Names" json:"Mask_View_Names"`
		CascadedViewNames MaskViewNames `xml:"Cascaded_View_Names" json:"Cascaded_View_Names"`
		Status            string        `xml:"status" json:"status"`
	}
	MaskViewNames struct {
		XMLName   xml.Name `xml:"Mask_View_Names" json:"-"`
		ViewCount int      `xml:"view_count" json:"view_count"`
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
		ThinDevs []ThinDev     `xml:"ThinDevs" json:"ThinDevs"`
	}
	ThinDev struct {
		XMLName              xml.Name `xml:"Device" json:"-"`
		DevName              string   `xml:"dev_name" json:"dev_name"`
		DevEmul              string   `xml:"dev_emul" json:"dev_emul"`
		MultiPool            string   `xml:"multi_pool" json:"multi_pool"`
		SharedTracks         int64    `xml:"shared_tracks" json:"shared_tracks"`
		PersistTracks        int64    `xml:"persist_tracks" json:"persist_tracks"`
		TotalTracks          int64    `xml:"total_tracks" json:"total_tracks"`
		AllocTracks          int64    `xml:"alloc_tracks" json:"alloc_tracks"`
		UnreducibleTracks    int64    `xml:"unreducible_tracks" json:"unreducible_tracks"`
		WrittenTracks        int64    `xml:"written_tracks" json:"written_tracks"`
		CompressedTracks     int64    `xml:"compressed_tracks" json:"compressed_tracks"`
		ExclusiveAllocTracks int64    `xml:"exclusive_alloc_tracks" json:"exclusive_alloc_tracks"`
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
		Status    string   `xml:"Status" json:Status"`
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

	//
	RDF struct {
		XMLName  xml.Name  `xml:"RDF" json:"-"`
		Info     RDFInfo   `xml:"RDF_Info" json:"RDF_Info"`
		Mode     RDFMode   `xml:"Mode" json:"Mode"`
		Link     RDFLink   `xml:"Link" json:"Link"`
		Local    RDFLocal  `xml:"Local" json:"Local"`
		Remote   RDFRemote `xml:"Remote" json:"Remote"`
		RDFAInfo RDFAInfo  `xml:"RdfaInfo" json:"RdfaInfo"`
	}
	RDFAInfo struct {
		XMLName xml.Name `xml:"RdfaInfo" json:"-"`
	}
	RDFRemote struct {
		XMLName     xml.Name `xml:"Remote" json:"-"`
		DevName     string   `xml:"dev_name" json:"dev_name"`
		RemoteSymid string   `xml:"remote_symid" json:"remote_symid"`
		WWN         string   `xml:"wwn" json:"wwn"`
		State       string   `xml:"state" json:"state"`
	}
	RDFLocal struct {
		XMLName    xml.Name `xml:"Local" json:"-"`
		DevName    string   `xml:"dev_name" json:"dev_name"`
		Type       string   `xml:"type" json:"type"`
		RAGroupNum int      `xml:"ra_group_num" json:"ra_group_num"`
		State      string   `xml:"state" json:"state"`
	}
	RDFStatus struct {
		XMLName              xml.Name `xml:"Status" json:"-"`
		RDF                  string   `xml:"rdf" json:"rdf"`
		RA                   string   `xml:"ra" json:"ra"`
		SA                   string   `xml:"sa" json:"sa"`
		Link                 string   `xml:"link" json:"link"`
		LinkStatusChangeTime string   `xml:"link_status_change_time" json:"link_status_change_time"`
	}
	RDFLink struct {
		XMLName                  xml.Name `xml:"Link" json:"-"`
		Configuration            string   `xml:"configuration" json:"configuration"`
		Domino                   string   `xml:"domino" json:"domino"`
		PreventAutomaticRecovery string   `xml:"prevent_automatic_recovery" json:"prevent_automatic_recovery"`
	}
	RDFMode struct {
		XMLName                    xml.Name `xml:"Mode" json:"-"`
		Mode                       string   `xml:"mode" json:"mode"`
		AdaptativeCopy             string   `xml:"adaptative_copy" json:"adaptative_copy"`
		AdaptativeCopyWritePending string   `xml:"adaptative_copy_write_pending" json:"adaptative_copy_write_pending"`
		AdaptativeCopySkew         int      `xml:"adaptative_copy_skew" json:"adaptative_copy_skew"`
		DeviceDomino               string   `xml:"device_domino" json:"device_domino"`
		StarMode                   bool     `xml:"star_mode" json:"star_mode"`
		SqarMode                   bool     `xml:"sqar_mode" json:"sqar_mode"`
	}
	RDFInfo struct {
		XMLName                       xml.Name  `xml:"RDF_Info" json:"-"`
		PairState                     string    `xml:"pair_state" json:"pair_state"`
		SuspendState                  string    `xml:"suspend_state" json:"suspend_state"`
		ConsistencyState              string    `xml:"consistency_state" json:"consistency_state"`
		ConsistencyExemptState        string    `xml:"consistency_exempt_state" json:"consistency_exempt_state"`
		ConfigRDFAWPaceExemptState    string    `xml:"config_rdfa_wpace_exempt_state" json:"config_rdfa_wpace_exempt_state"`
		EffectiveRDFAWPaceExemptState string    `xml:"effective_rdfa_wpace_exempt_state" json:"effective_rdfa_wpace_exempt_state"`
		WPaceInfo                     WPaceInfo `xml:"WPace_Info" json:"WPace_Info"`
		R1Invalids                    int       `xml:"r1_invalids" json:"r1_invalids"`
		R2Invalids                    int       `xml:"r2_invalids" json:"r2_invalids"`
		R2LargerThanR1                bool      `xml:"r2_larger_than_r1" json:"r2_larger_than_r1"`
		R1R2DeviceSize                string    `xml:"r1_r2_device_size" json:"r1_r2_device_size"`
		PairedWithDiskless            bool      `xml:"paired_with_diskless" json:"paired_with_diskless"`
		PairedWithConcurrent          bool      `xml:"paired_with_concurrent" json:"paired_with_concurrent"`
		PairedWithCascaded            bool      `xml:"paired_with_cascaded" json:"paired_with_cascaded"`
		ThickThinRelationship         bool      `xml:"thick_thin_relationship" json:"thick_thin_relationship"`
		R2NotReadIfInvalid            string    `xml:"r2_not_ready_if_invalid" json:"r2_not_ready_if_invalid"`
		PairConfiguration             string    `xml:"pair_configuration" json:"pair_configuration"`
	}
	WPaceInfo struct {
		XMLName                       xml.Name `xml:"WPace_Info" json:"-"`
		PacingCapable                 string   `xml:"pacing_capable" json:"pacing_capable"`
		ConfigRDFAWPaceExemptState    string   `xml:"config_rdfa_wpace_exempt_state" json:"config_rdfa_wpace_exempt_state"`
		EffectiveRDFAWPaceExemptState string   `xml:"effective_rdfa_wpace_exempt_state" json:"effective_rdfa_wpace_exempt_state"`
		RDFAWPaceState                string   `xml:"rdfa_wpace_state" json:"rdfa_wpace_state"`
		RDFADevPaceState              string   `xml:"rdfa_devpace_state" json:"rdfa_devpace_state"`
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
		SRPName       string   `xml:"srp_name" json:"srp_name"`
		Configuration string   `xml:"configuration" json:"configuration"`
		Status        string   `xml:"status" json:"status"`
		SnapvxSource  bool     `xml:"snapvx_source" json:"snapvx_source"`
		SnapvxTarget  bool     `xml:"snapvx_target" json:"snapvx_target"`
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
		XMLName xml.Name `xml:"Director" json:"-"`
		DirInfo DirInfo  `xml:"Dir_Info" json:"Dir_Info"`
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

	mappingSGs      map[string]mappingSGsValue
	mappingSGsValue struct {
		initiatorCount int
		items          []mappingSG
	}
	mappingSG struct {
		hbaId    string
		tgtId    string
		sgName   string
		viewName string
	}
)

const (
	// Resize methods
	ResizeExact resizeMethod = iota
	ResizeUp
	ResizeDown
)

var (
	// PromptReader is bufio.NewReader(os.Stdin) for testing dangerous command only, normally nil
	PromptReader *bufio.Reader
	//PromptReader = bufio.NewReader(os.Stdin)

	ErrNotFree = errors.New("device is not free")
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

func (t *Array) Run(args []string) error {
	parent := newParent()
	parent.AddCommand(newCreatePairCmd(t))
	parent.AddCommand(newDeletePairCmd(t))

	// skip past the --array <array> arguments
	parent.SetArgs(array.SkipArgs())

	setCmd := newSetCmd()
	setCmd.AddCommand(newSetSRDFModeCmd(t))
	parent.AddCommand(setCmd)

	addCmd := newAddCmd()
	addCmd.AddCommand(newAddDiskCmd(t))
	addCmd.AddCommand(newAddThinDevCmd(t))
	parent.AddCommand(addCmd)

	renameCmd := newRenameCmd()
	renameCmd.AddCommand(newRenameDiskCmd(t))
	parent.AddCommand(renameCmd)

	resizeCmd := newResizeCmd()
	resizeCmd.AddCommand(newResizeDiskCmd(t))
	parent.AddCommand(resizeCmd)

	delCmd := newDelCmd()
	delCmd.AddCommand(newDelDiskCmd(t))
	delCmd.AddCommand(newDelThinDevCmd(t))
	parent.AddCommand(delCmd)

	getCmd := newGetCmd()
	getCmd.AddCommand(newGetDirectorsCmd(t))
	getCmd.AddCommand(newGetPoolsCmd(t))
	getCmd.AddCommand(newGetSRPsCmd(t))
	getCmd.AddCommand(newGetStorageGroupsCmd(t))
	getCmd.AddCommand(newGetThinDevsCmd(t))
	getCmd.AddCommand(newGetViewsCmd(t))
	parent.AddCommand(getCmd)

	mapCmd := newMapCmd()
	mapCmd.AddCommand(newMapDiskCmd(t))
	parent.AddCommand(mapCmd)

	unmapCmd := newUnmapCmd()
	unmapCmd.AddCommand(newUnmapDiskCmd(t))
	parent.AddCommand(unmapCmd)

	return parent.Execute()
}

func (t *Array) symaccess() string {
	return filepath.Join(t.kwSymcliPath(), "symaccess")
}
func (t *Array) symcfg() string {
	return filepath.Join(t.kwSymcliPath(), "symcfg")
}
func (t *Array) symconfigure() string {
	return filepath.Join(t.kwSymcliPath(), "symconfigure")
}
func (t *Array) symdev() string {
	return filepath.Join(t.kwSymcliPath(), "symdev")
}
func (t *Array) symdisk() string {
	return filepath.Join(t.kwSymcliPath(), "symdisk")
}
func (t *Array) symsg() string {
	return filepath.Join(t.kwSymcliPath(), "symsg")
}
func (t *Array) symrdf() string {
	return filepath.Join(t.kwSymcliPath(), "symrdf")
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

func (t *Array) SymEnv() []string {
	var l []string
	if s := t.kwSymcliConnect(); s != "" {
		l = append(l, "SYMCLI_CONNECT="+s)
	}
	return l
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

func (t *Array) SymAccessShowViewDetail(name string) ([]MaskingView, error) {
	sid := t.kwSID()
	args := []string{"-sid", sid, "-ouput", "xml_e", "show", "view", name, "-detail"}
	cmd := command.New(
		command.WithPrompt(PromptReader),
		command.WithName(t.symaccess()),
		command.WithArgs(args),
		command.WithBufferedStdout(),
		command.WithBufferedStderr(),
		command.WithCommandLogLevel(zerolog.DebugLevel),
		command.WithLogLevel(zerolog.DebugLevel),
		command.WithEnv(t.SymEnv()),
		command.WithLogger(t.Log()),
	)
	err := cmd.Run()
	if err != nil {
		return nil, err
	}
	b := cmd.Stdout()
	return t.parseSymAccessListViewDetail(b)
}

func (t *Array) SymAccessListViewDetail() ([]MaskingView, error) {
	sid := t.kwSID()
	args := []string{"-sid", sid, "-ouput", "xml_e", "list", "view", "-detail"}
	cmd := command.New(
		command.WithPrompt(PromptReader),
		command.WithName(t.symaccess()),
		command.WithArgs(args),
		command.WithBufferedStdout(),
		command.WithBufferedStderr(),
		command.WithCommandLogLevel(zerolog.DebugLevel),
		command.WithLogLevel(zerolog.DebugLevel),
		command.WithEnv(t.SymEnv()),
		command.WithLogger(t.Log()),
	)
	err := cmd.Run()
	if err != nil {
		return nil, err
	}
	b := cmd.Stdout()
	return t.parseSymAccessListViewDetail(b)
}

func (t *Array) parseSymAccessListViewDetail(b []byte) ([]MaskingView, error) {
	var head XSymAccessListViewDetail
	if err := xml.Unmarshal(b, &head); err != nil {
		return nil, err
	}
	return head.Symmetrix.MaskingViews, nil
}

func (t *Array) SymCfgList(s string) (SymmInfo, error) {
	sid := t.kwSID()
	args := []string{"-sid", sid, "-ouput", "xml_e", "list"}
	cmd := command.New(
		command.WithPrompt(PromptReader),
		command.WithName(t.symcfg()),
		command.WithArgs(args),
		command.WithBufferedStdout(),
		command.WithBufferedStderr(),
		command.WithCommandLogLevel(zerolog.DebugLevel),
		command.WithLogLevel(zerolog.DebugLevel),
		command.WithEnv(t.SymEnv()),
		command.WithLogger(t.Log()),
	)
	err := cmd.Run()
	if err != nil {
		return SymmInfo{}, err
	}
	b := cmd.Stdout()
	return t.parseSymCfgList(b)
}

func (t *Array) parseSymCfgList(b []byte) (SymmInfo, error) {
	var head XSymCfgList
	if err := xml.Unmarshal(b, &head); err != nil {
		return SymmInfo{}, err
	}
	return head.Symmetrix.SymmInfo, nil
}

func (t *Array) SymCfgDirectorList(s string) ([]Director, error) {
	sid := t.kwSID()
	args := []string{"-sid", sid, "-ouput", "xml_e", "-dir", s, "-v", "list"}
	cmd := command.New(
		command.WithPrompt(PromptReader),
		command.WithName(t.symcfg()),
		command.WithArgs(args),
		command.WithBufferedStdout(),
		command.WithBufferedStderr(),
		command.WithCommandLogLevel(zerolog.DebugLevel),
		command.WithLogLevel(zerolog.DebugLevel),
		command.WithEnv(t.SymEnv()),
		command.WithLogger(t.Log()),
	)
	err := cmd.Run()
	if err != nil {
		return nil, err
	}
	b := cmd.Stdout()
	return t.parseSymCfgDirectorList(b)
}

func (t *Array) parseSymCfgDirectorList(b []byte) ([]Director, error) {
	var head XSymCfgDirList
	if err := xml.Unmarshal(b, &head); err != nil {
		return nil, err
	}
	return head.Symmetrix.Directors, nil
}

func (t *Array) SymCfgRDFGList(s string) ([]RDFGroup, error) {
	sid := t.kwSID()
	args := []string{"-sid", sid, "-ouput", "xml_e", "-rdfg", s, "list"}
	cmd := command.New(
		command.WithPrompt(PromptReader),
		command.WithName(t.symcfg()),
		command.WithArgs(args),
		command.WithBufferedStdout(),
		command.WithBufferedStderr(),
		command.WithCommandLogLevel(zerolog.DebugLevel),
		command.WithLogLevel(zerolog.DebugLevel),
		command.WithEnv(t.SymEnv()),
		command.WithLogger(t.Log()),
	)
	err := cmd.Run()
	if err != nil {
		return nil, err
	}
	b := cmd.Stdout()
	return t.parseSymCfgRDFGList(b)
}

func (t *Array) parseSymCfgRDFGList(b []byte) ([]RDFGroup, error) {
	var head XSymCfgRDFGList
	if err := xml.Unmarshal(b, &head); err != nil {
		return nil, err
	}
	return head.Symmetrix.RDFGroups, nil
}

func (t *Array) SymCfgPoolList() ([]DevicePool, error) {
	sid := t.kwSID()
	args := []string{"-sid", sid, "-ouput", "xml_e", "-pool", "list", "-v"}
	cmd := command.New(
		command.WithPrompt(PromptReader),
		command.WithName(t.symcfg()),
		command.WithArgs(args),
		command.WithBufferedStdout(),
		command.WithBufferedStderr(),
		command.WithCommandLogLevel(zerolog.DebugLevel),
		command.WithLogLevel(zerolog.DebugLevel),
		command.WithEnv(t.SymEnv()),
		command.WithLogger(t.Log()),
	)
	err := cmd.Run()
	if err != nil {
		return nil, err
	}
	b := cmd.Stdout()
	return t.parseSymCfgPoolList(b)
}

func (t *Array) parseSymCfgPoolList(b []byte) ([]DevicePool, error) {
	var head XSymCfgPoolList
	if err := xml.Unmarshal(b, &head); err != nil {
		return nil, err
	}
	return head.Symmetrix.DevicePools, nil
}

func (t *Array) SymCfgSLOList() ([]SLO, error) {
	sid := t.kwSID()
	args := []string{"-sid", sid, "-ouput", "xml_e", "list", "-slo", "-detail", "-v"}
	cmd := command.New(
		command.WithPrompt(PromptReader),
		command.WithName(t.symcfg()),
		command.WithArgs(args),
		command.WithBufferedStdout(),
		command.WithBufferedStderr(),
		command.WithCommandLogLevel(zerolog.DebugLevel),
		command.WithLogLevel(zerolog.DebugLevel),
		command.WithEnv(t.SymEnv()),
		command.WithLogger(t.Log()),
	)
	err := cmd.Run()
	if err != nil {
		return nil, err
	}
	b := cmd.Stdout()
	return t.parseSymCfgSLOList(b)
}

func (t *Array) parseSymCfgSLOList(b []byte) ([]SLO, error) {
	var head XSymCfgSLOList
	if err := xml.Unmarshal(b, &head); err != nil {
		return nil, err
	}
	return head.Symmetrix.SLOs, nil
}

func (t *Array) SymCfgSRPList() ([]SRP, error) {
	sid := t.kwSID()
	args := []string{"-sid", sid, "-ouput", "xml_e", "list", "-srp", "-detail", "-v"}
	cmd := command.New(
		command.WithPrompt(PromptReader),
		command.WithName(t.symcfg()),
		command.WithArgs(args),
		command.WithBufferedStdout(),
		command.WithBufferedStderr(),
		command.WithCommandLogLevel(zerolog.DebugLevel),
		command.WithLogLevel(zerolog.DebugLevel),
		command.WithEnv(t.SymEnv()),
		command.WithLogger(t.Log()),
	)
	err := cmd.Run()
	if err != nil {
		return nil, err
	}
	b := cmd.Stdout()
	return t.parseSymCfgSRPList(b)
}

func (t *Array) parseSymCfgSRPList(b []byte) ([]SRP, error) {
	var head XSymCfgSRPList
	if err := xml.Unmarshal(b, &head); err != nil {
		return nil, err
	}
	return head.Symmetrix.SRPs, nil
}

func (t *Array) SymDiskListDiskGroupSummary() ([]DiskGroup, error) {
	sid := t.kwSID()
	args := []string{"-sid", sid, "-ouput", "xml_e", "list", "-dskgroup_summary"}
	cmd := command.New(
		command.WithPrompt(PromptReader),
		command.WithName(t.symdisk()),
		command.WithArgs(args),
		command.WithBufferedStdout(),
		command.WithBufferedStderr(),
		command.WithCommandLogLevel(zerolog.DebugLevel),
		command.WithLogLevel(zerolog.DebugLevel),
		command.WithEnv(t.SymEnv()),
		command.WithLogger(t.Log()),
	)
	err := cmd.Run()
	if err != nil {
		return nil, err
	}
	b := cmd.Stdout()
	return t.parseSymDiskListDiskGroupSummary(b)
}

func (t *Array) parseSymDiskListDiskGroupSummary(b []byte) ([]DiskGroup, error) {
	var head XSymDiskListDiskGroupSummary
	if err := xml.Unmarshal(b, &head); err != nil {
		return nil, err
	}
	return head.Symmetrix.DiskGroups, nil
}

func (t *Array) SymDevListThinDevs(sid, devId string) ([]ThinDev, error) {
	args := []string{"-sid", sid, "-output", "xml_e", "list", "-tdevs", "-devs", devId}
	cmd := command.New(
		command.WithPrompt(PromptReader),
		command.WithName(t.symdev()),
		command.WithArgs(args),
		command.WithBufferedStdout(),
		command.WithBufferedStderr(),
		command.WithCommandLogLevel(zerolog.DebugLevel),
		command.WithLogLevel(zerolog.DebugLevel),
		command.WithEnv(t.SymEnv()),
		command.WithLogger(t.Log()),
	)
	err := cmd.Run()
	if err != nil {
		return nil, err
	}
	b := cmd.Stdout()
	return t.parseSymDevListThinDevs(b)
}

func (t *Array) parseSymDevListThinDevs(b []byte) ([]ThinDev, error) {
	var head XSymDevListThinDevs
	if err := xml.Unmarshal(b, &head); err != nil {
		return nil, err
	}
	return head.Symmetrix.ThinDevs, nil
}

func (t *Array) SymDevShow(sid, devId string) ([]Device, error) {
	args := []string{"-sid", sid, "-output", "xml_e", "show", devId}
	cmd := command.New(
		command.WithPrompt(PromptReader),
		command.WithName(t.symdev()),
		command.WithArgs(args),
		command.WithBufferedStdout(),
		command.WithBufferedStderr(),
		command.WithCommandLogLevel(zerolog.DebugLevel),
		command.WithLogLevel(zerolog.DebugLevel),
		command.WithEnv(t.SymEnv()),
		command.WithLogger(t.Log()),
	)
	err := cmd.Run()
	if err != nil {
		return nil, err
	}
	b := cmd.Stdout()
	return t.parseSymDevShow(b)
}

func (t *Array) parseSymDevShow(b []byte) ([]Device, error) {
	var head XSymDevShow
	if err := xml.Unmarshal(b, &head); err != nil {
		return nil, err
	}
	return head.Symmetrix.Devices, nil
}

func (t *Array) SymDevShowByWWN(sid, devId string) ([]Device, error) {
	args := []string{"-sid", sid, "-output", "xml_e", "show", "-wwn", devId}
	cmd := command.New(
		command.WithPrompt(PromptReader),
		command.WithName(t.symdev()),
		command.WithArgs(args),
		command.WithBufferedStdout(),
		command.WithBufferedStderr(),
		command.WithCommandLogLevel(zerolog.DebugLevel),
		command.WithLogLevel(zerolog.DebugLevel),
		command.WithEnv(t.SymEnv()),
		command.WithLogger(t.Log()),
	)
	err := cmd.Run()
	if err != nil {
		return nil, err
	}
	b := cmd.Stdout()
	var head XSymDevShow
	if err := xml.Unmarshal(b, &head); err != nil {
		return nil, err
	}
	return head.Symmetrix.Devices, nil
}

func (t *Array) SymDevList(sid string) ([]Device, error) {
	if sid == "" {
		sid = t.kwSID()
	}
	args := []string{"-sid", sid, "-output", "xml_e", "list"}
	cmd := command.New(
		command.WithPrompt(PromptReader),
		command.WithName(t.symdev()),
		command.WithArgs(args),
		command.WithBufferedStdout(),
		command.WithBufferedStderr(),
		command.WithCommandLogLevel(zerolog.DebugLevel),
		command.WithLogLevel(zerolog.DebugLevel),
		command.WithEnv(t.SymEnv()),
		command.WithLogger(t.Log()),
	)
	err := cmd.Run()
	if err != nil {
		return nil, err
	}
	b := cmd.Stdout()
	return t.parseSymDevList(b)
}

func (t *Array) parseSymDevList(b []byte) ([]Device, error) {
	var head XSymDevList
	if err := xml.Unmarshal(b, &head); err != nil {
		return nil, err
	}
	return head.Symmetrix.Devices, nil
}

func (t *Array) getPGByTgtIds(sid string, tgtIds []string) (PortGroup, error) {
	l, err := t.SymAccessListPort(sid)
	if err != nil {
		return PortGroup{}, err
	}
	for _, pg := range l {
		pgShow, err := t.SymAccessShowPort(sid, pg.GroupInfo.GroupName)
		if err != nil {
			return PortGroup{}, err
		}
		if pgShow.HasAllPortOf(tgtIds) {
			return pg, nil
		}
	}
	return PortGroup{}, os.ErrNotExist
}

func (t *Array) getSG(sid, name string) (SGInfo, error) {
	l, err := t.SymSGShow(sid, name)
	if err != nil {
		return SGInfo{}, err
	}
	if len(l) == 0 {
		return SGInfo{}, os.ErrNotExist
	}
	return l[0], nil
}

func (t *Array) SymSGShow(sid, name string) ([]SGInfo, error) {
	if sid == "" {
		sid = t.kwSID()
	}
	args := []string{"-sid", sid, "-output", "xml_e", "show", name}
	cmd := command.New(
		command.WithPrompt(PromptReader),
		command.WithName(t.symsg()),
		command.WithArgs(args),
		command.WithBufferedStdout(),
		command.WithBufferedStderr(),
		command.WithCommandLogLevel(zerolog.DebugLevel),
		command.WithLogLevel(zerolog.DebugLevel),
		command.WithEnv(t.SymEnv()),
		command.WithLogger(t.Log()),
	)
	err := cmd.Run()
	if err != nil {
		return nil, err
	}
	b := cmd.Stdout()
	return t.parseSymSGList(b)
}

func (t *Array) SymSGList(sid string) ([]SGInfo, error) {
	if sid == "" {
		sid = t.kwSID()
	}
	args := []string{"-sid", sid, "-output", "xml_e", "list", "-v"}
	cmd := command.New(
		command.WithPrompt(PromptReader),
		command.WithName(t.symsg()),
		command.WithArgs(args),
		command.WithBufferedStdout(),
		command.WithBufferedStderr(),
		command.WithCommandLogLevel(zerolog.DebugLevel),
		command.WithLogLevel(zerolog.DebugLevel),
		command.WithEnv(t.SymEnv()),
		command.WithLogger(t.Log()),
	)
	err := cmd.Run()
	if err != nil {
		return nil, err
	}
	b := cmd.Stdout()
	return t.parseSymSGList(b)
}

func (t *Array) parseSymSGList(b []byte) ([]SGInfo, error) {
	var head XSymSGList
	if err := xml.Unmarshal(b, &head); err != nil {
		return nil, err
	}
	return head.SG.SGInfos, nil
}

func (t *Array) getDev(sid, devId string) (Device, error) {
	var (
		devs []Device
		err  error
	)
	if devId == "" {
		return Device{}, fmt.Errorf("--dev is required")
	}
	if len(devId) > 6 {
		devs, err = t.SymDevShowByWWN(sid, devId)
	} else {
		devs, err = t.SymDevShow(sid, devId)
	}
	if err != nil {
		return Device{}, err
	}
	if len(devs) > 0 {
		return devs[0], nil
	}
	return Device{}, os.ErrNotExist
}

func (t *Array) addThinDevToSG(sid, devId, sg string) error {
	args := []string{"-sid", sid, "-name", sg, "-type", "storage", "add", "dev", devId}
	cmd := command.New(
		command.WithPrompt(PromptReader),
		command.WithName(t.symaccess()),
		command.WithArgs(args),
		command.WithBufferedStdout(),
		command.WithBufferedStderr(),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithLogLevel(zerolog.InfoLevel),
		command.WithEnv(t.SymEnv()),
		command.WithLogger(t.Log()),
	)
	return cmd.Run()
}

func (t *Array) removeThinDevFromSG(sid, devId, sg string) error {
	args := []string{"-sid", sid, "-name", sg, "-type", "storage", "remove", "dev", devId, "-unmap"}
	cmd := command.New(
		command.WithPrompt(PromptReader),
		command.WithName(t.symaccess()),
		command.WithArgs(args),
		command.WithBufferedStdout(),
		command.WithBufferedStderr(),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithLogLevel(zerolog.InfoLevel),
		command.WithEnv(t.SymEnv()),
		command.WithLogger(t.Log()),
	)
	return cmd.Run()
}

func (t *Array) SymAccessShowPort(sid, name string) (ShowPortGroup, error) {
	args := []string{"-sid", sid, "show", name, "-type", "port"}
	cmd := command.New(
		command.WithPrompt(PromptReader),
		command.WithName(t.symaccess()),
		command.WithArgs(args),
		command.WithBufferedStdout(),
		command.WithBufferedStderr(),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithLogLevel(zerolog.InfoLevel),
		command.WithEnv(t.SymEnv()),
		command.WithLogger(t.Log()),
	)
	err := cmd.Run()
	if err != nil {
		return ShowPortGroup{}, err
	}
	b := cmd.Stdout()
	return t.parseSymAccessShowPort(b)
}

func (t *Array) parseSymAccessShowPort(b []byte) (ShowPortGroup, error) {
	var head XSymAccessShowPort
	if err := xml.Unmarshal(b, &head); err != nil {
		return ShowPortGroup{}, err
	}
	return head.Symmetrix.PortGroup, nil
}

func (t *Array) SymAccessListPort(sid string) ([]PortGroup, error) {
	args := []string{"-sid", sid, "list", "-type", "port"}
	cmd := command.New(
		command.WithPrompt(PromptReader),
		command.WithName(t.symaccess()),
		command.WithArgs(args),
		command.WithBufferedStdout(),
		command.WithBufferedStderr(),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithLogLevel(zerolog.InfoLevel),
		command.WithEnv(t.SymEnv()),
		command.WithLogger(t.Log()),
	)
	err := cmd.Run()
	if err != nil {
		return nil, err
	}
	b := cmd.Stdout()
	return t.parseSymAccessListPort(b)
}

func (t *Array) parseSymAccessListPort(b []byte) ([]PortGroup, error) {
	var head XSymAccessListPort
	if err := xml.Unmarshal(b, &head); err != nil {
		return nil, err
	}
	return head.Symmetrix.PortGroups, nil
}

func (t *Array) SymAccessListDevInitiator(sid, wwn string) ([]InitiatorGroup, error) {
	args := []string{"-sid", sid, "list", "-type", "initiator", "-wwn", wwn}
	cmd := command.New(
		command.WithPrompt(PromptReader),
		command.WithName(t.symaccess()),
		command.WithArgs(args),
		command.WithBufferedStdout(),
		command.WithBufferedStderr(),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithLogLevel(zerolog.InfoLevel),
		command.WithEnv(t.SymEnv()),
		command.WithLogger(t.Log()),
	)
	err := cmd.Run()
	if err != nil {
		return nil, err
	}
	b := cmd.Stdout()
	return t.parseSymAccessListDevInitiator(b)
}

func (t *Array) parseSymAccessListDevInitiator(b []byte) ([]InitiatorGroup, error) {
	var head XSymAccessListDevInitiator
	if err := xml.Unmarshal(b, &head); err != nil {
		return nil, err
	}
	return head.Symmetrix.InitiatorGroups, nil
}

func (t *Array) getInitiatorViewNames(sid, wwn string) ([]string, error) {
	sgs, err := t.SymAccessListDevInitiator(sid, wwn)
	if err != nil {
		return nil, err
	}
	m := make(map[string]any)
	for _, sg := range sgs {
		for _, name := range sg.GroupInfo.MaskViewNames.ViewNames {
			name = strings.TrimSuffix(name, " *")
			m[name] = nil
		}
	}
	return maps.Keys(m), nil
}

func (t *Array) getView(name string) (MaskingView, error) {
	views, err := t.SymAccessShowViewDetail(name)
	if err != nil {
		return MaskingView{}, err
	}
	if len(views) == 0 {
		return MaskingView{}, fmt.Errorf("masking view '%s' does not exist", name)
	}
	return views[0], nil
}

func (t mappingSGs) Intersect(other mappingSGs) mappingSGs {
	m := make(mappingSGs)
	for k, v := range t {
		if _, ok := other[k]; ok {
			m[k] = v
		}
	}
	return m
}

func (t mappingSGs) narrowestSG() string {
	if len(t) == 0 {
		return ""
	}
	minimum := -1
	narrowest := ""
	for sgName, e := range t {
		if (minimum < 0) || (e.initiatorCount < minimum) {
			minimum = e.initiatorCount
			narrowest = sgName
		}
	}
	return narrowest
}

func (t *Array) filterMappingsSGs(current mappingSGs, sid string, slo, srp string) (mappingSGs, error) {
	m := make(mappingSGs)
	for sgName, e := range current {
		sg, err := t.getSG(sid, sgName)
		if err != nil {
			return m, err
		}
		if (srp != "") && (sg.SRPName != srp) {
			t.log.Infof("discard sg %s (srp %s, required %s)", sgName, sg.SRPName, srp)
			continue
		}
		if (slo != "") && (sg.SLOName != slo) {
			t.log.Infof("discard sg %s (slo %s, required %s)", sgName, sg.SLOName, slo)
			continue
		}
		m[sgName] = e
	}
	return m, nil
}

func (t *Array) bestSG(sid string, mappings []string, slo, srp string) (string, error) {
	if mappings == nil || len(mappings) == 0 {
		return "", nil
	}
	m, err := t.getStorageGroupOfMappings(sid, mappings)
	if err != nil {
		return "", err
	}
	if len(m) == 0 {
		return "", fmt.Errorf("no storage group found for the requested mappings")
	}
	if slo != "" || srp != "" {
		m, err = t.filterMappingsSGs(m, sid, slo, srp)
		if err != nil {
			return "", err
		}
		if len(m) == 0 {
			return "", fmt.Errorf("no storage group found for the requested mappings")
		}
	}
	narrowest := m.narrowestSG()
	t.log.Infof("candidates sgs: %s, retain: %s", maps.Keys(m), narrowest)
	return narrowest, nil
}

func (t *Array) getStorageGroupOfMappings(sid string, mappings []string) (mappingSGs, error) {
	var m mappingSGs
	for _, s := range mappings {
		elements := strings.Split(s, ":")
		if len(elements) != 2 {
			return m, fmt.Errorf("invalid mapping: %s: must be <hba>:<tgt>[,<tgt>...]", s)
		}
		hbaId := elements[0]
		if len(elements[1]) == 0 {
			return m, fmt.Errorf("invalid mapping: %s: must be <hba>:<tgt>[,<tgt>...]", s)
		}
		tgtIds := strings.Split(elements[1], ",")
		if len(tgtIds) == 0 {
			return m, fmt.Errorf("invalid mapping: %s: must be <hba>:<tgt>[,<tgt>...]", s)
		}
		for _, tgtId := range tgtIds {
			this, err := t.getStorageGroupOfMapping(sid, hbaId, tgtId)
			if err != nil {
				return m, err
			}
			if m == nil {
				m = this
			} else {
				m = m.Intersect(this)
			}
		}
	}
	return m, nil
}

func (t *Array) getStorageGroupOfMapping(sid, hbaId, tgtId string) (mappingSGs, error) {
	m := make(mappingSGs)
	ports := make(map[string]any)
	viewNames, err := t.getInitiatorViewNames(sid, hbaId)
	if err != nil {
		return nil, err
	}
	for _, viewName := range viewNames {
		viewSGs := make(map[string]any)
		view, err := t.getView(viewName)
		if err != nil {
			return nil, err
		}
		if len(view.ViewInfo.PortInfo.DirectorIdentifications) == 0 {
			continue
		}
		for _, portInfo := range view.ViewInfo.PortInfo.DirectorIdentifications {
			ports[portInfo.PortWWN] = nil
		}
		if _, ok := ports[tgtId]; !ok {
			continue
		}
		initiatorCount := 0
		for _, initiator := range view.ViewInfo.InitiatorList.Initiators {
			if initiator.WWN != nil {
				initiatorCount += 1
			}

		}
		if view.ViewInfo.SGChildInfo.ChildCount > 0 {
			for _, sg := range view.ViewInfo.SGChildInfo.SG {
				viewSGs[sg.GroupName] = nil
			}
		} else {
			viewSGs[view.ViewInfo.StorGrpName] = nil
		}
		for sgName := range viewSGs {
			mapping := mappingSG{
				sgName:   sgName,
				viewName: viewName,
				hbaId:    hbaId,
				tgtId:    tgtId,
			}
			if e, ok := m[sgName]; !ok {
				m[sgName] = mappingSGsValue{
					initiatorCount: initiatorCount,
					items:          []mappingSG{mapping},
				}
			} else {
				e.items = append(e.items, mapping)
				m[sgName] = e
			}
		}
	}
	return m, nil
}

func (t *Array) SymAccessListDevStorage(sid, devId string) ([]StorageGroup, error) {
	args := []string{"-sid", sid, "list", "-type", "storage", "-devs", devId}
	cmd := command.New(
		command.WithPrompt(PromptReader),
		command.WithName(t.symaccess()),
		command.WithArgs(args),
		command.WithBufferedStdout(),
		command.WithBufferedStderr(),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithLogLevel(zerolog.InfoLevel),
		command.WithEnv(t.SymEnv()),
		command.WithLogger(t.Log()),
	)
	err := cmd.Run()
	if err != nil {
		return nil, err
	}
	b := cmd.Stdout()
	return t.parseSymAccessListDevStorage(b)
}

func (t *Array) parseSymAccessListDevStorage(b []byte) ([]StorageGroup, error) {
	var head XSymAccessListDevStorage
	if err := xml.Unmarshal(b, &head); err != nil {
		return nil, err
	}
	return head.Symmetrix.StorageGroups, nil
}

func (t *Array) getDevSGs(sid, devId string) ([]StorageGroup, error) {
	sgs, err := t.SymAccessListDevStorage(sid, devId)
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

func (t *Array) getDevViewNames(sid, devId string) ([]string, error) {
	sgs, err := t.getDevSGs(sid, devId)
	if err != nil {
		return nil, err
	}
	m := make(map[string]any)
	for _, sg := range sgs {
		for _, name := range sg.GroupInfo.MaskViewNames.ViewNames {
			m[name] = nil
		}
	}
	return maps.Keys(m), nil
}

func (t *Array) addStorageGroupsToStorageGroup(sid, parent string, children []string) (Result, error) {
	var result Result
	args := []string{"-sid", sid, "-sg", parent, "add", "sg", strings.Join(children, ",")}
	cmd := command.New(
		command.WithPrompt(PromptReader),
		command.WithName(t.symsg()),
		command.WithArgs(args),
		command.WithBufferedStdout(),
		command.WithBufferedStderr(),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithLogLevel(zerolog.InfoLevel),
		command.WithEnv(t.SymEnv()),
		command.WithLogger(t.Log()),
	)
	err := cmd.Run()
	result.Ret = cmd.ExitCode()
	result.Out = string(cmd.Stdout())
	result.Err = string(cmd.Stderr())
	return result, err
}

func (t *Array) createStorageGroup(sid, name, srp, slo string) (Result, error) {
	var result Result
	args := []string{"-sid", sid, "create", name}
	if srp != "" {
		args = append(args, "-srp", srp)
	}
	if slo != "" {
		args = append(args, "-slo", slo)
	}
	cmd := command.New(
		command.WithPrompt(PromptReader),
		command.WithName(t.symsg()),
		command.WithArgs(args),
		command.WithBufferedStdout(),
		command.WithBufferedStderr(),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithLogLevel(zerolog.InfoLevel),
		command.WithEnv(t.SymEnv()),
		command.WithLogger(t.Log()),
	)
	err := cmd.Run()
	result.Ret = cmd.ExitCode()
	result.Out = string(cmd.Stdout())
	result.Err = string(cmd.Stderr())
	return result, err
}

func (t *Array) createView(sid, name string, portIds, sgNames, igNames []string) (Result, error) {
	var result Result

	pg, err := t.getPGByTgtIds(sid, portIds)
	if err == nil {
	} else if errors.Is(err, os.ErrNotExist) {
		result.Err = fmt.Sprintf("can't create the '%s' masking view: no pg with port ids %v", name, portIds)
		return result, nil
	} else {
		return result, err
	}

	args := []string{"-sid", sid, "create", "view", name, "-pg", pg.GroupInfo.GroupName}
	if len(sgNames) > 0 {
		args = append(args, "-sg", strings.Join(sgNames, ","))
	}
	if len(igNames) > 0 {
		args = append(args, "-ig", strings.Join(igNames, ","))
	}
	cmd := command.New(
		command.WithPrompt(PromptReader),
		command.WithName(t.symaccess()),
		command.WithArgs(args),
		command.WithBufferedStdout(),
		command.WithBufferedStderr(),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithLogLevel(zerolog.InfoLevel),
		command.WithEnv(t.SymEnv()),
		command.WithLogger(t.Log()),
	)
	err = cmd.Run()
	result.Ret = cmd.ExitCode()
	result.Out = string(cmd.Stdout())
	result.Err = string(cmd.Stderr())
	return result, err
}

func (t *Array) createThinDev(sid, name string, size string, sgName string) (Result, error) {
	var result Result
	if name == "" {
		name = "NONAME"
	}
	sizeBytes, err := sizeconv.FromSize(size)
	if err != nil {
		return result, err
	}
	args := []string{"-sid", sid, "create", "-tdev", "-N", "1", "-cap", fmt.Sprint(sizeBytes / 1024 / 1024)}

	_, err = t.getSG(sid, sgName)
	if err == nil {
		args = append(args, "-sg", sgName)
	} else if errors.Is(err, os.ErrNotExist) {
		// pass
	} else {
		return result, err
	}

	args = append(args, "-emulation", "FBA", "-device_name", name, "-noprompt", "-v")
	cmd := command.New(
		command.WithPrompt(PromptReader),
		command.WithName(t.symdev()),
		command.WithArgs(args),
		command.WithBufferedStdout(),
		command.WithBufferedStderr(),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithLogLevel(zerolog.InfoLevel),
		command.WithEnv(t.SymEnv()),
		command.WithLogger(t.Log()),
	)
	err = cmd.Run()
	result.Ret = cmd.ExitCode()
	result.Out = string(cmd.Stdout())
	result.Err = string(cmd.Stderr())
	return result, err
}

func (t *Array) createGatekeepers(sid, sgName string, count *int) (Result, error) {
	var result Result
	if count == nil {
		v := DumpDefaultGKCount
		count = &v
	}
	sg, err := t.getSG(sid, sgName)
	result.Err = err.Error()
	if err != nil {
		return result, err
	}
	if sg.NumOfGKs > *count {
		return result, nil
	}
	args := []string{"-sid", sid, "create", "-gk", "-N", fmt.Sprint(*count - sg.NumOfGKs), "-sg", sgName}
	cmd := command.New(
		command.WithPrompt(PromptReader),
		command.WithName(t.symdev()),
		command.WithArgs(args),
		command.WithBufferedStdout(),
		command.WithBufferedStderr(),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithLogLevel(zerolog.InfoLevel),
		command.WithEnv(t.SymEnv()),
		command.WithLogger(t.Log()),
	)
	err = cmd.Run()
	result.Ret = cmd.ExitCode()
	result.Out = string(cmd.Stdout())
	result.Err = string(cmd.Stderr())
	return result, err
}

func (t *Array) createInitiator(sid, name string, consistent bool) (Result, error) {
	var result Result
	args := []string{"-sid", sid, "name", name, "-type", "initiator"}
	if consistent {
		args = append(args, "-consistent")
	}
	args = append(args, "create")
	cmd := command.New(
		command.WithPrompt(PromptReader),
		command.WithName(t.symaccess()),
		command.WithArgs(args),
		command.WithBufferedStdout(),
		command.WithBufferedStderr(),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithLogLevel(zerolog.InfoLevel),
		command.WithEnv(t.SymEnv()),
		command.WithLogger(t.Log()),
	)
	err := cmd.Run()
	result.Ret = cmd.ExitCode()
	result.Out = string(cmd.Stdout())
	result.Err = string(cmd.Stderr())
	return result, err
}

func (t *Array) addInitiatorToInitiatorGroup(sid, name string, ig string) (Result, error) {
	var result Result
	args := []string{"-sid", sid, "name", name, "-type", "initiator", "-ig", ig, "add"}
	cmd := command.New(
		command.WithPrompt(PromptReader),
		command.WithName(t.symaccess()),
		command.WithArgs(args),
		command.WithBufferedStdout(),
		command.WithBufferedStderr(),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithLogLevel(zerolog.InfoLevel),
		command.WithEnv(t.SymEnv()),
		command.WithLogger(t.Log()),
	)
	err := cmd.Run()
	result.Ret = cmd.ExitCode()
	result.Out = string(cmd.Stdout())
	result.Err = string(cmd.Stderr())
	return result, err
}

func (t *Array) addHBAToInitiator(sid, name string, hbaId string) (Result, error) {
	var result Result
	args := []string{"-sid", sid, "name", name, "-type", "initiator", "-wwn", hbaId, "add"}
	cmd := command.New(
		command.WithPrompt(PromptReader),
		command.WithName(t.symaccess()),
		command.WithArgs(args),
		command.WithBufferedStdout(),
		command.WithBufferedStderr(),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithLogLevel(zerolog.InfoLevel),
		command.WithEnv(t.SymEnv()),
		command.WithLogger(t.Log()),
	)
	err := cmd.Run()
	result.Ret = cmd.ExitCode()
	result.Out = string(cmd.Stdout())
	result.Err = string(cmd.Stderr())
	return result, err
}

func (t *Array) RenameDisk(opt OptRenameDisk) (Device, error) {
	if opt.SID == "" {
		opt.SID = t.kwSID()
	}
	dev, err := t.getDev(opt.SID, opt.Dev)
	if err != nil {
		return dev, err
	}
	if opt.Name == "" {
		return dev, fmt.Errorf("--name is required")
	}
	args := []string{"-sid", opt.SID, "set", "dev", opt.Dev, "-attribute", "device_name=" + opt.Name}
	cmd := command.New(
		command.WithPrompt(PromptReader),
		command.WithName(t.symdev()),
		command.WithArgs(args),
		command.WithBufferedStdout(),
		command.WithBufferedStderr(),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithLogLevel(zerolog.InfoLevel),
		command.WithEnv(t.SymEnv()),
		command.WithLogger(t.Log()),
	)
	err = cmd.Run()
	if err != nil {
		return dev, err
	}
	return t.getDev(opt.SID, opt.Dev)
}

func (t *Array) ResizeDisk(opt OptResizeDisk) (Device, error) {
	if opt.SID == "" {
		opt.SID = t.kwSID()
	}
	dev, err := t.getDev(opt.SID, opt.Dev)
	if err != nil {
		return dev, err
	}
	if opt.Size == "" {
		return dev, fmt.Errorf("--size is required")
	}
	method := ResizeExact
	if len(opt.Size) > 1 {
		switch opt.Size[0] {
		case '+':
			opt.Size = opt.Size[1:]
			method = ResizeUp
		case '-':
			opt.Size = opt.Size[1:]
			method = ResizeDown
		}
	}
	sizeBytes, err := sizeconv.FromSize(opt.Size)
	if err != nil {
		return dev, err
	}
	if method != ResizeExact {
		switch method {
		case ResizeUp:
			sizeBytes = dev.Capacity.Megabytes*1024*1024 + sizeBytes
		case ResizeDown:
			sizeBytes = dev.Capacity.Megabytes*1024*1024 - sizeBytes
		}
	}
	if dev.Capacity.Megabytes*1024*1024 > sizeBytes && !opt.Force {
		return dev, fmt.Errorf("the target size is smaller than the current size. refuse to process. use --force if you accept the data loss risk.")
	}

	args := []string{"-sid", opt.SID, "modify", opt.Dev, "-tdev", "-cap", fmt.Sprint(sizeBytes / 1024 / 1024), "-captype", "mb", "-noprompt"}
	if t.IsPowerMax() && dev.RDF != nil {
		args = append(args, "-rdfg", fmt.Sprint(dev.RDF.Local.RAGroupNum))
	}
	cmd := command.New(
		command.WithPrompt(PromptReader),
		command.WithName(t.symdev()),
		command.WithArgs(args),
		command.WithBufferedStdout(),
		command.WithBufferedStderr(),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithLogLevel(zerolog.InfoLevel),
		command.WithEnv(t.SymEnv()),
		command.WithLogger(t.Log()),
	)
	err = cmd.Run()
	if err != nil {
		return dev, err
	}
	return t.getDev(opt.SID, opt.Dev)
}

func (t *Array) IsPowerMax() bool {
	return true
}

func (t *Array) IsThinDevFreed(sid, devId string) (bool, error) {
	devs, err := t.SymDevListThinDevs(sid, devId)
	if err != nil {
		return false, err
	}
	if devs[0].AllocTracks > 0 {
		return false, nil
	}
	return true, nil
}

func (t *Array) freeThinDev(sid, devId string) error {
	args := []string{"-sid", sid, "free", "-devs", devId, "-all", "-noprompt"}

	cmd := command.New(
		command.WithPrompt(PromptReader),
		command.WithName(t.symdev()),
		command.WithArgs(args),
		command.WithBufferedStdout(),
		command.WithBufferedStderr(),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithLogLevel(zerolog.InfoLevel),
		command.WithEnv(t.SymEnv()),
		command.WithLogger(t.Log()),
	)
	return cmd.Run()
}

func (t *Array) FreeThinDev(opt OptFreeThinDev) error {
	if opt.SID == "" {
		opt.SID = t.kwSID()
	}
	for {
		err := t.freeThinDev(opt.SID, opt.Dev)
		if err != nil {
			return err
		}
		if v, err := t.IsThinDevFreed(opt.SID, opt.Dev); err != nil {
			return err
		} else if !v {
			continue
		}
		if v, err := t.IsThinDevStatusDeallocating(opt.SID, opt.Dev); err != nil {
			return err
		} else if v {
			t.Log().Infof("device %s status is deallocating", opt.Dev)
			continue
		} else {
			t.Log().Infof("device %s status is not deallocating", opt.Dev)
		}
		if v, err := t.IsThinDevStatusFreeingAll(opt.SID, opt.Dev); err != nil {
			return err
		} else if v {
			t.Log().Infof("device %s status is freeingall", opt.Dev)
			continue
		} else {
			t.Log().Infof("device %s status is not freeingall", opt.Dev)
		}
		time.Sleep(5 * time.Second)
		break
	}
	return nil
}

func (t *Array) IsThinDevStatusDeallocating(sid, devId string) (bool, error) {
	return t.SymCfgVerifyThinDevStatus(sid, devId, "-deallocating")
}

func (t *Array) IsThinDevStatusFreeingAll(sid, devId string) (bool, error) {
	return t.SymCfgVerifyThinDevStatus(sid, devId, "-freeingall")
}

func (t *Array) SymCfgVerifyThinDevStatus(sid, devId, status string) (bool, error) {
	args := []string{"-sid", sid, "verify", "-tdevs", "-devs", devId, status}

	cmd := command.New(
		command.WithPrompt(PromptReader),
		command.WithName(t.symcfg()),
		command.WithArgs(args),
		command.WithBufferedStdout(),
		command.WithBufferedStderr(),
		command.WithCommandLogLevel(zerolog.DebugLevel),
		command.WithLogLevel(zerolog.DebugLevel),
		command.WithEnv(t.SymEnv()),
		command.WithLogger(t.Log()),
	)
	err := cmd.Run()
	if err != nil {
		return false, err
	}
	b := cmd.Stdout()
	b = bytes.TrimSpace(b)
	l := bytes.Fields(b)
	if len(l) == 0 {
		return false, fmt.Errorf("unexpected verify output: %s", string(b))
	}
	if string(l[0]) == "None" {
		return false, nil
	}
	return true, nil
}

func (t *Array) setDevRO(sid, devId string) error {
	args := []string{"-sid", sid, "write_disable", devId, "-noprompt"}

	cmd := command.New(
		command.WithPrompt(PromptReader),
		command.WithName(t.symdev()),
		command.WithArgs(args),
		command.WithBufferedStdout(),
		command.WithBufferedStderr(),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithLogLevel(zerolog.InfoLevel),
		command.WithEnv(t.SymEnv()),
		command.WithLogger(t.Log()),
	)
	return cmd.Run()
}

func (t *Array) SetSRDFMode(opt OptSetSRDFMode) error {
	if opt.SID == "" {
		opt.SID = t.kwSID()
	}
	if opt.SRDFMode == "" {
		return fmt.Errorf("--srdf-mode is required")
	}
	dev, err := t.getDev(opt.SID, opt.Dev)
	if err != nil {
		return err
	}
	if dev.RDF == nil {
		return fmt.Errorf("dev %s is not in a RDF relation", opt.Dev)
	}

	rdfg := fmt.Sprint(dev.RDF.Local.RAGroupNum)
	dst := dev.RDF.Remote.DevName

	pairFile, err := t.writePairFile(opt.Dev, dst)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(pairFile) }()

	args := []string{"-sid", opt.SID, "-f", pairFile, "-rdfg", rdfg, "set", "mode", opt.SRDFMode, "-noprompt"}

	cmd := command.New(
		command.WithPrompt(PromptReader),
		command.WithName(t.symrdf()),
		command.WithArgs(args),
		command.WithBufferedStdout(),
		command.WithBufferedStderr(),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithLogLevel(zerolog.InfoLevel),
		command.WithEnv(t.SymEnv()),
		command.WithLogger(t.Log()),
	)
	return cmd.Run()
}

func (t *Array) AddThinDev(opt OptAddThinDev) (Device, error) {
	if opt.SID == "" {
		opt.SID = t.kwSID()
	}
	if opt.SRDF && opt.RDFG == "" {
		return Device{}, fmt.Errorf("--srdf is specified but --rdfg is not")
	}
	r1Devs, err := t.CreateThinDev(opt)
	if err != nil {
		return Device{}, err
	}
	r1 := r1Devs[0]
	if opt.SRDF {
		rdfg, err := t.SymCfgRDFGList(opt.RDFG)
		if err != nil {
			return Device{}, err
		}
		if len(rdfg) == 0 {
			return Device{}, fmt.Errorf("can't find remote sid of rdfg %s", opt.RDFG)
		}
		r2Devs, err := t.CreateThinDev(OptAddThinDev{
			Name:     opt.Name,
			RDFG:     opt.RDFG,
			Size:     opt.Size,
			SG:       opt.SG,
			SRDF:     opt.SRDF,
			SRDFMode: opt.SRDFMode,
			SRDFType: opt.SRDFType,
			SID:      rdfg[0].RemoteSymId,
		})
		if err != nil {
			return Device{}, err
		}
		r2 := r2Devs[0]
		err = t.CreatePair(OptCreatePair{
			Pair:     r1 + ":" + r2,
			RDFG:     opt.RDFG,
			SRDFMode: opt.SRDFMode,
			SRDFType: opt.SRDFType,
			SID:      opt.SID,
		})
		if err != nil {
			return Device{}, err
		}
	}
	return t.getDev(opt.SID, r1Devs[0])
}

func (t *Array) getDevsFromCreateThinDevOutput(b []byte) ([]string, error) {
	reader := bytes.NewReader(b)
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)
		if strings.Contains(line, "devices created are") {
			begin := strings.Index(line, "[ ") + 1
			end := strings.Index(line, " ]")
			return strings.Fields(line[begin:end]), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("device not found in 'symdev create -tdev' output: %s", string(b))
}

func (t *Array) CreateThinDev(opt OptAddThinDev) ([]string, error) {
	if opt.Name == "" {
		return nil, fmt.Errorf("--name is required")
	}
	if opt.Size == "" {
		return nil, fmt.Errorf("--size is required")
	}
	sizeBytes, err := sizeconv.FromSize(opt.Size)
	if err != nil {
		return nil, err
	}
	if opt.SID == "" {
		opt.SID = t.kwSID()
	}

	args := []string{"-sid", opt.SID, "create", "-tdev", "-N", "1", "-cap", fmt.Sprint(sizeBytes / 1024 / 1024), "-captype", "mb"}
	if opt.SG != "" {
		args = append(args, "-sg", opt.SG)
	}
	args = append(args, "-emulation", "FBA", "-device_name", opt.Name, "-noprompt", "-v")

	cmd := command.New(
		command.WithPrompt(PromptReader),
		command.WithName(t.symdev()),
		command.WithArgs(args),
		command.WithBufferedStdout(),
		command.WithBufferedStderr(),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithLogLevel(zerolog.InfoLevel),
		command.WithEnv(t.SymEnv()),
		command.WithLogger(t.Log()),
	)
	err = cmd.Run()
	if err != nil {
		return nil, err
	}
	b := cmd.Stdout()

	return t.getDevsFromCreateThinDevOutput(b)
}

func (t *Array) DelThinDev(opt OptDelThinDev) (Device, error) {
	if opt.SID == "" {
		opt.SID = t.kwSID()
	}
	dev, err := t.getDev(opt.SID, opt.Dev)
	if err != nil {
		return Device{}, err
	}
	err = t.delThinDev(opt.SID, opt.Dev)
	if err != nil {
		return Device{}, err
	}
	return dev, nil
}

func (t *Array) delThinDev(sid, devId string) error {
	args := []string{"-sid", sid, "delete", devId, "-noprompt"}

	cmd := command.New(
		command.WithPrompt(PromptReader),
		command.WithName(t.symdev()),
		command.WithArgs(args),
		command.WithBufferedStdout(),
		command.WithBufferedStderr(),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithLogLevel(zerolog.InfoLevel),
		command.WithEnv(t.SymEnv()),
		command.WithLogger(t.Log()),
	)
	err := cmd.Run()
	if err != nil {
		stderr := string(cmd.Stderr())
		if strings.Contains(stderr, "A free of all allocations is required") {
			return ErrNotFree
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
	defer f.Close()
	if _, err := fmt.Fprintf(f, "%s %s\n", src, dst); err != nil {
		return "", err
	}
	return path, nil
}

func (t *Array) CreatePair(opt OptCreatePair) error {
	if opt.Pair == "" {
		return fmt.Errorf("--pair is required")
	}
	if opt.SRDFType == "" {
		return fmt.Errorf("--srdf-type is required")
	}
	if opt.SRDFMode == "" {
		return fmt.Errorf("--srdf-mode is required")
	}
	l := strings.Split(opt.Pair, ":")
	if len(l) != 1 {
		return fmt.Errorf("misformatted pair %s: expect 1 column", opt.Pair)
	}
	src := l[0]
	dst := l[1]

	if opt.SID == "" {
		opt.SID = t.kwSID()
	}

	srcDev, err := t.SymDevShow(opt.SID, src)
	if err != nil {
		return err
	}
	if srcDev[0].RDF != nil {
		return fmt.Errorf("dev %s is already is in a RDF relation", src)
	}
	pairFile, err := t.writePairFile(src, dst)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(pairFile) }()
	return t.runCreatePair(opt.SID, pairFile, opt.RDFG, opt.SRDFMode, opt.SRDFType, opt.Invalidate)
}

func (t *Array) runCreatePair(sid, pairFile, rdfg, rdfMode, rdfType, invalidate string) error {
	args := []string{"-sid", sid, "-f", pairFile, "-rdfg", rdfg, "createpair", "-rdf_mode", rdfMode, "-type", rdfType}
	if invalidate == "R1" || invalidate == "R2" {
		args = append(args, "-invalidate", invalidate)
	} else {
		args = append(args, "-establish")
	}
	cmd := command.New(
		command.WithPrompt(PromptReader),
		command.WithName(t.symrdf()),
		command.WithArgs(args),
		command.WithBufferedStdout(),
		command.WithBufferedStderr(),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithLogLevel(zerolog.InfoLevel),
		command.WithEnv(t.SymEnv()),
		command.WithLogger(t.Log()),
	)
	return cmd.Run()
}

func (t *Array) runSuspendPair(sid, pairFile, rdfg string) error {
	args := []string{"-sid", sid, "-f", pairFile, "-rdfg", rdfg, "suspend", "-noprompt"}
	cmd := command.New(
		command.WithPrompt(PromptReader),
		command.WithName(t.symrdf()),
		command.WithArgs(args),
		command.WithBufferedStdout(),
		command.WithBufferedStderr(),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithLogLevel(zerolog.InfoLevel),
		command.WithEnv(t.SymEnv()),
		command.WithLogger(t.Log()),
	)
	return cmd.Run()
}

func (t *Array) runDeletePair(sid, pairFile, rdfg string) error {
	args := []string{"-sid", sid, "-f", pairFile, "-rdfg", rdfg, "delepair", "-noprompt", "-force"}
	cmd := command.New(
		command.WithPrompt(PromptReader),
		command.WithName(t.symrdf()),
		command.WithArgs(args),
		command.WithBufferedStdout(),
		command.WithBufferedStderr(),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithLogLevel(zerolog.InfoLevel),
		command.WithEnv(t.SymEnv()),
		command.WithLogger(t.Log()),
	)
	return cmd.Run()
}

func (t *Array) deletePair(sid, devId string) (*RDF, error) {
	dev, err := t.getDev(sid, devId)
	if err != nil {
		return nil, err
	}
	if dev.RDF == nil {
		t.log.Debugf("dev %s is not in a RDF relation", devId)
		return nil, nil
	}
	rdfg := fmt.Sprint(dev.RDF.Local.RAGroupNum)
	dst := dev.RDF.Remote.DevName
	pairFile, err := t.writePairFile(devId, dst)
	if err != nil {
		return dev.RDF, err
	}
	defer func() { _ = os.Remove(pairFile) }()

	if dev.RDF.Info.PairState != "Suspended" {
		if err := t.runSuspendPair(sid, pairFile, rdfg); err != nil {
			return dev.RDF, err
		}
	}

	err = t.runDeletePair(sid, pairFile, rdfg)
	if err != nil {
		return dev.RDF, err
	}
	return dev.RDF, err
}

func (t *Array) DeletePair(opt OptDeletePair) (*RDF, error) {
	if opt.SID == "" {
		opt.SID = t.kwSID()
	}
	return t.deletePair(opt.SID, opt.Dev)
}

func (t *Array) MapDisk(opt OptMapDisk) (array.Disk, error) {
	var disk array.Disk
	if opt.SID == "" {
		opt.SID = t.kwSID()
	}
	dev, err := t.getDev(opt.SID, opt.Dev)
	if err != nil {
		return disk, err
	}

	driverData := make(map[string]any)
	driverData["dev"] = dev
	disk.DriverData = driverData
	disk.DiskID = dev.Product.WWN
	disk.DevID = dev.DevInfo.DevName

	if err := t.mapDisk(opt); err != nil {
		return disk, err
	}

	if data, err := t.getMappings(opt.SID, opt.Dev, opt.Mappings); err != nil {
		return disk, err
	} else {
		disk.Mappings = data
	}

	return disk, nil
}

func (t *Array) mapDisk(opt OptMapDisk) error {
	if opt.SG == "" {
		sg, err := t.bestSG(opt.SID, opt.Mappings, opt.SLO, opt.SRP)
		if err != nil {
			return err
		}
		opt.SG = sg
	}
	if err := t.addThinDevToSG(opt.SID, opt.Dev, opt.SG); err != nil {
		return err
	}
	return nil
}

func (t *Array) getMappings(sid, devId string, mappings []string) (array.Mappings, error) {
	sgs, err := t.getDevSGs(sid, dev)
	if err != nil {
		return nil, err
	}
	arrayMappings := make(array.Mappings)
	for _, sg := range sgs {
		for _, viewName := range sg.GroupInfo.MaskViewNames.ViewNames {
			view, err := t.getView(viewName)
			if err != nil {
				return nil, err
			}
			for _, portInfo := range view.ViewInfo.PortInfo.DirectorIdentifications {
				tgtId := portInfo.PortWWN
				for _, initiator := range view.ViewInfo.InitiatorList.Initiators {
					if initiator.WWN == nil {
						continue
					}
					hbaId := *initiator.WWN
					key := hbaId + ":" + tgtId
					for _, device := range view.ViewInfo.Devices {
						if device.DevName != devId {
							continue
						}
						for _, devPortInfo := range device.DevPortInfo {
							if devPortInfo.Port != portInfo.Port {
								continue
							}
							lun, err := strconv.ParseInt(devPortInfo.HostLUN, 16, 64)
							if err != nil {
								return arrayMappings, err
							}
							arrayMappings[key] = array.Mapping{
								TGTID: tgtId,
								HBAID: hbaId,
								LUN:   fmt.Sprint(lun),
							}
						}
					}
				}
			}
		}
	}
	return arrayMappings, nil
}

func (t *Array) AddDisk(opt OptAddDisk) (array.Disk, error) {
	var disk array.Disk
	if opt.SID == "" {
		opt.SID = t.kwSID()
	}
	dev, err := t.AddThinDev(OptAddThinDev{
		Name:     opt.Name,
		RDFG:     opt.RDFG,
		Size:     opt.Size,
		SLO:      opt.SLO,
		SG:       opt.SG,
		SRDF:     opt.SRDF,
		SRDFMode: opt.SRDFMode,
		SRDFType: opt.SRDFType,
		SID:      opt.SID,
	})
	if err != nil {
		return disk, err
	}

	driverData := make(map[string]any)
	driverData["dev"] = dev
	disk.DriverData = driverData
	disk.DiskID = dev.Product.WWN
	disk.DevID = dev.DevInfo.DevName
	disk.DriverData = driverData

	if err := t.mapDisk(OptMapDisk{
		Dev:      dev.DevInfo.DevName,
		SID:      opt.SID,
		SLO:      opt.SLO,
		SRP:      opt.SRP,
		SG:       opt.SG,
		Mappings: opt.Mappings,
	}); err != nil {
		return disk, err
	}

	dev, err = t.getDev(opt.SID, dev.DevInfo.DevName)
	if err != nil {
		return disk, err
	}

	if data, err := t.getMappings(opt.SID, dev.DevInfo.DevName, opt.Mappings); err != nil {
		return disk, err
	} else {
		disk.Mappings = data
	}

	driverData["dev"] = dev
	disk.DriverData = driverData
	return disk, nil
}

func (t *Array) unmap(sid, devId string) error {
	sgs, err := t.getDevSGs(sid, devId)
	if err != nil {
		return err
	}
	for _, sg := range sgs {
		if err := t.removeThinDevFromSG(sid, dev, sg.GroupInfo.GroupName); err != nil {
			return err
		}
	}
	return nil
}

func (t *Array) UnmapDisk(opt OptUnmapDisk) (array.Disk, error) {
	var disk array.Disk
	if opt.SID == "" {
		opt.SID = t.kwSID()
	}
	dev, err := t.getDev(opt.SID, opt.Dev)
	if err != nil {
		return disk, err
	}
	if err := t.unmap(opt.SID, opt.Dev); err != nil {
		return disk, err
	}

	driverData := make(map[string]any)
	driverData["dev"] = dev
	disk.DriverData = driverData
	disk.DiskID = dev.Product.WWN
	disk.DevID = dev.DevInfo.DevName
	disk.DriverData = driverData

	return disk, nil
}

func (t *Array) DelDisk(opt OptDelDisk) (array.Disk, error) {
	var disk array.Disk
	if opt.SID == "" {
		opt.SID = t.kwSID()
	}

	dev, err := t.getDev(opt.SID, opt.Dev)
	if err != nil {
		return disk, err
	}

	if dev.DevInfo.SnapvxSource {
		return disk, fmt.Errorf("dev %s is a snapvx_source. can not delete", opt.Dev)
	}
	if err := t.setDevRO(opt.SID, opt.Dev); err != nil {
		return disk, err
	}
	if err := t.unmap(opt.SID, opt.Dev); err != nil {
		return disk, err
	}
	if _, err := t.deletePair(opt.SID, opt.Dev); err != nil {
		return disk, err
	}

	maxRetry := 5
	retryDelay := 5 * time.Second

	for i := 1; i <= maxRetry; i++ {
		if err := t.freeThinDev(opt.SID, opt.Dev); err != nil {
			return disk, err
		}
		if err := t.delThinDev(opt.SID, opt.Dev); err != nil {
			if errors.Is(err, ErrNotFree) {
				if i >= maxRetry {
					return disk, fmt.Errorf("dev %s is still not free of all allocations after 5 tries", opt.Dev)
				} else {
					time.Sleep(retryDelay)
					continue
				}
			}
			return disk, err
		}

	}

	driverData := make(map[string]any)
	driverData["dev"] = dev
	disk.DriverData = driverData
	disk.DiskID = dev.Product.WWN
	disk.DevID = dev.DevInfo.DevName
	disk.DriverData = driverData

	return disk, nil
}

func (t *Array) AddMasking(b []byte) (MaskingDump, error) {
	var data MaskingDump
	if err := json.Unmarshal(b, &data); err != nil {
		return MaskingDump{}, err
	}
	return t.addMasking(data)
}

// Dump/Restore of masking views
type (
	Result struct {
		Cmd []string `json:"cmd"`
		Ret int      `json:"ret"`
		Out string   `json:"out"`
		Err string   `json:"err"`
	}
	MaskingDump struct {
		InitiatorGroups []MaskingDumpIG   `json:"ig"`
		StorageGroups   []MaskingDumpSG   `json:"sg"`
		Gatekeepers     []MaskingDumpGK   `json:"gk"`
		Devices         []MaskingDumpDev  `json:"dev"`
		Views           []MaskingDumpView `json:"mv"`
	}
	MaskingDumpIG struct {
		Name            string   `json:"name"`
		HBAIds          []string `json:"hba_ids"`
		InitiatorGroups []string `json:"igs"`
		Consistent      *bool    `json:"consistent"`
		Results         []Result `json:"result"`
	}
	MaskingDumpSG struct {
		Name          string   `json:"name"`
		SRP           string   `json:"srp"`
		SLO           string   `json:"slo"`
		StorageGroups []string `json:"sg"`
		Results       []Result `json:"result"`
	}
	MaskingDumpGK struct {
		StorageGroup string   `json:"sg"`
		Count        *int     `json:"count"`
		Results      []Result `json:"result"`
	}
	MaskingDumpDev struct {
		Name         string   `json:"name"`
		Size         string   `json:"size"`
		StorageGroup string   `json:"sg"`
		Results      []Result `json:"result"`
	}
	MaskingDumpView struct {
		Name                string   `json:"name"`
		PortIds             []string `json:"pg"`
		StorageGroupNames   []string `json:"sgs"`
		InitiatorGroupNames []string `json:"igs"`
		Results             []Result `json:"result"`
	}
)

var (
	DumpDefaultGKCount    = 6
	DumpDefaultConsistent = true
)

func (t *Array) addMasking(data MaskingDump) (MaskingDump, error) {
	var err error
	data, err = t.addDumpInitiatorGroups(data)
	if err != nil {
		return data, err
	}
	data, err = t.addDumpStorageGroups(data)
	if err != nil {
		return data, err
	}
	data, err = t.addDumpGatekeepers(data)
	if err != nil {
		return data, err
	}
	data, err = t.addDumpDevices(data)
	if err != nil {
		return data, err
	}
	data, err = t.addDumpViews(data)
	if err != nil {
		return data, err
	}
	return data, nil
}

func (t *Array) addDumpInitiatorGroups(data MaskingDump) (MaskingDump, error) {
	for i, e := range data.InitiatorGroups {
		results, err := t.addDumpInitiatorGroup(e)
		if err != nil {
			return data, err
		}
		e.Results = append(e.Results, results...)
		data.InitiatorGroups[i] = e
	}
	return data, nil
}

func (t *Array) addDumpStorageGroups(data MaskingDump) (MaskingDump, error) {
	for i, e := range data.StorageGroups {
		results, err := t.addDumpStorageGroup(e)
		if err != nil {
			return data, err
		}
		e.Results = append(e.Results, results...)
		data.StorageGroups[i] = e
	}
	return data, nil
}

func (t *Array) addDumpGatekeepers(data MaskingDump) (MaskingDump, error) {
	for i, e := range data.Gatekeepers {
		results, err := t.addDumpGatekeeper(e)
		if err != nil {
			return data, err
		}
		e.Results = append(e.Results, results...)
		data.Gatekeepers[i] = e
	}
	return data, nil
}

func (t *Array) addDumpDevices(data MaskingDump) (MaskingDump, error) {
	for i, e := range data.Devices {
		results, err := t.addDumpDevice(e)
		if err != nil {
			return data, err
		}
		e.Results = append(e.Results, results...)
		data.Devices[i] = e
	}
	return data, nil
}

func (t *Array) addDumpViews(data MaskingDump) (MaskingDump, error) {
	for i, e := range data.Views {
		results, err := t.addDumpView(e)
		if err != nil {
			return data, err
		}
		e.Results = results
		e.Results = append(e.Results, results...)
		data.Views[i] = e
	}
	return data, nil
}

func (t *Array) addDumpStorageGroup(data MaskingDumpSG) ([]Result, error) {
	var results []Result
	if result, err := t.createStorageGroup(t.kwSID(), data.Name, data.SRP, data.SLO); err != nil {
		return results, err
	} else {
		results = append(results, result)
	}
	if result, err := t.addStorageGroupsToStorageGroup(t.kwSID(), data.Name, data.StorageGroups); err != nil {
		return results, err
	} else {
		results = append(results, result)
	}
	return results, nil
}

func (t *Array) addDumpInitiatorGroup(data MaskingDumpIG) ([]Result, error) {
	var results []Result
	if data.Consistent == nil {
		v := DumpDefaultConsistent
		data.Consistent = &v
	}
	if result, err := t.createInitiator(t.kwSID(), data.Name, *data.Consistent); err != nil {
		return results, err
	} else {
		results = append(results, result)
	}
	for _, ig := range data.InitiatorGroups {
		if result, err := t.addInitiatorToInitiatorGroup(t.kwSID(), data.Name, ig); err != nil {
			return results, err
		} else {
			results = append(results, result)
		}
	}
	for _, hbaId := range data.HBAIds {
		if result, err := t.addHBAToInitiator(t.kwSID(), data.Name, hbaId); err != nil {
			return results, err
		} else {
			results = append(results, result)
		}
	}
	return results, nil
}

func (t *Array) addDumpGatekeeper(data MaskingDumpGK) ([]Result, error) {
	var results []Result
	if result, err := t.createGatekeepers(t.kwSID(), data.StorageGroup, data.Count); err != nil {
		return results, err
	} else {
		results = append(results, result)
	}
	return results, nil
}

func (t *Array) addDumpDevice(data MaskingDumpDev) ([]Result, error) {
	var results []Result
	if result, err := t.createThinDev(t.kwSID(), data.Name, data.Size, data.StorageGroup); err != nil {
		return results, err
	} else {
		results = append(results, result)
	}
	return results, nil
}

func (t *Array) addDumpView(data MaskingDumpView) ([]Result, error) {
	var results []Result
	if result, err := t.createView(t.kwSID(), data.Name, data.PortIds, data.StorageGroupNames, data.InitiatorGroupNames); err != nil {
		return results, err
	} else {
		results = append(results, result)
	}
	return results, nil
}

func (t ShowPortGroup) HasPort(tgtId string) bool {
	for _, directorId := range t.GroupInfo.DirectorIdentifications {
		if directorId.PortWWN == tgtId {
			return true
		}
	}
	return false
}

func (t ShowPortGroup) HasAllPortOf(tgtIds []string) bool {
	if len(tgtIds) != len(t.GroupInfo.DirectorIdentifications) {
		return false
	}
	for _, tgtId := range tgtIds {
		if !t.HasPort(tgtId) {
			return false
		}
	}
	return true
}

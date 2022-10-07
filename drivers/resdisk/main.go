package resdisk

import (
	"opensvc.com/opensvc/core/keywords"
	"opensvc.com/opensvc/core/rawconfig"
	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/util/converters"
)

type (
	T struct {
		resource.T
		PRKey          string
		PromoteRW      bool
		NoPreemptAbort bool
		SCSIReserv     bool
	}
)

var (
	KWPRKey = keywords.Keyword{
		Option:   "prkey",
		Attr:     "PRKey",
		Scopable: true,
		Text:     "Defines a specific persistent reservation key for the resource. Takes priority over the service-level defined prkey and the node.conf specified prkey.",
	}
	KWPromoteRW = keywords.Keyword{
		Option:    "promote_rw",
		Attr:      "PromoteRW",
		Converter: converters.Bool,
		Text:      "If set to ``true``, OpenSVC will try to promote the base devices to read-write on start.",
	}
	KWNoPreemptAbort = keywords.Keyword{
		Option:    "no_preempt_abort",
		Attr:      "NoPreemptAbort",
		Scopable:  true,
		Converter: converters.Bool,
		Text:      "If set to ``true``, OpenSVC will preempt scsi reservation with a preempt command instead of a preempt and and abort. Some scsi target implementations do not support this last mode (esx). If set to ``false`` or not set, :kw:`no_preempt_abort` can be activated on a per-resource basis.",
	}
	KWSCSIReserv = keywords.Keyword{
		Option:    "scsireserv",
		Attr:      "SCSIReserv",
		Converter: converters.Bool,
		Text:      "If set to ``true``, OpenSVC will try to acquire a type-5 (write exclusive, registrant only) scsi3 persistent reservation on every path to every disks held by this resource. Existing reservations are preempted to not block service start-up. If the start-up was not legitimate the data are still protected from being written over from both nodes. If set to ``false`` or not set, :kw:`scsireserv` can be activated on a per-resource basis.",
	}

	BaseKeywords = []keywords.Keyword{
		KWPRKey,
		KWPromoteRW,
		KWNoPreemptAbort,
		KWSCSIReserv,
	}
)

func (t T) IsSCSIPersistentReservationPreemptAbortDisabled() bool {
	return t.NoPreemptAbort
}

func (t T) IsSCSIPersistentReservationEnabled() bool {
	return t.SCSIReserv
}

func (t T) PersistentReservationKey() string {
	if t.PRKey != "" {
		return t.PRKey
	}
	if nodePRKey := rawconfig.NodeSection().PRKey; nodePRKey != "" {
		return nodePRKey
	}
	return ""
}

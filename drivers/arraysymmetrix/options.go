package arraysymmetrix

import "github.com/opensvc/om3/v3/core/array"

// The options each action of this array is asked for.

type OptResizeDisk struct {
	Dev   string
	SID   string
	Size  string
	Force bool
}

type OptUnmapDisk struct {
	Dev string
	SID string
}

type OptRenameDisk struct {
	Dev  string
	Name string
	SID  string
}

type OptMapDisk struct {
	Dev      string
	SID      string
	SLO      string
	SRP      string
	SG       string
	Mappings array.Mappings
}

type OptDelDisk struct {
	Dev string
	SID string
}

type OptAddThinDev struct {
	Name     string
	RDFG     string
	Size     string
	SG       string
	SLO      string
	SRDF     bool
	SRDFMode string
	SRDFType string
	SID      string
}

type OptDelThinDev struct {
	Dev string
	SID string
}

type OptDeletePair struct {
	Dev string
	SID string
}

type OptCreatePair struct {
	Pair       string
	RDFG       string
	Invalidate string
	SID        string
	SRDFMode   string
	SRDFType   string
}

type OptAddDisk struct {
	Name     string
	Size     string
	SID      string
	SG       string
	SLO      string
	SRP      string
	SRDF     bool
	SRDFMode string
	SRDFType string
	RDFG     string
	Mappings array.Mappings
}

type OptFreeThinDev struct {
	SID string
	Dev string
}

type OptSetSRDFMode struct {
	SRDFMode string
	Dev      string
	SID      string
}

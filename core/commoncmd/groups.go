package commoncmd

import "github.com/spf13/cobra"

var (
	GroupIDObjectKinds    = "object kinds"
	GroupIDOrchestrated   = "orchestrated"
	GroupIDQuery          = "query"
	GroupIDSubsystems     = "subsystems"
	GroupIDResourceGroups = "resource groups"
	GroupIDReplication    = "replication"
)

// NewGroupObjectKinds returns the section listing the commands scoped to an
// object kind, at the root of the command tree.
func NewGroupObjectKinds() *cobra.Group {
	return &cobra.Group{
		ID:    GroupIDObjectKinds,
		Title: "Object Kinds:",
	}
}

func NewGroupOrchestrated() *cobra.Group {
	return &cobra.Group{
		ID:    GroupIDOrchestrated,
		Title: "Orchestrated Commands:",
	}
}

func NewGroupReplication() *cobra.Group {
	return &cobra.Group{
		ID:    GroupIDReplication,
		Title: "Replication Commands:",
	}
}

func NewGroupQuery() *cobra.Group {
	return &cobra.Group{
		ID:    GroupIDQuery,
		Title: "Query Commands:",
	}
}

func NewGroupSubsystems() *cobra.Group {
	return &cobra.Group{
		ID:    GroupIDSubsystems,
		Title: "Subsystems:",
	}
}

func NewGroupResources() *cobra.Group {
	return &cobra.Group{
		ID:    GroupIDResourceGroups,
		Title: "Resource Groups:",
	}
}

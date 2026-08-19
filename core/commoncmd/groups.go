package commoncmd

import "github.com/spf13/cobra"

var (
	GroupIDOrchestrated   = "orchestrated"
	GroupIDQuery          = "query"
	GroupIDSubsystems     = "subsystems"
	GroupIDResourceGroups = "resource groups"
	GroupIDReplication    = "replication"
)

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

package commoncmd

import "github.com/spf13/cobra"

var (
	GroupIDOrchestratedActions = "orchestrated actions"
	GroupIDQuery               = "query"
	GroupIDSubsystems          = "subsystems"
	GroupIDResourceGroups      = "resource groups"
)

func NewGroupOrchestratedActions() *cobra.Group {
	return &cobra.Group{
		ID:    GroupIDOrchestratedActions,
		Title: "Orchestrated Actions:",
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

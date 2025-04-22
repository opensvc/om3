package commoncmd

import "github.com/spf13/cobra"

var (
	GroupIDOrchestratedActions = "orchestrated actions"
	GroupIDQuery               = "query"
	GroupIDSubsystems          = "subsystems"
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

package rescontainerocibase

import (
	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/keywords"
	"github.com/opensvc/om3/v3/drivers/rescontainer"
)

// The groups of the keywords of the oci containers, docker and podman, beside
// the ones every container driver shares.
var (
	// GroupImage holds the image the container runs, and how it is pulled.
	GroupImage = keywords.Group{
		ID:    "image",
		Name:  "Image",
		Text:  keywords.NewText(fs, "text/group/image"),
		Order: 20,
	}

	// GroupProcess holds the process the container runs, and how the
	// engine runs it.
	GroupProcess = keywords.Group{
		ID:    "process",
		Name:  "Process",
		Text:  keywords.NewText(fs, "text/group/process"),
		Order: 30,
	}

	// GroupEnvironment holds the variables the process of the container
	// is given.
	GroupEnvironment = keywords.Group{
		ID:    "environment",
		Name:  "Environment",
		Text:  keywords.NewText(fs, "text/group/environment"),
		Order: 40,
	}

	// GroupIsolation holds the namespaces the container shares with the
	// node or another container, and the privileges it is given.
	GroupIsolation = keywords.Group{
		ID:    "isolation",
		Name:  "Isolation and privileges",
		Text:  keywords.NewText(fs, "text/group/isolation"),
		Order: 70,
	}

	// GroupQuiesce holds how the writes of the container are suspended
	// for a run of a sync resource. Its strategies are the two ways the
	// quiesce keyword says: pause, freezing the processes, reads none of
	// the keywords feeding a command, which a command does.
	//
	// The quiesce keyword is the selector, used by both: a configuration
	// writing pause and a quiesce_input is told by its value, which the
	// validation of the strategies does not read, so it is the wizard
	// alone that knows the input is ignored.
	GroupQuiesce = keywords.Group{
		ID:    "quiesce",
		Name:  "Quiesce",
		Text:  keywords.NewText(fs, "text/group/quiesce"),
		Order: 75,
		Strategies: []keywords.Strategy{
			{
				ID:       "pause",
				Name:     "Pause the processes",
				Text:     keywords.NewText(fs, "text/strategy/quiesce/pause"),
				Uses:     []string{"quiesce"},
				Excludes: []string{"quiesce_input", "quiesce_ready", "resume_input", "quiesce_timeout"},
				When:     keyop.ParseList("quiesce=pause"),
			},
			{
				ID:   "command",
				Name: "Run a command in the container",
				Text: keywords.NewText(fs, "text/strategy/quiesce/command"),
				Uses: []string{"quiesce", "quiesce_input", "quiesce_ready", "resume_input", "quiesce_timeout"},
				When: keyop.ParseList("quiesce!=pause"),
			},
		},
	}

	// Groups are the groups the keywords of an oci container name.
	Groups = []keywords.Group{
		rescontainer.GroupIdentity,
		GroupImage,
		GroupProcess,
		GroupEnvironment,
		rescontainer.GroupStorage,
		rescontainer.GroupNetwork,
		GroupIsolation,
		GroupQuiesce,
		rescontainer.GroupGuest,
		rescontainer.GroupTimeouts,
	}
)

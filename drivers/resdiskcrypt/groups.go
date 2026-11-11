//go:build linux

package resdiskcrypt

import (
	"github.com/opensvc/om3/v3/core/keywords"
)

// The groups of the keywords of a crypt disk beside the ones of the disk
// drivers: the passphrase opening the device, and how the provision derives
// the key of the LUKS header from it.
//
// A passphrase managed by om or supplied by the user is no strategy of the
// passphrase group: manage_passphrase chooses between the two, and the secret
// keeps the passphrase either way.
var (
	// GroupPassphrase holds where the passphrase opening the device is
	// kept, and whether om generates it.
	GroupPassphrase = keywords.Group{
		ID:    "passphrase",
		Name:  "Passphrase",
		Text:  keywords.NewText(fs, "text/group/passphrase"),
		Order: 40,
	}

	// GroupKeyDerivation holds the key derivation function the provision
	// formats the LUKS header with, and its costs.
	GroupKeyDerivation = keywords.Group{
		ID:    "key_derivation",
		Name:  "Key derivation",
		Text:  keywords.NewText(fs, "text/group/key_derivation"),
		Order: 42,
	}
)

package naming

import (
	"fmt"
	"math/rand"
)

var (
	traits = []string{
		"active", "brave", "bright", "calm", "chic", "civil", "clean", "clever", "cool",
		"crafty", "daring", "decent", "eager", "fair", "faith", "fiery", "fierce",
		"fit", "funny", "gentle", "giving", "glad", "grace", "happy", "hardy", "humble",
		"ideal", "jolly", "joyful", "just", "kind", "lively", "loyal", "merry", "moral",
		"noble", "patent", "peace", "polite", "proud", "quick", "quiet", "ready",
		"relax", "reliable", "right", "robust", "sage", "sharp", "shrewd", "sincere",
		"smart", "solid", "sound", "sweet", "tender", "thrifty", "tidy", "tough",
		"trusty", "upbeat", "vivid", "warm", "witty", "worthy", "young", "zestful",
		"able", "adapt", "alert", "apt", "aware", "bold", "brisk", "calm", "chill",
		"chief", "classy", "clean", "clear", "couth", "coy", "dear", "devout", "dreamy",
		"elite", "equal", "exact", "famed", "fast", "fit", "famed", "fine", "firm",
		"flash", "frank", "grand", "great",
	}
	animals = []string{
		"bat", "bear", "bee", "bison", "boar", "bug", "cat", "clam", "cobra", "crab",
		"crow", "deer", "dingo", "dog", "dove", "duck", "eagle", "eel", "ferret",
		"finch", "fish", "fox", "frog", "gecko", "goat", "goose", "gopher", "horse",
		"hyena", "ibex", "jaguar", "koala", "lemur", "leopard", "lion", "lizard",
		"llama", "lobster", "lynx", "mole", "moose", "mouse", "mule", "newt", "otter",
		"owl", "ox", "panda", "peafowl", "penguin", "pig", "pike", "quail", "rat",
		"raven", "roach", "seal", "shark", "sheep", "skunk", "sloth", "snail", "snake",
		"squid", "swan", "swift", "tapir", "tiger", "toad", "trout", "tuna", "turkey",
		"viper", "vole", "wasp", "whale", "wolf", "wombat", "worm", "wren", "yak",
		"zebra", "zebu",
	}
)

func Random() string {
	trait := traits[rand.Intn(len(traits))]
	animal := animals[rand.Intn(len(animals))]
	return fmt.Sprintf("%v-%v", trait, animal)
}

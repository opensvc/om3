package df

// DFEntry represents a parsed line of the df unix command
type DFEntry struct {
	Device      string
	Total       int64
	Used        int64
	Free        int64
	UsedPercent int64
	MountPoint  string
}

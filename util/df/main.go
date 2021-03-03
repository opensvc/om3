package df

// Entry represents a parsed line of the df unix command
type Entry struct {
	Device      string
	Total       int64
	Used        int64
	Free        int64
	UsedPercent int64
	MountPoint  string
}

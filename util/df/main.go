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

// Usage executes and parses a df command
func Usage() ([]Entry, error) {
	b, err := doDFUsage()
	if err != nil {
		return nil, err
	}
	return parse(b)
}

// Inode executes and parses a df command
func Inode() ([]Entry, error) {
	b, err := doDFInode()
	if err != nil {
		return nil, err
	}
	return parse(b)
}

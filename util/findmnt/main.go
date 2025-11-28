package findmnt

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strings"

	"github.com/opensvc/om3/util/capabilities"
	"github.com/opensvc/om3/util/file"
)

type (
	MountInfo struct {
		Source  string `json:"source"`
		Target  string `json:"target"`
		FsType  string `json:"fstype"`
		Options string `json:"options"`
	}

	info struct {
		Filesystems []MountInfo `json:"filesystems"`
	}
)

const (
	PathNfsSeparator = ':'
	JsonCapability   = "node.x.findmnt.json"
)

var (
	jsonFlag = "-J"
)

// Has returns true when {dev} is mounted on {mnt} using the findmnt command
func Has(dev string, mnt string) (bool, error) {
	l, err := List(dev, mnt)
	if err != nil {
		return false, err
	}
	return len(l) > 0, nil
}

// HasMntWithTypes returns true when a fs with type matching one of {fsTypes} is mounted on {mnt} using the findmnt command
func HasMntWithTypes(fsTypes []string, mnt string) (bool, error) {
	l, err := List("", mnt)
	if err != nil {
		return false, err
	}
	for _, m := range l {
		if slices.Contains(fsTypes, m.FsType) {
			return true, nil
		}
	}
	return false, nil
}

// HasFromMount returns true when {dev} is mounted on {mnt} using the mount command
func HasFromMount(dev string, mnt string) (bool, error) {
	cmd := mountCmd()
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, err
	}
	split := strings.Split(string(output), "\n")
	for _, line := range split {
		elems := strings.Split(line, " ")
		if len(elems) >= 3 {
			if elems[0] == dev && elems[2] == mnt {
				return true, nil
			}
		}
	}
	return false, nil
}

func newInfo() *info {
	return &info{Filesystems: make([]MountInfo, 0)}
}

// List return matching dev and mnt list of MountInfo.
// findmnt exec is used to do initial filtering,
// then filter on mnt is used (for nfs) + custom filter on [dev] for bind mounts.
//
// We can't use findmnt -J -T {mnt} -S {dev} for nfs because it may hang.
// A timeout version of findmnt is not sufficient, we have to differentiate hang but mounted
// from not mounted.
//
// So when dev is on nfs, We can't use findmnt -J -T {mnt} -S {dev}
// Instead findmnt -J -S {dev} is used, then mnt is filtered within List function.
func List(dev string, mnt string) (mounts []MountInfo, err error) {
	var (
		devIsDir, devIsNfs bool
	)

	if _, err = exec.LookPath("findmnt"); err != nil {
		return
	}

	if dev != "" {
		if devIsDir, err = file.ExistsAndDir(dev); err != nil {
			return
		} else if !devIsDir {
			devIsNfs = isNfsPath(dev)
		}
	}

	args := findMntArgs(dev, mnt, devIsDir, devIsNfs)
	if mounts, err = findMnt(args); err != nil {
		return
	}

	if mnt != "" {
		filtered := make([]MountInfo, 0)
		for _, mi := range mounts {
			if mi.Target != mnt {
				continue
			}
			filtered = append(filtered, mi)
		}
		mounts = filtered
	}

	if devIsDir {
		filtered := make([]MountInfo, 0)
		pattern := fmt.Sprintf("[%s]", dev)
		for _, mi := range mounts {
			if !strings.Contains(mi.Source, pattern) {
				continue
			}
			filtered = append(filtered, mi)
		}
		mounts = filtered
	}
	return
}

// findMntArgs returns findmnt exec args for dev and mnt.
// When dev is on nfs, -T mnt is skipped to prevent command hang
// When dev is dir, -S dev is skipped
func findMntArgs(dev, mnt string, devIsDir, devIsNfs bool) []string {
	var opts []string
	if capabilities.Has(JsonCapability) {
		opts = append(opts, jsonFlag)
	} else {
		opts = append(opts, "-P")
	}

	if !devIsDir && dev != "" {
		opts = append(opts, "-S", dev)
	}
	if mnt != "" && !devIsNfs {
		opts = append(opts, "-T", mnt)
	}
	return opts
}

func findMnt(opts []string) (mounts []MountInfo, err error) {
	cmd := exec.Command("findmnt", opts...)
	stdout, err := cmd.Output()
	if err != nil {
		return nil, nil
	}

	if slices.Contains(opts, jsonFlag) {
		data := newInfo()
		if err := json.Unmarshal(stdout, &data); err != nil {
			return nil, err
		}
		return data.Filesystems, nil
	}
	return parseMountInfo(stdout)
}

func parseMountInfo(data []byte) ([]MountInfo, error) {
	lines := strings.Split(string(data), "\n")
	mounts := make([]MountInfo, 0)
	re := regexp.MustCompile(`(\w+)="([^"]*)"`)
	for _, line := range lines {
		if line == "" {
			continue
		}
		m := re.FindAllStringSubmatch(line, -1)

		quoteCount := strings.Count(line, `"`)
		if quoteCount%2 != 0 {
			return nil, fmt.Errorf("unmatched quotes in line: %s", line)
		}

		mi := MountInfo{}
		for _, match := range m {
			if len(match) != 3 {
				continue
			}
			key := match[1]
			value := match[2]
			switch key {
			case "SOURCE":
				mi.Source = value
			case "TARGET":
				mi.Target = value
			case "FSTYPE":
				mi.FsType = value
			case "OPTIONS":
				mi.Options = value
			default:
				return nil, fmt.Errorf("unexpected findmnt key: %s", key)
			}
		}
		mounts = append(mounts, mi)
	}
	return mounts, nil
}

func isNfsPath(s string) bool {
	if strings.HasPrefix(s, string(os.PathSeparator)) {
		return false
	}
	split := strings.Split(s, string(PathNfsSeparator))
	if len(split) != 2 {
		return false
	}
	return len(split[0]) > 0 && len(split[1]) > 0
}

func init() {
	capabilities.Register(CapabilitiesScanner)
}

func CapabilitiesScanner() ([]string, error) {
	l := make([]string, 0)
	cmd := exec.Command("findmnt", jsonFlag)
	if err := cmd.Run(); err == nil {
		l = append(l, JsonCapability)
	}
	return l, nil
}

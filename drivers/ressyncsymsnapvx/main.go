package ressyncsymsnapvx

import (
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/opensvc/om3/core/provisioned"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/drivers/ressync"
	"github.com/opensvc/om3/util/command"
	"github.com/opensvc/om3/util/xmap"
	"github.com/rs/zerolog"
)

// T is the driver structure.
type (
	T struct {
		ressync.T
		Absolute    string
		Devices     []string
		DevicesFrom []string
		Delta       string
		Name        string
		ObjectFQDN  string
		Secure      bool
		SymID       string
	}

	//
	// Example xml output:
	//
	//	<?xml version="1.0" standalone="yes" ?>
	//	<SymCLI_ML>
	//	  <Inquiry>
	//	    <Device>
	//	      <symid>000000000016</symid>
	//	      <pd_name>/dev/rdsk/c0t60000000000000000006533030334233d0s2</pd_name>
	//	      <dev_name>003B3</dev_name>
	//	      <director>1D</director>
	//	      <port>8</port>
	//	    </Device>
	//	  </Inquiry>
	//	</SymCLI_ML>
	//
	XInqPdevfile struct {
		XMLName xml.Name    `xml:"SymCLI_ML"`
		Inquiry XInqInquiry `xml:"Inquiry"`
	}
	XInqInquiry struct {
		XMLName xml.Name     `xml:"Inquiry"`
		Devices []XInqDevice `xml:"Device"`
	}
	XInqDevice struct {
		XMLName  xml.Name `xml:"Device"`
		SymID    string   `xml:"symid"`
		DevName  string   `xml:"dev_name"`
		Director string   `xml:"director"`
		Port     int      `xml:"port"`
	}

	// Example: symsnapvx list -output xml_e
	//
	// <?xml version="1.0" standalone="yes" ?>
	// <SymCLI_ML>
	//   <Symmetrix>
	//     <Symm_Info>
	//       <symid>000111111111</symid>
	//       <microcode_version>5978</microcode_version>
	//     </Symm_Info>
	//     <Snapvx>
	//       <Snapshot>
	//         <source>00097</source>
	//         <snapshot_name>SNAP_1</snapshot_name>
	//         <last_timestamp>Fri May 31 06:15:05 2024</last_timestamp>
	//         <num_generations>20</num_generations>
	//         <link>No</link>
	//         <restore>No</restore>
	//         <failed>No</failed>
	//         <error_reason>NA</error_reason>
	//         <GCM>False</GCM>
	//         <zDP>False</zDP>
	//         <secured>Yes</secured>
	//         <expanded>No</expanded>
	//         <bgdefinprog>No</bgdefinprog>
	//         <policy>No</policy>
	//         <persistent>No</persistent>
	//         <cloud>No</cloud>
	//       </Snapshot>
	//
	XSnapvxList struct {
		XMLName   xml.Name   `xml:"SymCLI_ML"`
		Symmetrix XSymmetrix `xml:"Symmetrix"`
	}
	XSymmetrix struct {
		XMLName xml.Name `xml:"Symmetrix"`
		Snapvx  XSnapvx  `xml:"Snapvx"`
	}
	XSnapvx struct {
		XMLName   xml.Name    `xml:"Snapvx"`
		Snapshots []XSnapshot `xml:"Snapshot"`
	}
	XSnapshot struct {
		XMLName        xml.Name `xml:"Snapshot"`
		Source         string   `xml:"source"`
		SnapshotName   string   `xml:"snapshot_name"`
		LastTimestamp  string   `xml:"last_timestamp"`
		NumGenerations int      `xml:"num_generations"`
		Link           string   `xml:"link"`
		Restore        string   `xml:"restore"`
		Failed         string   `xml:"failed"`
		ErrorReason    string   `xml:"error_reason"`
		Secured        string   `xml:"secured"`
		Expanded       string   `xml:"expanded"`
	}
)

const (
	timeFormat = time.ANSIC
	symsnapvx  = "/usr/symcli/bin/symsnapvx"
)

func New() resource.Driver {
	return &T{}
}

func (t *T) Update(ctx context.Context) error {
	if err := t.establish(); err != nil {
		return err
	}
	return nil
}

func (t *T) mergeDevs() ([]string, error) {
	m := make(map[string]any)
	for _, devID := range t.Devices {
		m[devID] = nil
	}
	for _, rid := range t.DevicesFrom {
		r := t.GetObjectDriver().ResourceByID(rid)
		if r == nil {
			t.StatusLog().Warn("referenced rid %s as devs source does not exist", rid)
			continue
		}
		i, ok := r.(resource.SubDeviceser)
		if !ok {
			t.StatusLog().Warn("referenced rid %s as devs source does not support sub devs listing", rid)
			continue
		}
		for _, dev := range i.SubDevices() {
			dev, err := t.devFromDevPath(dev.Path())
			if err != nil {
				return nil, err
			}
			if dev.SymID != t.SymID {
				continue
			}
			m[dev.DevName] = nil
		}
	}
	return xmap.Keys(m), nil
}

// devFromDevPath uses syminq to resolve a device path into a symmetrix device id.
func (t *T) devFromDevPath(devPath string) (XInqDevice, error) {
	info, err := os.Lstat(devPath)
	if err != nil {
		return XInqDevice{}, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(devPath)
		if err != nil {
			return XInqDevice{}, err
		}
		if !strings.HasPrefix(devPath, "/") {
			devPath = filepath.Join(filepath.Dir(devPath), target)
		}
	}
	args := []string{"-pdevfile", devPath, "-output", "xml_e"}
	cmd := exec.Command("syminq", args...)
	t.Log().Debugf("exec: %s", cmd)
	b, err := cmd.Output()
	if err != nil {
		return XInqDevice{}, fmt.Errorf("%s: %w", devPath, err)
	}
	head, err := parseInq(b)
	if err != nil {
		return XInqDevice{}, fmt.Errorf("%s: %w", devPath, err)
	}
	if n := len(head.Inquiry.Devices); n != 1 {
		return XInqDevice{}, fmt.Errorf("%s: expected 1 symdev from inq, got %d", devPath, n)
	}
	return head.Inquiry.Devices[0], nil
}

func parseInq(b []byte) (*XInqPdevfile, error) {
	var head XInqPdevfile
	if err := xml.Unmarshal(b, &head); err != nil {
		return nil, err
	}
	return &head, nil
}

func (t *T) List() ([]XSnapshot, error) {
	mergedDevs, err := t.mergeDevs()
	if err != nil {
		return nil, err
	}
	args := []string{"list", "-sid", t.SymID, "-devs", strings.Join(mergedDevs, ","), "-output", "xml_e"}
	cmd := exec.Command(symsnapvx, args...)
	t.Log().Debugf("exec: %s", cmd)
	b, err := cmd.Output()
	if err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			if len(e.Stderr) != 0 {
				return nil, err
			}
		} else {
			return nil, err
		}
	}
	var head XSnapvxList
	if err := xml.Unmarshal(b, &head); err != nil {
		return nil, err
	}
	return head.Symmetrix.Snapvx.Snapshots, nil
}

func (t *T) getSnap() (*XSnapshot, error) {
	snaps, err := t.List()
	if err != nil {
		return nil, err
	}
	name := t.formatName()
	for _, snap := range snaps {
		if snap.SnapshotName == name {
			return &snap, nil
		}
	}
	return nil, nil
}

func (t *T) Status(ctx context.Context) status.T {
	snap, err := t.getSnap()
	if err != nil {
		t.StatusLog().Error("%s", err)
		return status.Undef
	}
	if snap == nil {
		t.StatusLog().Info("no snapshot yet")
		return status.Down
	}
	latest, err := time.Parse(time.ANSIC, snap.LastTimestamp)
	if err != nil {
		t.StatusLog().Error("%s: %s", snap.SnapshotName, err)
		return status.Undef
	}
	t.StatusLog().Info("last snapshot on %s, %d generations", latest, snap.NumGenerations)
	if t.MaxDelay != nil && time.Since(latest) > *t.MaxDelay {
		t.StatusLog().Warn("last snapshot too old (<%s)", *t.MaxDelay)
		return status.Warn
	}
	return status.Up
}

// Label returns a formatted short description of the Resource
func (t T) Label() string {
	devsCount := len(t.Devices)
	devsFromCount := len(t.DevicesFrom)
	switch {
	case t.Name != "":
		return fmt.Sprintf("symsnapvx symid %s %s", t.SymID, t.Name)
	case devsCount > 0 && devsFromCount == 0:
		return fmt.Sprintf("symsnapvx symid %s devs %s", t.SymID, strings.Join(t.Devices, " "))
	case devsCount == 0 && devsFromCount > 0:
		return fmt.Sprintf("symsnapvx symid %s devs from %s", t.SymID, strings.Join(t.DevicesFrom, " "))
	case devsCount > 0 && devsFromCount > 0:
		return fmt.Sprintf("symsnapvx symid %s devs %s and from %s", t.SymID, strings.Join(t.Devices, " "), strings.Join(t.DevicesFrom, " "))
	default:
		return fmt.Sprintf("symsnapvx symid %s", t.SymID)
	}
}

func (t T) ScheduleOptions() resource.ScheduleOptions {
	return resource.ScheduleOptions{
		Action: "sync_update",
		Option: "schedule",
		Base:   "",
	}
}

func (t T) Provisioned() (provisioned.T, error) {
	return provisioned.NotApplicable, nil
}

func (t T) Info(ctx context.Context) (resource.InfoKeys, error) {
	mergedDevs, _ := t.mergeDevs()
	m := resource.InfoKeys{
		{Key: "devs", Value: strings.Join(t.Devices, " ")},
		{Key: "name", Value: t.Name},
		{Key: "symid", Value: t.SymID},
		{Key: "secure", Value: fmt.Sprintf("%v", t.Secure)},
		{Key: "max_delay", Value: fmt.Sprintf("%s", t.MaxDelay)},
		{Key: "schedule", Value: t.Schedule},
		{Key: "devids", Value: strings.Join(mergedDevs, ",")},
	}
	if t.Absolute != "" {
		m = append(m, resource.InfoKey{Key: "absolute", Value: t.Absolute})
	}
	if t.Delta != "" {
		m = append(m, resource.InfoKey{Key: "delta", Value: t.Delta})
	}
	snap, err := t.getSnap()
	if err == nil {
		m = append(m, resource.InfoKey{Key: "num_generations", Value: fmt.Sprint(snap.NumGenerations)})
		m = append(m, resource.InfoKey{Key: "last_timestamp", Value: snap.LastTimestamp})
	}
	return m, nil
}

func (t *T) formatName() string {
	if t.Name != "" {
		return t.Name
	}
	return fmt.Sprintf("%s.%s", t.ResourceID.Index(), t.ObjectFQDN)
}

func (t *T) establish() error {
	mergedDevs, err := t.mergeDevs()
	if err != nil {
		return err
	}
	if len(mergedDevs) == 0 {
		return fmt.Errorf("no devs")
	}

	args := []string{"establish", "-noprompt", "-sid", t.SymID, "-devs"}
	args = append(args, mergedDevs...)

	if t.Secure {
		args = append(args, "-secure")
	} else if t.Delta != "" || t.Absolute != "" {
		args = append(args, "-ttl")
	}
	if t.Delta != "" && t.Absolute != "" {
		return fmt.Errorf("set delta or absolute, not both")
	}
	if t.Delta != "" {
		args = append(args, "-delta", t.Delta)
	}
	if t.Absolute != "" {
		args = append(args, "-absolute", t.Absolute)
	}
	args = append(args, "-name", t.formatName())

	cmd := command.New(
		command.WithName(symsnapvx),
		command.WithArgs(args),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
		command.WithLogger(t.Log()),
	)

	return cmd.Run()
}

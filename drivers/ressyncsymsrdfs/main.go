package ressyncsymsrdfs

import (
	"bufio"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/drivers/arraysymmetrix"
	"github.com/opensvc/om3/drivers/ressync"
	"github.com/opensvc/om3/util/command"
	"github.com/opensvc/om3/util/device"
	"github.com/opensvc/om3/util/hostname"
	"github.com/opensvc/om3/util/xsession"
	"github.com/rs/zerolog"
)

// T is the driver structure.
type (
	T struct {
		ressync.T
		SymDG    string
		SymID    string
		RDFG     int
		Nodes    []string
		DRPNodes []string
	}

	/*
		<?xml version="1.0" standalone="yes" ?>
		<SymCLI_ML>
		  <Inquiry>
		    <Dev_Info>
		      <pd_name>/dev/sdb</pd_name>
		      <dev_name>000F1</dev_name>
		      <symid>000000000561</symid>
		      <dev_ident_name>TOOLS</dev_ident_name>
		    </Dev_Info>
		    <Product>
		      <vendor>EMC</vendor>
		    </Product>
		  </Inquiry>
		  <Inquiry>
		    <Dev_Info>
		      ...
	*/
	XInqIdentifierDeviceName struct {
		XMLName   xml.Name                          `xml:"SymCLI_ML"`
		Inquiries []XInqIdentifierDeviceNameInquiry `xml:"Inquiry"`
	}
	XInqIdentifierDeviceNameInquiry struct {
		XMLName xml.Name                               `xml:"Inquiry"`
		DevInfo XInqIdentifierDeviceNameInquiryDevInfo `xml:"Dev_Info"`
	}
	XInqIdentifierDeviceNameInquiryDevInfo struct {
		XMLName      xml.Name `xml:"Dev_Info"`
		SymID        string   `xml:"symid"`
		DevName      string   `xml:"dev_name"`
		DevIdentName string   `xml:"dev_ident_name"`
	}

	/*
		<SymCLI_ML>
		  <DG>
		    <DG_Info>
		      <name>pmax1</name>
		      <type>RDF1</type>
		      <symid>000000000193</symid>
		    </DG_Info>
		    <Device>
		      <Dev_Info>
			<dev_name>003AD</dev_name>
			<configuration>RDF1+TDEV</configuration>
			<ld_name>DEV001</ld_name>
			<status>Ready</status>
		      </Dev_Info>
		      <Front_End>
			<Port>
			  <pd_name>/dev/sdq</pd_name>
			  <director>07E</director>
			  <port>1</port>
			</Port>
		      </Front_End>
		    </Device>
		  </DG>
		</SymCLI_ML>
	*/
	XDGListLD struct {
		XMLName xml.Name    `xml:"SymCLI_ML"`
		DG      XDGListLDDG `xml:"DG"`
	}
	XDGListLDDG struct {
		XMLName xml.Name            `xml:"DG"`
		Devices []XDGListLDDGDevice `xml:"Device"`
	}
	XDGListLDDGDevice struct {
		XMLName  xml.Name                  `xml:"Device"`
		DevInfo  XDGListLDDGDeviceDevInfo  `xml:"Dev_Info"`
		FrontEnd XDGListLDDGDeviceFrontEnd `xml:"Front_End"`
	}
	XDGListLDDGDeviceDevInfo struct {
		XMLName       xml.Name `xml:"Dev_Info"`
		Status        string   `xml:"status"`
		Configuration string   `xml:"configuration"`
		LDName        string   `xml:"ld_name"`
		DevName       string   `xml:"dev_name"`
	}
	XDGListLDDGDeviceFrontEnd struct {
		XMLName  xml.Name `xml:"Front_End"`
		Director string   `xml:"director"`
		Port     int      `xml:"port"`
	}

	/*
		<?xml version="1.0" standalone="yes" ?>
		<SymCLI_ML>
		  <DG>
		    <DG_Info>
		      <name>TOOLS</name>
		      <type>RDF1</type>
		      <symid>000000000193</symid>
		      <microcode_version>5978</microcode_version>
		      <remote_symid>000000000197</remote_symid>
		      <remote_microcode_version>5978</remote_microcode_version>
		      <ra_group_num>1</ra_group_num>
		      <ra_group_num_hex>00</ra_group_num_hex>
		      <rdfa_session_num>0</rdfa_session_num>
		      <rdfa_cycle_num>0</rdfa_cycle_num>
		      <rdfa_session_active>False</rdfa_session_active>
		      <rdfa_consistency_exempt_devices>No</rdfa_consistency_exempt_devices>
		      <rdfa_wpace_exempt_devices>No</rdfa_wpace_exempt_devices>
		      <rdfa_avg_cycle_time>00:00:00</rdfa_avg_cycle_time>
		      <rdfa_avg_transmit_cycle_time>00:00:00</rdfa_avg_transmit_cycle_time>
		      <duration_of_last_cycle>00:00:00</duration_of_last_cycle>
		      <session_priority>33</session_priority>
		      <transmit_queue_depth_on_r1_side>0</transmit_queue_depth_on_r1_side>
		      <tracks_not_committed_to_r2_side>0</tracks_not_committed_to_r2_side>
		      <rdfa_tracks_not_committed_to_r2_side>0</rdfa_tracks_not_committed_to_r2_side>
		      <rdfa_time_r2_is_behind_r1>00:00:00</rdfa_time_r2_is_behind_r1>
		      <rdfa_r2_image_capture_time>N/A</rdfa_r2_image_capture_time>
		      <r2_data_is_consistent>N/A</r2_data_is_consistent>
		      <rdfa_min_cycle_time>00:00:15</rdfa_min_cycle_time>
		      <rdfa_session_priority>33</rdfa_session_priority>
		      <rdfa_r1_percent_cache_in_use>0</rdfa_r1_percent_cache_in_use>
		      <rdfa_r2_percent_cache_in_use>0</rdfa_r2_percent_cache_in_use>
		      <rdfa_r1_dse_used_trks>0</rdfa_r1_dse_used_trks>
		      <rdfa_r2_dse_used_trks>0</rdfa_r2_dse_used_trks>
		      <rdfa_transmit_idle_time>00:00:00</rdfa_transmit_idle_time>
		      <rdfa_r1_shared_trks>0</rdfa_r1_shared_trks>
		    </DG_Info>
		    <RDF_Pair>
		      <link_status>Ready</link_status>
		      <mode>Synchronous</mode>
		      <device_domino>Disabled</device_domino>
		      <adaptive_copy>Disabled</adaptive_copy>
		      <rdfa_consistency_state>N/A</rdfa_consistency_state>
		      <consistency_state>Disabled</consistency_state>
		      <consistency_exempt_state>Disabled</consistency_exempt_state>
		      <exempt_state>Disabled</exempt_state>
		      <pair_state>Synchronized</pair_state>
		      <r2_larger_than_r1>False</r2_larger_than_r1>
		      <r1_r2_device_size>Equals</r1_r2_device_size>
		      <Source>
			<ld_name>DEV001</ld_name>
			<dev_name>00321</dev_name>
			<state>Ready</state>
			<r1_invalids>0</r1_invalids>
			<r2_invalids>0</r2_invalids>
		      </Source>
		      <Target>
			<dev_name>00321</dev_name>
			<state>Write Disabled</state>
			<r1_invalids>0</r1_invalids>
			<r2_invalids>0</r2_invalids>
		      </Target>
		    </RDF_Pair>
		    <RDF_Pair_Totals>
		      <Source>
			<r1_invalids>0</r1_invalids>
			<r2_invalids>0</r2_invalids>
			<r1_invalid_mbs>0.0</r1_invalid_mbs>
			<r2_invalid_mbs>0.0</r2_invalid_mbs>
		      </Source>
		      <Target>
			<r1_invalids>0</r1_invalids>
			<r2_invalids>0</r2_invalids>
			<r1_invalid_mbs>0.0</r1_invalid_mbs>
			<r2_invalid_mbs>0.0</r2_invalid_mbs>
		      </Target>
		    </RDF_Pair_Totals>
		  </DG>
		</SymCLI_ML>
	*/
	XRDFQuery struct {
		XMLName xml.Name    `xml:"SymCLI_ML"`
		DG      XRDFQueryDG `xml:"DG"`
	}
	XRDFQueryDG struct {
		XMLName  xml.Name             `xml:"DG"`
		DGInfo   XRDFQueryDGInfo      `xml:"DG_Info"`
		RDFPairs []XRDFQueryDGRDFPair `xml:"RDF_Pair"`
	}
	XRDFQueryDGInfo struct {
		XMLName     xml.Name `xml:"DG_Info"`
		Name        string   `xml:"name"`
		Type        string   `xml:"type"`
		SymID       string   `xml:"symid"`
		RemoteSymID string   `xml:"remote_symid"`
	}
	XRDFQueryDGRDFPair struct {
		XMLName   xml.Name `xml:"RDF_Pair"`
		PairState string   `xml:"pair_state"`
		Mode      string   `xml:"mode"`
	}

	/*
		<?xml version="1.0" standalone="yes" ?>
		<SymCLI_ML>
		  <DG>
		    <DG_Info>
		      <name>TOOLS</name>
		      <type>RDF2</type>
		      <symid>000000000193</symid>
		      <valid>Yes</valid>
		      <std_devs>2</std_devs>
		      <gk_devs>0</gk_devs>
		      <all_bcvs>0</all_bcvs>
		      <all_vdevs>0</all_vdevs>
		      <all_tgts>0</all_tgts>
		      <in_cg>No</in_cg>
		      <num_sgs>0</num_sgs>
		    </DG_Info>
		  </DG>
		  <DG>
		    <DG_Info>
		      <name>ADM</name>
		      <type>RDF1</type>
		      <symid>000000000193</symid>
		      <valid>Yes</valid>
		      <std_devs>1</std_devs>
		      <gk_devs>0</gk_devs>
		      <all_bcvs>0</all_bcvs>
		      <all_vdevs>0</all_vdevs>
		      <all_tgts>0</all_tgts>
		      <in_cg>No</in_cg>
		      <num_sgs>0</num_sgs>
		    </DG_Info>
		  </DG>
		</SymCLI_ML>
	*/
	XDGList struct {
		XMLName xml.Name    `xml:"SymCLI_ML"`
		DGs     []XDGListDG `xml:"DG"`
	}
	XDGListDG struct {
		XMLName xml.Name        `xml:"DG"`
		DGInfo  XDGListDGDGInfo `xml:"DG_Info"`
	}
	XDGListDGDGInfo struct {
		XMLName xml.Name `xml:"DG_Info"`
		Name    string   `xml:"name"`
		Type    string   `xml:"type"`
		SymID   string   `xml:"symid"`
	}

	XDevList struct {
		XMLName   xml.Name          `xml:"SymCLI_ML" json:"-"`
		Symmetrix XDevListSymmetrix `xml:"Symmetrix" json:"Symmetrix"`
	}
	XDevListSymmetrix struct {
		XMLName  xml.Name                     `xml:"Symmetrix" json:"-"`
		Devices  []arraysymmetrix.Device      `xml:"Device" json:"Device"`
		SymmInfo arraysymmetrix.SymmInfoShort `xml:"Symm_Info" json:"Symm_Info"`
	}
)

const (
	symdev = "/usr/symcli/bin/symdev"
	symdg  = "/usr/symcli/bin/symdg"
	syminq = "/usr/symcli/bin/syminq"
	symrdf = "/usr/symcli/bin/symrdf"
)

func (t XRDFQueryDG) PairState() string {
	m := make(map[string]any)
	var state string
	for _, rdfPair := range t.RDFPairs {
		state := fmt.Sprintf("%s/%s", rdfPair.Mode, rdfPair.PairState)
		m[state] = nil
	}
	if len(m) == 1 {
		return state
	}
	return "mixed srdf pairs state"
}

func New() resource.Driver {
	return &T{}
}

func (t *T) listPD() ([]string, error) {
	l := make([]string, 0)
	m, err := t.getPDNameByDevNameMap()
	if err != nil {
		return l, err
	}
	args := []string{"-g", t.SymDG, "list", "ld", "-output", "xml_e", "-i", "15", "-c", "4"}
	cmd := exec.Command(symdg, args...)
	t.Log().Debugf("run %s", cmd)
	b, err := cmd.Output()
	if err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			if len(e.Stderr) != 0 {
				return nil, err
			}
		} else {
			return l, err
		}
	}
	var head XDGListLD
	if err := xml.Unmarshal(b, &head); err != nil {
		return l, err
	}
	for _, dev := range head.DG.Devices {
		name := strings.TrimLeft(dev.DevInfo.DevName, "0")
		if pds, ok := m[name]; ok {
			l = append(l, pds...)
		}
	}
	return l, err
}

func (t *T) getPDNameByDevNameMap() (map[string][]string, error) {
	m := make(map[string][]string)
	args := []string{"-identifier", "device_name", "-output", "xml_e"}
	cmd := exec.Command(syminq, args...)
	t.Log().Debugf("run %s", cmd)
	b, err := cmd.Output()
	if err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			if len(e.Stderr) != 0 {
				return nil, err
			}
		} else {
			return m, err
		}
	}
	var head XInqIdentifierDeviceName
	if err := xml.Unmarshal(b, &head); err != nil {
		return nil, err
	}
	for _, inq := range head.Inquiries {
		name := strings.TrimLeft(inq.DevInfo.DevName, "0")
		if l, ok := m[name]; ok {
			m[name] = append(l, inq.DevInfo.DevIdentName)
		} else {
			m[name] = []string{inq.DevInfo.DevIdentName}
		}
	}
	return m, nil
}

func (t *T) promoteRW() error {
	if runtime.GOOS != "linux" {
		return nil
	}
	devs, err := t.listPD()
	if err != nil {
		return err
	}
	for _, dev := range devs {
		if !strings.HasPrefix(dev, "/dev/mapper/") && !strings.HasPrefix(dev, "/dev/dm-") && !strings.HasPrefix(dev, "/dev/rdsk/") {
			continue
		}
		if err := t.promoteRWDev(dev); err != nil {
			return err
		}
	}
	return nil
}

func (t *T) promoteRWDev(dev string) error {
	d := device.New(dev, device.WithLogger(t.Log()))
	return d.PromoteRW()
}

func (t *T) getSymIDFromExportSafe(path string) string {
	if s, err := t.getSymIDFromExport(path); err != nil {
		return ""
	} else {
		return s
	}
}

func (t *T) getSymIDFromExport(filename string) (string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	var firstLine string
	if scanner.Scan() {
		firstLine = scanner.Text()
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}

	words := strings.Fields(firstLine)
	wordCount := len(words)
	if wordCount == 0 {
		return "", fmt.Errorf("unexpected content: %s", firstLine)
	}
	return words[wordCount-1], nil
}

func (t *T) Status(ctx context.Context) status.T {
	dg, err := t.dg()
	if err != nil {
		t.StatusLog().Warn("%s", err)
		return status.Warn
	}
	t.StatusLog().Info("current state %s", dg.PairState())
	if t.isSynchronousAndSynchronized() {
		return status.Up
	}
	t.StatusLog().Warn("expecting Synchronous/Synchronized")
	return status.Warn
}

func (t *T) Update(ctx context.Context) error {
	if err := t.updateRDFDGExport(); err != nil {
		return err
	}
	if err := t.updateLocalDGExport(); err != nil {
		return err
	}
	if err := t.updateWWNMap(); err != nil {
		return err
	}
	if err := t.postIngest(ctx); err != nil {
		return err
	}
	return nil
}

func (t *T) Ingest(ctx context.Context) error {
	switch {
	case t.SymID == t.getSymIDFromExportSafe(t.dgLocalFilename()):
		return t.dgImport(t.dgLocalFilename())
	case t.SymID == t.getSymIDFromExportSafe(t.dgRDFFilename()):
		return t.dgImport(t.dgRDFFilename())
	default:
		return nil
	}
}

func (t *T) updateRDFDGExport() error {
	filename := t.dgRDFFilename()
	if err := os.Remove(filename); errors.Is(err, os.ErrNotExist) {
		// pass
	} else if err != nil {
		return err
	}
	if err := t.dgExportRDF(filename); err != nil {
		return err
	}
	return t.sends(filename)
}

func (t *T) updateLocalDGExport() error {
	filename := t.dgLocalFilename()
	if err := os.Remove(filename); errors.Is(err, os.ErrNotExist) {
		// pass
	} else if err != nil {
		return err
	}
	if err := t.dgExport(filename); err != nil {
		return err
	}
	return t.sends(filename)
}

func (t *T) updateWWNMap() error {
	devs := make([]string, 0)
	file, err := os.Open(t.dgLocalFilename())
	if err != nil {
		return err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, "DEV") {
			continue
		}
		words := strings.Fields(line)
		if len(words) < 2 {
			continue
		}
		devs = append(devs, words[1])
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	l := make([][2]string, 0)

	b, err := t.devList(devs)
	if err != nil {
		return err
	}

	var data XDevList
	if err := xml.Unmarshal(b, &data); err != nil {
		return err
	}

	for _, device := range data.Symmetrix.Devices {
		if device.Product == nil {
			continue
		}
		if device.RDF == nil {
			continue
		}
		l = append(l, [2]string{device.Product.WWN, device.RDF.Remote.WWN})
	}

	// dump map in a json file
	filename := t.dgWWNMapFilename()
	file, err = os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	enc := json.NewEncoder(file)
	if err := enc.Encode(l); err != nil {
		return err
	}
	return t.sends(filename)
}

func (t *T) sends(filename string) error {
	head := t.GetObjectDriver().VarDir()
	c, err := client.New()
	if err != nil {
		return err
	}
	send := func(filename, nodename string) error {
		file, err := os.Open(filename)
		if err != nil {
			return err
		}
		defer file.Close()
		ctx := context.Background()
		response, err := c.PostInstanceStateFileWithBody(ctx, nodename, t.Path.Namespace, t.Path.Kind, t.Path.Name, "application/octet-stream", file, func(ctx context.Context, req *http.Request) error {
			req.Header.Add("x-relative-path", filename[len(head):])
			return nil
		})
		if err != nil {
			return err
		}
		if response.StatusCode != http.StatusNoContent {
			return fmt.Errorf("unexpected response: %s", response.Status)
		}
		return nil
	}
	var errs error

	for _, nodename := range append(t.Nodes, t.DRPNodes...) {
		if nodename == hostname.Hostname() {
			continue
		}
		if err := send(filename, nodename); err != nil {
			errs = errors.Join(errs, fmt.Errorf("failed to send state file %s to node %s: %w", filename, nodename, err))
		}
		t.Log().Infof("state file %s sent to node %s", filename, nodename)
	}
	return errs
}

func (t *T) postIngest(ctx context.Context) error {
	c, err := client.New()
	if err != nil {
		return err
	}
	var errs error
	for _, nodename := range append(t.Nodes, t.DRPNodes...) {
		if nodename == hostname.Hostname() {
			continue
		}
		rid := t.RID()
		sid := xsession.ID
		params := api.PostInstanceActionSyncIngestParams{
			Rid:          &rid,
			RequesterSid: &sid,
		}
		resp, err := c.PostInstanceActionSyncIngestWithResponse(ctx, nodename, t.Path.Namespace, t.Path.Kind, t.Path.Name, &params)
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("%s: %w", nodename, err))
			continue
		}
		switch resp.StatusCode() {
		case 200:
			t.Log().Infof("%s: state files ingested", nodename)
		case 400:
			errs = errors.Join(errs, fmt.Errorf("%s: %w", nodename, resp.JSON400))
		case 401:
			errs = errors.Join(errs, fmt.Errorf("%s: %w", nodename, resp.JSON401))
		case 403:
			errs = errors.Join(errs, fmt.Errorf("%s: %w", nodename, resp.JSON403))
		case 500:
			errs = errors.Join(errs, fmt.Errorf("%s: %w", nodename, resp.JSON500))
		}
	}
	return errs
}

// Label returns a formatted short description of the Resource
func (t *T) Label() string {
	return fmt.Sprintf("srdf/s symid:%s dg:%s rdfg:%d", t.SymDG, t.SymID, t.RDFG)
}

func (t *T) Info(ctx context.Context) (resource.InfoKeys, error) {
	m := resource.InfoKeys{
		{Key: "symid", Value: t.SymID},
		{Key: "symdg", Value: t.SymDG},
	}
	return m, nil
}

func (t *T) resume() error {
	args := []string{"-g", t.SymDG, "-rdfg", fmt.Sprint(t.RDFG), "-noprompt", "resume", "-i", "15", "-c", "4"}
	cmd := command.New(
		command.WithName(symrdf),
		command.WithArgs(args),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
		command.WithLogger(t.Log()),
	)
	return cmd.Run()
}

func (t *T) suspend() error {
	args := []string{"-g", t.SymDG, "-rdfg", fmt.Sprint(t.RDFG), "-noprompt", "suspend", "-i", "15", "-c", "4"}
	cmd := command.New(
		command.WithName(symrdf),
		command.WithArgs(args),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
		command.WithLogger(t.Log()),
	)
	return cmd.Run()
}

func (t *T) establish() error {
	args := []string{"-g", t.SymDG, "-rdfg", fmt.Sprint(t.RDFG), "-noprompt", "establish", "-i", "15", "-c", "4"}
	cmd := command.New(
		command.WithName(symrdf),
		command.WithArgs(args),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
		command.WithLogger(t.Log()),
	)
	return cmd.Run()
}

func (t *T) failover() error {
	args := []string{"-g", t.SymDG, "-rdfg", fmt.Sprint(t.RDFG), "-noprompt", "failover", "-i", "15", "-c", "4"}
	cmd := command.New(
		command.WithName(symrdf),
		command.WithArgs(args),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
		command.WithLogger(t.Log()),
	)
	return cmd.Run()
}

func (t *T) failoverEstablish() error {
	args := []string{"-g", t.SymDG, "-rdfg", fmt.Sprint(t.RDFG), "-noprompt", "failover", "-establish", "-i", "15", "-c", "4"}
	cmd := command.New(
		command.WithName(symrdf),
		command.WithArgs(args),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
		command.WithLogger(t.Log()),
	)
	return cmd.Run()
}

func (t *T) split() error {
	args := []string{"-g", t.SymDG, "-rdfg", fmt.Sprint(t.RDFG), "-noprompt", "split", "-i", "15", "-c", "4"}
	cmd := command.New(
		command.WithName(symrdf),
		command.WithArgs(args),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
		command.WithLogger(t.Log()),
	)
	return cmd.Run()
}

func (t *T) swap() error {
	args := []string{"-g", t.SymDG, "-rdfg", fmt.Sprint(t.RDFG), "-noprompt", "swap", "-i", "15", "-c", "4"}
	cmd := command.New(
		command.WithName(symrdf),
		command.WithArgs(args),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
		command.WithLogger(t.Log()),
	)
	return cmd.Run()
}

func (t *T) isSynchronous() bool {
	args := []string{"-g", t.SymDG, "-rdfg", fmt.Sprint(t.RDFG), "verify", "-synchronous", "-i", "15", "-c", "4"}
	cmd := command.New(
		command.WithName(symrdf),
		command.WithArgs(args),
		command.WithCommandLogLevel(zerolog.DebugLevel),
		command.WithStdoutLogLevel(zerolog.DebugLevel),
		command.WithStderrLogLevel(zerolog.DebugLevel),
		command.WithLogger(t.Log()),
	)
	_ = cmd.Run()
	return cmd.ExitCode() == 0
}

func (t *T) isAsynchronous() bool {
	args := []string{"-g", t.SymDG, "-rdfg", fmt.Sprint(t.RDFG), "verify", "-asynchronous", "-i", "15", "-c", "4"}
	cmd := command.New(
		command.WithName(symrdf),
		command.WithArgs(args),
		command.WithCommandLogLevel(zerolog.DebugLevel),
		command.WithStdoutLogLevel(zerolog.DebugLevel),
		command.WithStderrLogLevel(zerolog.DebugLevel),
		command.WithLogger(t.Log()),
	)
	_ = cmd.Run()
	return cmd.ExitCode() == 0
}

func (t *T) isACPDisk() bool {
	args := []string{"-g", t.SymDG, "-rdfg", fmt.Sprint(t.RDFG), "verify", "-acp_disk", "-synchronized", "-i", "15", "-c", "4"}
	cmd := command.New(
		command.WithName(symrdf),
		command.WithArgs(args),
		command.WithCommandLogLevel(zerolog.DebugLevel),
		command.WithStdoutLogLevel(zerolog.DebugLevel),
		command.WithStderrLogLevel(zerolog.DebugLevel),
		command.WithLogger(t.Log()),
	)
	_ = cmd.Run()
	return cmd.ExitCode() == 0
}

func (t *T) isSynchronized() bool {
	args := []string{"-g", t.SymDG, "-rdfg", fmt.Sprint(t.RDFG), "verify", "-synchronized", "-i", "15", "-c", "4"}
	cmd := command.New(
		command.WithName(symrdf),
		command.WithArgs(args),
		command.WithCommandLogLevel(zerolog.DebugLevel),
		command.WithStdoutLogLevel(zerolog.DebugLevel),
		command.WithStderrLogLevel(zerolog.DebugLevel),
		command.WithLogger(t.Log()),
	)
	_ = cmd.Run()
	return cmd.ExitCode() == 0
}

func (t *T) isSynchronousAndSynchronized() bool {
	args := []string{"-g", t.SymDG, "-rdfg", fmt.Sprint(t.RDFG), "verify", "-synchronous", "-synchronized", "-i", "15", "-c", "4"}
	cmd := command.New(
		command.WithName(symrdf),
		command.WithArgs(args),
		command.WithCommandLogLevel(zerolog.DebugLevel),
		command.WithStdoutLogLevel(zerolog.DebugLevel),
		command.WithStderrLogLevel(zerolog.DebugLevel),
		command.WithLogger(t.Log()),
	)
	_ = cmd.Run()
	return cmd.ExitCode() == 0
}

func (t *T) isSyncInProg() bool {
	args := []string{"-g", t.SymDG, "-rdfg", fmt.Sprint(t.RDFG), "verify", "-syncinprog", "-i", "15", "-c", "4"}
	cmd := command.New(
		command.WithName(symrdf),
		command.WithArgs(args),
		command.WithCommandLogLevel(zerolog.DebugLevel),
		command.WithStdoutLogLevel(zerolog.DebugLevel),
		command.WithStderrLogLevel(zerolog.DebugLevel),
		command.WithLogger(t.Log()),
	)
	_ = cmd.Run()
	return cmd.ExitCode() == 0
}

func (t *T) isSuspended() bool {
	args := []string{"-g", t.SymDG, "-rdfg", fmt.Sprint(t.RDFG), "verify", "-suspended", "-i", "15", "-c", "4"}
	cmd := command.New(
		command.WithName(symrdf),
		command.WithArgs(args),
		command.WithCommandLogLevel(zerolog.DebugLevel),
		command.WithStdoutLogLevel(zerolog.DebugLevel),
		command.WithStderrLogLevel(zerolog.DebugLevel),
		command.WithLogger(t.Log()),
	)
	_ = cmd.Run()
	return cmd.ExitCode() == 0
}

func (t *T) isSplit() bool {
	args := []string{"-g", t.SymDG, "-rdfg", fmt.Sprint(t.RDFG), "verify", "-split", "-i", "15", "-c", "4"}
	cmd := command.New(
		command.WithName(symrdf),
		command.WithArgs(args),
		command.WithCommandLogLevel(zerolog.DebugLevel),
		command.WithStdoutLogLevel(zerolog.DebugLevel),
		command.WithStderrLogLevel(zerolog.DebugLevel),
		command.WithLogger(t.Log()),
	)
	_ = cmd.Run()
	return cmd.ExitCode() == 0
}

func (t *T) isFailedOver() bool {
	args := []string{"-g", t.SymDG, "-rdfg", fmt.Sprint(t.RDFG), "verify", "-failedover", "-i", "15", "-c", "4"}
	cmd := command.New(
		command.WithName(symrdf),
		command.WithArgs(args),
		command.WithCommandLogLevel(zerolog.DebugLevel),
		command.WithStdoutLogLevel(zerolog.DebugLevel),
		command.WithStderrLogLevel(zerolog.DebugLevel),
		command.WithLogger(t.Log()),
	)
	_ = cmd.Run()
	return cmd.ExitCode() == 0
}

func (t *T) isPartitioned() bool {
	args := []string{"-g", t.SymDG, "-rdfg", fmt.Sprint(t.RDFG), "verify", "-partitioned", "-i", "15", "-c", "4"}
	cmd := command.New(
		command.WithName(symrdf),
		command.WithArgs(args),
		command.WithCommandLogLevel(zerolog.DebugLevel),
		command.WithStdoutLogLevel(zerolog.DebugLevel),
		command.WithStderrLogLevel(zerolog.DebugLevel),
		command.WithLogger(t.Log()),
	)
	_ = cmd.Run()
	return cmd.ExitCode() == 0
}

func (t *T) isConsistent() bool {
	args := []string{"-g", t.SymDG, "-rdfg", fmt.Sprint(t.RDFG), "verify", "-consistent", "-i", "15", "-c", "4"}
	cmd := command.New(
		command.WithName(symrdf),
		command.WithArgs(args),
		command.WithCommandLogLevel(zerolog.DebugLevel),
		command.WithStdoutLogLevel(zerolog.DebugLevel),
		command.WithStderrLogLevel(zerolog.DebugLevel),
		command.WithLogger(t.Log()),
	)
	_ = cmd.Run()
	return cmd.ExitCode() == 0
}

func (t *T) isEnabled() bool {
	args := []string{"-g", t.SymDG, "-rdfg", fmt.Sprint(t.RDFG), "verify", "-enabled", "-i", "15", "-c", "4"}
	cmd := command.New(
		command.WithName(symrdf),
		command.WithArgs(args),
		command.WithCommandLogLevel(zerolog.DebugLevel),
		command.WithStdoutLogLevel(zerolog.DebugLevel),
		command.WithStderrLogLevel(zerolog.DebugLevel),
		command.WithLogger(t.Log()),
	)
	_ = cmd.Run()
	return cmd.ExitCode() == 0
}

// Split stops replication and makes both R1 and R2 writeable
func (t *T) Split(ctx context.Context) error {
	return t.split()
}

// Pause stops the replication and leaves R2 read-only
func (t *T) Pause(ctx context.Context) error {
	return t.suspend()
}

// Pause starts again the replication
func (t *T) Resume(ctx context.Context) error {
	return t.establish()
}

func (t *T) Start(ctx context.Context) error {
	localhost := hostname.Hostname()
	dg, err := t.dg()
	if err != nil {
		return err
	}
	switch {
	case slices.Contains(t.DRPNodes, localhost):
		switch dg.DGInfo.Type {
		case "RDF2":
			switch {
			case t.isSynchronousAndSynchronized():
				err = t.split()
			case t.isPartitioned():
				t.Log().Warnf("symrdf dg %s is RDF2 and partitioned. failover is preferred action.", t.SymDG)
				err = t.failover()
			case t.isFailedOver():
				t.Log().Infof("symrdf dg %s is already RDF2 and FailedOver.", t.SymDG)
			case t.isSuspended():
				t.Log().Warnf("symrdf dg %s is RDF2 and suspended: R2 data may be outdated.", t.SymDG)
				err = t.split()
			case t.isSplit():
				t.Log().Infof("symrdf dg %s is RDF2 and already split.", t.SymDG)
			default:
				return fmt.Errorf("symrdf dg %s is RDF2 on drp node and unexpected SRDF state, you have to manually return to a sane SRDF status.", t.SymDG)
			}
		case "RDF1":
			switch {
			case t.isSynchronousAndSynchronized():
				// pass
			default:
				return fmt.Errorf("symrdf dg %s is RDF1 on drp node, you have to manually return to a sane SRDF status.", t.SymDG)
			}
		}
	case slices.Contains(t.Nodes, localhost):
		switch dg.DGInfo.Type {
		case "RDF1":
			switch {
			case t.isSynchronousAndSynchronized():
				t.Log().Infof("symrdf dg %s is RDF1 and synchronous/synchronized.", t.SymDG)
			case t.isPartitioned():
				t.Log().Warnf("symrdf dg %s is RDF1 and partitioned.", t.SymDG)
			case t.isFailedOver():
				return fmt.Errorf("symrdf dg %s is RDF1 and write protected, you have to manually run either sync_split+sync_establish (ie losing R2 data), or syncfailback (ie losing R1 data)", t.SymDG)
			case t.isSuspended():
				t.Log().Warnf("symrdf dg %s is RDF1 and suspended.", t.SymDG)
			case t.isSplit():
				t.Log().Warnf("symrdf dg %s is RDF1 and split.", t.SymDG)
			default:
				return fmt.Errorf("symrdf dg %s is RDF1 on primary node and unexpected SRDF state, you have to manually return to a sane SRDF status.", t.SymDG)
			}
		case "RDF2":
			switch {
			case t.isSynchronousAndSynchronized():
				err = t.failoverEstablish()
			case t.isPartitioned():
				t.Log().Warnf("symrdf dg %s is RDF2 and partitioned, failover is preferred action.", t.SymDG)
				t.failover()
			default:
				return fmt.Errorf("symrdf dg %s is RDF2 on primary node, you have to manually return to a sane SRDF status.", t.SymDG)
			}
		}
	}
	if err != nil {
		return err
	}
	return t.promoteRW()
}

func (t *T) dgImport(path string) error {
	args := []string{"import", t.SymDG, "-f", path, "-i", "15", "-c", "4"}
	cmd := command.New(
		command.WithName(symdg),
		command.WithArgs(args),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
		command.WithLogger(t.Log()),
	)
	return cmd.Run()
}

func (t *T) dgExportRDF(path string) error {
	args := []string{"export", t.SymDG, "-f", path, "-rdf", "-i", "15", "-c", "4"}
	cmd := command.New(
		command.WithName(symdg),
		command.WithArgs(args),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
		command.WithLogger(t.Log()),
	)
	return cmd.Run()
}

func (t *T) dgExport(path string) error {
	args := []string{"export", t.SymDG, "-f", path, "-i", "15", "-c", "4"}
	cmd := command.New(
		command.WithName(symdg),
		command.WithArgs(args),
		command.WithCommandLogLevel(zerolog.InfoLevel),
		command.WithStdoutLogLevel(zerolog.InfoLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
		command.WithLogger(t.Log()),
	)
	return cmd.Run()
}

func (t *T) devList(devs []string) ([]byte, error) {
	args := []string{"list", "-devs", strings.Join(devs, ","), "-sid", t.SymID, "-v", "-output", "xml_e"}
	cmd := command.New(
		command.WithName(symdev),
		command.WithArgs(args),
		command.WithCommandLogLevel(zerolog.DebugLevel),
		command.WithStdoutLogLevel(zerolog.DebugLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
		command.WithBufferedStdout(),
		command.WithLogger(t.Log()),
	)
	err := cmd.Run()
	return cmd.Stdout(), err
}

func (t *T) dgQuery() ([]byte, error) {
	args := []string{"list", "-i", "15", "-c", "4", "-output", "xml_e"}
	cmd := command.New(
		command.WithName(symdg),
		command.WithArgs(args),
		command.WithCommandLogLevel(zerolog.DebugLevel),
		command.WithStdoutLogLevel(zerolog.DebugLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
		command.WithBufferedStdout(),
		command.WithLogger(t.Log()),
	)
	err := cmd.Run()
	return cmd.Stdout(), err
}

func (t *T) rdfQuery() ([]byte, error) {
	args := []string{"-g", t.SymDG, "-rdfg", fmt.Sprint(t.RDFG), "query", "-output", "xml_e"}
	cmd := command.New(
		command.WithName(symrdf),
		command.WithArgs(args),
		command.WithCommandLogLevel(zerolog.DebugLevel),
		command.WithStdoutLogLevel(zerolog.DebugLevel),
		command.WithStderrLogLevel(zerolog.ErrorLevel),
		command.WithBufferedStdout(),
		command.WithLogger(t.Log()),
	)
	err := cmd.Run()
	return cmd.Stdout(), err
}

func (t *T) dg() (XRDFQueryDG, error) {
	var data XRDFQuery
	b, err := t.rdfQuery()
	if err != nil {
		return XRDFQueryDG{}, err
	}
	if err := xml.Unmarshal(b, &data); err != nil {
		return XRDFQueryDG{}, err
	}
	return data.DG, nil
}

func (t *T) dgList() ([]string, error) {
	var data XDGList
	l := make([]string, 0)
	b, err := t.dgQuery()
	if err != nil {
		return l, err
	}
	if err := xml.Unmarshal(b, &data); err != nil {
		return l, err
	}
	for _, dg := range data.DGs {
		l = append(l, dg.DGInfo.Name)
	}
	return l, nil
}

func (t *T) dgTmpLocalFilename() string {
	return filepath.Join(t.VarDir(), fmt.Sprintf("symrdf_%s.dg.tmp.local", t.SymDG))
}

func (t *T) dgLocalFilename() string {
	return filepath.Join(t.VarDir(), fmt.Sprintf("symrdf_%s.dg.local", t.SymDG))
}

func (t *T) dgRDFFilename() string {
	return filepath.Join(t.VarDir(), fmt.Sprintf("symrdf_%s.dg.rdf", t.SymDG))
}

func (t *T) dgWWNMapFilename() string {
	return filepath.Join(t.VarDir(), "wwn_map")
}

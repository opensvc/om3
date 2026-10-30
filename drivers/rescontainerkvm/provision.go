package rescontainerkvm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"syscall"

	"github.com/opensvc/om3/v3/core/datarecv"
	"github.com/opensvc/om3/v3/core/vpath"
	"github.com/opensvc/om3/v3/util/uri"
)

const (
	virtInstall = "virt-install"

	// imageDir holds an image downloaded, for nobody to read and as large as
	// a disk: not a place of root, nor a tmpfs.
	imageDir = "/var/tmp"
)

type (
	// vmSpec is what the keywords of a guest ask of virt-install.
	//
	// The keywords describe a guest, and virt-install and libvirt make it
	// on the qemu of the node: om knows none of the qemu options, and the
	// ones they do not cover are for virtinst_args.
	vmSpec struct {
		name     string
		vcpus    int
		memoryMB int64
		osFlag   string
		osValue  string
		disks    []string
		bridges  []string
		nics     []guestNIC
		userData string
		netData  string
		extra    []string
	}
)

// args returns the arguments of the virt-install command. The domain is
// made and started, the install of a cloud image being its first boot.
func (t vmSpec) args() []string {
	l := []string{
		virtInstall,
		"--name", t.name,
		"--memory", strconv.FormatInt(t.memoryMB, 10),
		"--vcpus", strconv.Itoa(t.vcpus),
		t.osFlag, t.osValue,
		"--import",
	}
	for _, disk := range t.disks {
		l = append(l, "--disk", disk+",format=raw,cache=none")
	}
	for _, bridge := range t.bridges {
		l = append(l, "--network", "bridge="+bridge+",model=virtio")
	}
	for i, n := range t.nics {
		// The tap is om's, filtered before the guest runs: libvirt is to use
		// it rather than make one, which virt-install has no option to say.
		l = append(l,
			"--network", fmt.Sprintf("type=ethernet,target.dev=%s,mac=%s,model=virtio", n.dev, n.mac),
			"--xml", fmt.Sprintf("./devices/interface[%d]/target/@managed=no", len(t.bridges)+i+1),
		)
	}
	if t.userData != "" {
		seed := "user-data=" + t.userData
		if t.netData != "" {
			seed += ",network-config=" + t.netData
		}
		l = append(l, "--cloud-init", seed)
	}
	l = append(l, "--noautoconsole")
	return append(l, t.extra...)
}

// hasSpec says the keywords describe the guest to make, rather than virtinst
// saying how to make it, or nothing, a domain defined by a sysadmin.
func (t *T) hasSpec() bool {
	return t.VCPUs > 0 || t.Memory != nil || len(t.Disk) > 0 || len(t.Bridge) > 0 || len(t.Network) > 0 || t.OSInfo != "" ||
		t.Image != "" || t.CloudInit != "" || len(t.VirtInstArgs) > 0
}

// specError says what the keywords describing the guest lack. It is the
// provision's alone: these keywords are not read once the domain is defined,
// so they tell nothing of the state of a guest, as a ConfigError would.
func (t *T) specError() error {
	if !t.hasSpec() {
		return nil
	}
	if len(t.VirtInst) > 0 {
		return errors.New("virtinst says all of the command: it excludes vcpus, memory, disk, bridge, network, osinfo, image, cloud_init and virtinst_args")
	}
	missing := make([]string, 0)
	if t.VCPUs <= 0 {
		missing = append(missing, "vcpus")
	}
	if t.Memory == nil || *t.Memory < 1024*1024 {
		missing = append(missing, "memory")
	}
	if len(t.Disk) == 0 {
		missing = append(missing, "disk")
	}
	if len(missing) > 0 {
		return fmt.Errorf("a guest needs %s", strings.Join(missing, ", "))
	}
	return t.specNetworkError()
}

// virtInstHelp returns what the virt-install of the node says of its options:
// which it has says the version better than its number, the distributions
// patching it.
func virtInstHelp(ctx context.Context, name string) ([]byte, error) {
	b, err := exec.CommandContext(ctx, name, "--help").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s --help: %w", name, err)
	}
	return b, nil
}

// osOption returns the option naming the os of the guest, and its value.
// --osinfo is the name of --os-variant since virt-install 4.
func osOption(help []byte, osinfo string) (string, string) {
	if bytes.Contains(help, []byte("--osinfo")) {
		if osinfo == "" {
			return "--osinfo", "detect=on,require=off"
		}
		return "--osinfo", osinfo
	}
	if osinfo == "" {
		return "--os-variant", "generic"
	}
	return "--os-variant", osinfo
}

// specCommand returns the virt-install command of the keywords, with what
// removes the files it was given once it ended.
func (t *T) specCommand(ctx context.Context) ([]string, func(), error) {
	cleanup := func() {}
	if err := t.specError(); err != nil {
		return nil, cleanup, err
	}
	help, err := virtInstHelp(ctx, virtInstall)
	if err != nil {
		return nil, cleanup, err
	}
	disks, err := vpath.DevpathsOf(ctx, t.RID(), t.Disk, t.Path.Namespace, vpath.ResolverOf(t.GetObject()))
	if err != nil {
		return nil, cleanup, err
	}
	if len(disks) == 0 {
		return nil, cleanup, fmt.Errorf("disk: no device")
	}
	spec := vmSpec{
		name:     t.Name,
		vcpus:    t.VCPUs,
		memoryMB: *t.Memory / (1024 * 1024),
		disks:    disks,
		bridges:  t.Bridge,
		extra:    t.VirtInstArgs,
	}
	spec.osFlag, spec.osValue = osOption(help, t.OSInfo)
	if t.CloudInit != "" || (t.Image != "" && len(t.Network) > 0) {
		if !bytes.Contains(help, []byte("--cloud-init")) {
			return nil, cleanup, fmt.Errorf("the %s of this node has no --cloud-init option", virtInstall)
		}
		p, err := t.userDataFile()
		if err != nil {
			return nil, cleanup, err
		}
		cleanup = func() { _ = os.Remove(p) }
		spec.userData = p
	}
	spec.nics, err = t.networkUp(ctx)
	if err != nil {
		return nil, cleanup, err
	}
	if spec.userData != "" && len(spec.nics) > 0 {
		p, err := writeSeedFile(t.Name, "network-config", []byte(networkConfig(spec.nics)))
		if err != nil {
			return nil, cleanup, err
		}
		removeUserData := cleanup
		cleanup = func() { removeUserData(); _ = os.Remove(p) }
		spec.netData = p
	}
	// Last, for what can refuse the provision not to cost a disk written.
	if t.Image != "" {
		if err := t.writeImage(ctx, disks[0]); err != nil {
			return nil, cleanup, err
		}
	}
	return spec.args(), cleanup, nil
}

// userDataFile writes the key the cloud_init keyword names, or nothing to do
// for a guest needing cloud-init for its network only, and returns its path.
func (t *T) userDataFile() (string, error) {
	if t.CloudInit == "" {
		return writeSeedFile(t.Name, "user-data", []byte("#cloud-config\n"))
	}
	km, err := datarecv.ParseKeyMetaRelObj(t.CloudInit, t.GetObject())
	if err != nil {
		return "", fmt.Errorf("cloud_init: %w", err)
	}
	b, err := km.Decode()
	if err != nil {
		return "", fmt.Errorf("cloud_init: %w", err)
	}
	return writeSeedFile(t.Name, "user-data", b)
}

// writeSeedFile writes a file only root reads, as it may hold secrets, and
// returns its path.
func writeSeedFile(name, kind string, b []byte) (string, error) {
	f, err := os.CreateTemp("", name+".*."+kind)
	if err != nil {
		return "", err
	}
	_, err = f.Write(b)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

// isURL says the image keyword names something to download.
func isURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

// writeImage writes the image keyword to the device, as raw. The device may
// be larger than the image, what the guest grows into at its first boot.
//
// A downloaded image is anybody's, and what parses it is no place for root:
// qemu-img runs as nobody then, handed the device for the time of the write.
func (t *T) writeImage(ctx context.Context, dev string) error {
	src := t.Image
	var cred *syscall.Credential
	if isURL(src) {
		var err error
		if cred, err = nobody(); err != nil {
			return fmt.Errorf("image: %w", err)
		}
		limit, err := deviceSize(dev)
		if err != nil {
			return fmt.Errorf("image: %w", err)
		}
		p, err := downloadImage(ctx, src, imageDir, limit)
		if err != nil {
			return fmt.Errorf("image: %w", err)
		}
		defer func() { _ = os.Remove(p) }()
		if err := os.Chown(p, int(cred.Uid), int(cred.Gid)); err != nil {
			return fmt.Errorf("image: %w", err)
		}
		restore, err := lendDevice(dev, cred)
		if err != nil {
			return fmt.Errorf("image: %w", err)
		}
		defer restore()
		src = p
	}
	format, err := imageFormat(ctx, src, cred)
	if err != nil {
		return fmt.Errorf("image: %w", err)
	}
	t.Log().Infof("write the %s image to %s", format, dev)
	if _, err := qemuImg(ctx, cred, "convert", "-n", "-f", format, "-O", "raw", src, dev); err != nil {
		return fmt.Errorf("image: %w", err)
	}
	return nil
}

// nobody returns the credentials what parses a downloaded image runs with.
func nobody() (*syscall.Credential, error) {
	u, err := user.Lookup("nobody")
	if err != nil {
		return nil, err
	}
	uid, err := strconv.ParseUint(u.Uid, 10, 32)
	if err != nil {
		return nil, err
	}
	gid, err := strconv.ParseUint(u.Gid, 10, 32)
	if err != nil {
		return nil, err
	}
	return &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)}, nil
}

// lendDevice gives the device to the credentials, and returns what takes it
// back.
func lendDevice(dev string, cred *syscall.Credential) (func(), error) {
	var st syscall.Stat_t
	if err := syscall.Stat(dev, &st); err != nil {
		return nil, fmt.Errorf("stat %s: %w", dev, err)
	}
	if err := os.Chown(dev, int(cred.Uid), int(cred.Gid)); err != nil {
		return nil, err
	}
	return func() { _ = os.Chown(dev, int(st.Uid), int(st.Gid)) }, nil
}

// deviceSize returns the size of a block device or of a file.
func deviceSize(dev string) (int64, error) {
	f, err := os.Open(dev)
	if err != nil {
		return 0, err
	}
	defer func() { _ = f.Close() }()
	return f.Seek(0, io.SeekEnd)
}

// qemuImg runs qemu-img, as the credentials when given, and returns its output.
func qemuImg(ctx context.Context, cred *syscall.Credential, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "qemu-img", args...)
	cmd.Dir = "/"
	if cred != nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{Credential: cred}
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	b, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("qemu-img %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return b, nil
}

// imageFormat returns the format of the image.
func imageFormat(ctx context.Context, src string, cred *syscall.Credential) (string, error) {
	b, err := qemuImg(ctx, cred, "info", "--output=json", src)
	if err != nil {
		return "", err
	}
	return parseImageInfo(b)
}

// parseImageInfo returns the format of the image qemu-img describes, refusing
// what makes it read something else: run as root, the convert of a qcow2 with
// a backing file, which a downloaded image names as it wishes, would copy that
// file of the node to the disk of the guest.
func parseImageInfo(b []byte) (string, error) {
	var info struct {
		Format          string `json:"format"`
		BackingFilename string `json:"backing-filename"`
		FormatSpecific  struct {
			Data struct {
				DataFile string `json:"data-file"`
			} `json:"data"`
		} `json:"format-specific"`
	}
	if err := json.Unmarshal(b, &info); err != nil {
		return "", err
	}
	switch {
	case info.Format != "qcow2" && info.Format != "raw":
		return "", fmt.Errorf("%s: a qcow2 or raw image is expected", info.Format)
	case info.BackingFilename != "":
		return "", fmt.Errorf("the image has a backing file %s", info.BackingFilename)
	case info.FormatSpecific.Data.DataFile != "":
		return "", fmt.Errorf("the image has a data file %s", info.FormatSpecific.Data.DataFile)
	}
	return info.Format, nil
}

// downloadImage downloads the url to a file of the directory, the policy of
// the node on the urls it may fetch applying, and returns its path. An image
// over limit fits no disk, and is refused as it arrives rather than fill the
// directory of the node.
func downloadImage(ctx context.Context, url, dir string, limit int64) (string, error) {
	client, err := uri.SafeHttpClient(url)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s: %s", url, resp.Status)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(dir, "image.*")
	if err != nil {
		return "", err
	}
	n, err := io.Copy(f, io.LimitReader(resp.Body, limit+1))
	if err == nil && n > limit {
		err = fmt.Errorf("%s: larger than the %d bytes of the disk", url, limit)
	}
	if err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

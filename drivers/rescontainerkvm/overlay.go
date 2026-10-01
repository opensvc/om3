package rescontainerkvm

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/antchfx/xmlquery"
)

// overlayDir is where the overlays of a move are written, on both nodes. It
// is the directory libvirt keeps its images in, which its security drivers
// let qemu open, and it is outside the datasets a move sends.
const overlayDir = "/var/lib/libvirt/images"

type (
	// moveOverlay is the qcow2 a copied disk of the guest writes to while
	// a move runs, over the zvol or the file that holds the disk.
	//
	// Under it the disk no longer changes, so the snapshot a move sends of
	// it is the disk as the guest left it, and what the migration has left
	// to copy is what the guest writes from then on: the overlay.
	moveOverlay struct {
		Target string
		Base   string
		Format string
		Path   string
	}
)

// newMoveOverlays returns the overlay of each copied disk.
func newMoveOverlays(domain string, disks []moveDisk) []moveOverlay {
	l := make([]moveOverlay, 0, len(disks))
	for _, d := range disks {
		format := d.Format
		if format == "" {
			format = "raw"
		}
		l = append(l, moveOverlay{
			Target: d.Target,
			Base:   d.Source,
			Format: format,
			Path:   filepath.Join(overlayDir, domain+"."+d.Target+".osvc-move.qcow2"),
		})
	}
	return l
}

// snapshotArgs returns the virsh arguments putting the overlays over the
// copied disks of the running domain, and leaving its other disks as they
// are: a disk the snapshot does not name would be snapshotted the way its
// definition says.
func snapshotArgs(domain string, overlays []moveOverlay, targets []string) []string {
	args := []string{"snapshot-create-as", domain, "--name", "osvc-move", "--disk-only", "--no-metadata", "--atomic"}
	overlaid := make(map[string]bool, len(overlays))
	for _, o := range overlays {
		overlaid[o.Target] = true
		args = append(args, "--diskspec", o.Target+",snapshot=external,file="+o.Path)
	}
	for _, target := range targets {
		if !overlaid[target] {
			args = append(args, "--diskspec", target+",snapshot=no")
		}
	}
	return args
}

// overlayCreateCmdline returns the command creating, on the destination, the
// overlay the migration copies the one of the source into, over the copy of
// the disk the move sent there. Its size is the one of the disk.
func overlayCreateCmdline(o moveOverlay) string {
	return strings.Join([]string{
		"qemu-img", "create", "-q", "-f", "qcow2",
		"-b", shellQuote(o.Base), "-F", shellQuote(o.Format),
		shellQuote(o.Path),
	}, " ")
}

// blockcommitArgs returns the virsh arguments merging the overlay of a disk
// into the disk under it, and putting the guest back on that disk.
func blockcommitArgs(domain, target string) []string {
	return []string{"blockcommit", domain, target, "--active", "--pivot", "--wait"}
}

// parseDiskTargets returns the targets of every disk of a domain definition,
// with a source on the host or not.
func parseDiskTargets(r io.Reader) ([]string, error) {
	doc, err := xmlquery.Parse(r)
	if err != nil {
		return nil, err
	}
	es, err := xmlquery.QueryAll(doc, "//domain/devices/disk/target")
	if err != nil {
		return nil, err
	}
	l := make([]string, 0, len(es))
	for _, e := range es {
		if dev := e.SelectAttr("dev"); dev != "" {
			l = append(l, dev)
		}
	}
	return l, nil
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// virsh runs a virsh command on this node.
func (t *T) virsh(ctx context.Context, args ...string) error {
	t.Log().Infof("virsh %s", strings.Join(args, " "))
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "virsh", args...)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("virsh %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// inactiveDefinition returns the persistent definition of the domain.
func (t *T) inactiveDefinition(ctx context.Context) ([]byte, error) {
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "virsh", "dumpxml", "--inactive", t.Name)
	cmd.Stderr = &stderr
	b, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("virsh dumpxml --inactive %s: %w: %s", t.Name, err, strings.TrimSpace(stderr.String()))
	}
	return b, nil
}

// defineFrom makes definition the persistent definition of the domain on
// this node.
func (t *T) defineFrom(ctx context.Context, definition []byte) error {
	f, err := os.CreateTemp("", "osvc-move-*.xml")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(definition); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return t.virsh(ctx, "define", f.Name())
}

// remote runs a command on the node to, with stdin fed to it when not nil.
func (t *T) remote(ctx context.Context, to, cmdline string, stdin []byte) error {
	client, err := t.NewSSHClient(to)
	if err != nil {
		return err
	}
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()
	var stderr bytes.Buffer
	session.Stderr = &stderr
	if stdin != nil {
		session.Stdin = bytes.NewReader(stdin)
	}
	stop := context.AfterFunc(ctx, func() { _ = session.Close() })
	defer stop()
	t.Log().Infof("ssh %s %s", to, cmdline)
	if err := session.Run(cmdline); err != nil {
		return fmt.Errorf("ssh %s %s: %w: %s", to, cmdline, err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// putOverlays puts the overlays over the copied disks of the running guest.
func (t *T) putOverlays(ctx context.Context, overlays []moveOverlay) error {
	f, err := os.Open(t.configFile())
	if err != nil {
		return err
	}
	targets, err := parseDiskTargets(f)
	f.Close()
	if err != nil {
		return err
	}
	for _, o := range overlays {
		_ = os.Remove(o.Path)
	}
	return t.virsh(ctx, snapshotArgs(t.Name, overlays, targets)...)
}

// commitOverlays merges the overlays into the disks under them on this node,
// and puts the guest back on its disks. It goes on past a failure, so every
// disk it can is back on its disk, and says what failed.
func (t *T) commitOverlays(ctx context.Context, overlays []moveOverlay) error {
	var errs []string
	for _, o := range overlays {
		if err := t.virsh(ctx, blockcommitArgs(t.Name, o.Target)...); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

// removeOverlays removes the overlay files on this node.
func removeOverlays(overlays []moveOverlay) {
	for _, o := range overlays {
		_ = os.Remove(o.Path)
	}
}

// removeOverlaysCmdline returns the command removing the overlay files on the
// destination.
func removeOverlaysCmdline(overlays []moveOverlay) string {
	l := []string{"rm", "-f"}
	for _, o := range overlays {
		l = append(l, shellQuote(o.Path))
	}
	return strings.Join(l, " ")
}

// overlayTargets names the disks the overlays are over.
func overlayTargets(overlays []moveOverlay) string {
	l := make([]string, len(overlays))
	for i, o := range overlays {
		l[i] = o.Target
	}
	return strings.Join(l, ", ")
}

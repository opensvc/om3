package rescontainerkvm

import (
	"fmt"
	"html"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/opensvc/om3/v3/core/claim"
	"github.com/opensvc/om3/v3/core/resource"
	"github.com/opensvc/om3/v3/core/vsockauth"
	"github.com/opensvc/om3/v3/util/sizeconv"
)

var (
	partitionRe  = regexp.MustCompile(`<partition>[^<]*</partition>`)
	memoryRe     = regexp.MustCompile(`<memory(?: unit='([A-Za-z]+)')?>(\d+)</memory>`)
	systemdHexRe = regexp.MustCompile(`\\x([0-9a-f]{2})`)

	hypervisorEntryRe = regexp.MustCompile(`<entry>` + regexp.QuoteMeta(vsockauth.OEMPrefix) + `[^<]*</entry>`)
	smbiosRe          = regexp.MustCompile(`<smbios mode='[a-z]+'/>`)
	domainUUIDRe      = regexp.MustCompile(`<uuid>([^<]+)</uuid>`)
)

// partitionOf returns the libvirt partition of a process group.
//
// libvirt makes a slice of a partition by joining its elements with a dash,
// escaping them: /a/b/c is a-b-c.slice, which systemd nests in a.slice and
// a-b.slice. So the group, whose last element names the whole of it, is the
// partition holding the dashes the name of the slice is split on.
func partitionOf(id string) string {
	name := strings.TrimSuffix(path.Base(id), ".slice")
	words := strings.Split(name, "-")
	for i, word := range words {
		words[i] = systemdHexRe.ReplaceAllStringFunc(word, func(s string) string {
			b, err := strconv.ParseUint(s[2:], 16, 8)
			if err != nil {
				return s
			}
			return string(rune(b))
		})
	}
	return "/" + strings.Join(words, "/")
}

// definitionWithPartition returns the definition of a domain with this partition, the
// rest of it left as libvirt wrote it.
func definitionWithPartition(b []byte, partition string) ([]byte, error) {
	tag := []byte("<partition>" + partition + "</partition>")
	if partitionRe.Match(b) {
		return partitionRe.ReplaceAllLiteral(b, tag), nil
	}
	s := string(b)
	if i := strings.Index(s, "<resource>"); i >= 0 {
		i += len("<resource>")
		return []byte(s[:i] + "\n    " + string(tag) + s[i:]), nil
	}
	i := strings.LastIndex(s, "</domain>")
	if i < 0 {
		return nil, fmt.Errorf("no domain in the definition")
	}
	return []byte(s[:i] + "  <resource>\n    " + string(tag) + "\n  </resource>\n" + s[i:]), nil
}

// setPartition writes the partition in the definition file of a domain, which
// the start defines, and the other nodes receive.
func setPartition(file, partition string) error {
	return rewriteDefinition(file, func(b []byte) ([]byte, error) {
		return definitionWithPartition(b, partition)
	})
}

// setSerialLog writes the log of the serial port in the definition file of a
// domain, for a guest defined before it had one, or by a sysadmin.
func setSerialLog(file, log string) error {
	return rewriteDefinition(file, func(b []byte) ([]byte, error) {
		return definitionWithSerialLog(b, log), nil
	})
}

// setHypervisorKey writes this OEM string in the definition file of a domain.
func setHypervisorKey(file, oem string) error {
	return rewriteDefinition(file, func(b []byte) ([]byte, error) {
		return definitionWithHypervisorKey(b, oem)
	})
}

// definitionWithHypervisorKey returns the definition of a domain whose firmware
// holds this OEM string instead of the former one, the rest of it left as
// libvirt wrote it.
func definitionWithHypervisorKey(b []byte, oem string) ([]byte, error) {
	s := string(b)
	entry := "<entry>" + html.EscapeString(oem) + "</entry>"
	switch {
	case hypervisorEntryRe.MatchString(s):
		return []byte(hypervisorEntryRe.ReplaceAllLiteralString(s, entry)), nil
	case strings.Contains(s, "<oemStrings>"):
		s = strings.Replace(s, "<oemStrings>", "<oemStrings>\n      "+entry, 1)
	case strings.Contains(s, "<sysinfo type='smbios'>"):
		s = strings.Replace(s, "<sysinfo type='smbios'>", "<sysinfo type='smbios'>\n    <oemStrings>\n      "+entry+"\n    </oemStrings>", 1)
	default:
		i := strings.Index(s, "<os")
		if i < 0 {
			return nil, fmt.Errorf("no os in the definition")
		}
		s = s[:i] + "<sysinfo type='smbios'>\n    <oemStrings>\n      " + entry + "\n    </oemStrings>\n  </sysinfo>\n  " + s[i:]
	}
	smbios := "<smbios mode='sysinfo'/>"
	switch {
	case smbiosRe.MatchString(s):
		s = smbiosRe.ReplaceAllLiteralString(s, smbios)
	case strings.Contains(s, "</os>"):
		s = strings.Replace(s, "</os>", "  "+smbios+"\n  </os>", 1)
	default:
		return nil, fmt.Errorf("no os in the definition")
	}
	return []byte(s), nil
}

// domainUUID returns the uuid of a domain definition, empty when it has none.
func domainUUID(b []byte) string {
	m := domainUUIDRe.FindSubmatch(b)
	if m == nil {
		return ""
	}
	return string(m[1])
}

func rewriteDefinition(file string, rewrite func([]byte) ([]byte, error)) error {
	b, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	after, err := rewrite(b)
	if err != nil {
		return fmt.Errorf("%s: %w", file, err)
	}
	if string(after) == string(b) {
		return nil
	}
	info, err := os.Stat(file)
	if err != nil {
		return err
	}
	return os.WriteFile(file, after, info.Mode())
}

// definedMemory returns the memory a definition gives a domain, in bytes, 0
// when it says none.
func definedMemory(b []byte) int64 {
	m := memoryRe.FindSubmatch(b)
	if m == nil {
		return 0
	}
	n, err := strconv.ParseInt(string(m[2]), 10, 64)
	if err != nil {
		return 0
	}
	switch strings.ToLower(string(m[1])) {
	case "", "kib", "k":
		return n * 1024
	case "mib", "m":
		return n * 1024 * 1024
	case "gib", "g":
		return n * 1024 * 1024 * 1024
	case "b", "bytes":
		return n
	}
	return 0
}

// guestMemory returns the memory of the guest: what its definition says, the
// truth of a provisioned one, else the keyword.
func (t *T) guestMemory() int64 {
	if b, err := os.ReadFile(t.configFile()); err == nil {
		if n := definedMemory(b); n > 0 {
			return n
		}
	}
	if t.Memory != nil {
		return *t.Memory
	}
	return 0
}

// uncappableError says a namespace, or the object, counts on caps this node
// can not put on a guest: libvirt places it in a partition of its own, out of
// them all.
func (t *T) uncappableError() error {
	o, _ := t.GetObject().(resourceCapper)
	for _, claimType := range claim.ComputeTypes {
		if _, capped, err := claim.Limit(t.Path.Namespace, claimType, ""); err != nil {
			return err
		} else if capped {
			return fmt.Errorf("the %s namespace claims the %s, and this node can not cap a guest: it needs systemd and a libvirt with partitions", t.Path.Namespace, claimType)
		}
		if err := cappedError(o, t.RID(), claimType); err != nil {
			return err
		}
	}
	return nil
}

// resourceCapper says the cap of the groups holding a resource.
type resourceCapper interface {
	ResourceCap(rid, claimType string) (int64, error)
}

// cappedError says the groups holding a resource cap what this node can not
// cap on a guest.
func cappedError(o resourceCapper, rid, claimType string) error {
	if o == nil {
		return nil
	}
	limit, err := o.ResourceCap(rid, claimType)
	if err != nil {
		return err
	}
	if limit != claim.Unbounded {
		return fmt.Errorf("%s is capped on the %s, and this node can not cap a guest: it needs systemd and a libvirt with partitions", rid, claimType)
	}
	return nil
}

// memCapError says the process group of the resource can not hold the guest.
func (t *T) memCapError() error {
	if t.partition() == "" {
		return t.uncappableError()
	}
	o, ok := t.GetObject().(interface {
		MemCap(rid string) (int64, error)
	})
	if !ok {
		return nil
	}
	limit, err := o.MemCap(t.RID())
	if err != nil {
		return err
	}
	need := t.guestMemory() + claim.GuestMemoryMargin
	if limit != claim.Unbounded && t.guestMemory() != 0 && limit < need {
		return fmt.Errorf("memory cap %s: the guest needs %s, its memory and what qemu takes", sizeconv.BSizeCompact(float64(limit)), sizeconv.BSizeCompact(float64(need)))
	}
	return t.sharedMemCapError()
}

// sharedMemCapError says the guests of the object do not fit its cap together:
// they share its group, each fitting it alone, and one of them is killed the
// day they all touch their memory.
func (t *T) sharedMemCapError() error {
	o, ok := t.GetObject().(interface {
		ObjectMemCap() (int64, error)
		Resources() resource.Drivers
	})
	if !ok {
		return nil
	}
	limit, err := o.ObjectMemCap()
	if err != nil || limit == claim.Unbounded {
		return err
	}
	need := int64(0)
	guests := 0
	for _, r := range o.Resources() {
		if g, ok := r.(*T); ok && !g.IsDisabled() && g.guestMemory() > 0 {
			need += g.guestMemory() + claim.GuestMemoryMargin
			guests++
		}
	}
	if guests < 2 || limit >= need {
		return nil
	}
	return fmt.Errorf("memory cap %s: the %d guests of the object need %s together", sizeconv.BSizeCompact(float64(limit)), guests, sizeconv.BSizeCompact(float64(need)))
}

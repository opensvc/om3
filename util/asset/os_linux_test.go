//go:build linux

package asset

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestOSProbe verifies the vendor and the release told of the files of
// /etc of the distributions are the ones OpenSVC v2 tells of the same files,
// as the filtersets of the collector match them.
func TestOSProbe(t *testing.T) {
	cases := map[string]struct {
		files   map[string]string
		vendor  string
		release string
	}{
		"ubuntu 24.04": {
			files: map[string]string{
				"os-release":     "PRETTY_NAME=\"Ubuntu 24.04.5 LTS\"\nNAME=\"Ubuntu\"\nVERSION_ID=\"24.04\"\nVERSION=\"24.04.5 LTS (Noble Numbat)\"\nID=ubuntu\n",
				"lsb-release":    "DISTRIB_ID=Ubuntu\nDISTRIB_RELEASE=24.04\nDISTRIB_DESCRIPTION=\"Ubuntu 24.04.5 LTS\"\n",
				"debian_version": "trixie/sid\n",
			},
			vendor:  "Ubuntu",
			release: "24.04.5 LTS",
		},
		"debian 12": {
			files: map[string]string{
				"os-release":     "PRETTY_NAME=\"Debian GNU/Linux 12 (bookworm)\"\nNAME=\"Debian GNU/Linux\"\nVERSION_ID=\"12\"\nID=debian\n",
				"debian_version": "12.7\n",
			},
			vendor:  "Debian",
			release: "GNU/Linux 12 (bookworm)",
		},
		"rhel 9": {
			files: map[string]string{
				"os-release":     "NAME=\"Red Hat Enterprise Linux\"\nVERSION=\"9.4 (Plow)\"\nID=\"rhel\"\nVERSION_ID=\"9.4\"\nPRETTY_NAME=\"Red Hat Enterprise Linux 9.4 (Plow)\"\n",
				"redhat-release": "Red Hat Enterprise Linux release 9.4 (Plow)\n",
			},
			vendor:  "Red Hat",
			release: "Enterprise Linux 9.4 (Plow)",
		},
		"rhel 7 server": {
			files: map[string]string{
				"os-release":     "NAME=\"Red Hat Enterprise Linux Server\"\nVERSION=\"7.9 (Maipo)\"\nID=\"rhel\"\nVERSION_ID=\"7.9\"\nPRETTY_NAME=\"Red Hat Enterprise Linux Server 7.9 (Maipo)\"\n",
				"redhat-release": "Red Hat Enterprise Linux Server release 7.9 (Maipo)\n",
			},
			vendor:  "Red Hat",
			release: "Enterprise Linux Server 7.9 (Maipo)",
		},
		"rhel 7 short pretty name": {
			files: map[string]string{
				"os-release":     "NAME=\"Red Hat Enterprise Linux\"\nID=\"rhel\"\nVERSION_ID=\"7.2\"\nVERSION=\"7.2\"\nPRETTY_NAME=\"Red Hat Enterprise Linux\"\n",
				"redhat-release": "Red Hat Enterprise Linux release 7.2\n",
			},
			vendor:  "Red Hat",
			release: "Enterprise Linux 7.2",
		},
		"centos 7": {
			files: map[string]string{
				"os-release":     "NAME=\"CentOS Linux\"\nVERSION=\"7 (Core)\"\nID=\"centos\"\nVERSION_ID=\"7\"\nPRETTY_NAME=\"CentOS Linux 7 (Core)\"\n",
				"redhat-release": "CentOS Linux release 7.9.2009 (Core)\n",
			},
			vendor:  "CentOS",
			release: "7.9.2009 (Core)",
		},
		"rocky 9": {
			files: map[string]string{
				"os-release":     "NAME=\"Rocky Linux\"\nVERSION=\"9.4 (Blue Onyx)\"\nID=\"rocky\"\nID_LIKE=\"rhel centos fedora\"\nVERSION_ID=\"9.4\"\nPRETTY_NAME=\"Rocky Linux 9.4 (Blue Onyx)\"\n",
				"redhat-release": "Rocky Linux release 9.4 (Blue Onyx)\n",
			},
			vendor:  "Red Hat",
			release: "Rocky Linux 9.4 (Blue Onyx)",
		},
		"oracle linux 8": {
			files: map[string]string{
				"os-release":     "NAME=\"Oracle Linux Server\"\nVERSION=\"8.10\"\nID=\"ol\"\nVERSION_ID=\"8.10\"\nPRETTY_NAME=\"Oracle Linux Server 8.10\"\n",
				"oracle-release": "Oracle Linux Server release 8.10\n",
				"redhat-release": "Red Hat Enterprise Linux release 8.10 (Ootpa)\n",
			},
			vendor:  "Oracle",
			release: "Linux Server 8.10",
		},
		"oracle vm server": {
			files: map[string]string{
				"oracle-release": "Oracle VM server release 3.4.6\n",
				"redhat-release": "Oracle VM server release 3.4.6\n",
			},
			vendor:  "Oracle",
			release: "VM server release 3.4.6",
		},
		"sles 15": {
			files: map[string]string{
				"os-release": "NAME=\"SLES\"\nVERSION=\"15-SP5\"\nVERSION_ID=\"15.5\"\nPRETTY_NAME=\"SUSE Linux Enterprise Server 15 SP5\"\nID=\"sles\"\n",
			},
			vendor:  "SuSE",
			release: "Linux Enterprise Server 15 SP5",
		},
		"sles 11": {
			files: map[string]string{
				"SuSE-release": "SUSE Linux Enterprise Server 11 (x86_64)\nVERSION = 11\nPATCHLEVEL = 4\n",
			},
			vendor:  "SuSE",
			release: "11.4",
		},
		"alpine": {
			files: map[string]string{
				"os-release":     "NAME=\"Alpine Linux\"\nID=alpine\nVERSION_ID=3.20.3\nPRETTY_NAME=\"Alpine Linux v3.20\"\n",
				"alpine-release": "3.20.3\n",
			},
			vendor:  "Alpine",
			release: "Linux v3.20",
		},
		"alpine without os-release": {
			files: map[string]string{
				"alpine-release": "3.20.3\n",
			},
			vendor:  "Alpine",
			release: "3.20.3",
		},
		"fedora": {
			files: map[string]string{
				"os-release": "NAME=\"Fedora Linux\"\nVERSION=\"40 (Server Edition)\"\nID=fedora\nVERSION_ID=40\nPRETTY_NAME=\"Fedora Linux 40 (Server Edition)\"\n",
			},
			vendor:  "Fedora",
			release: "Linux 40 (Server Edition)",
		},
		"unknown id": {
			files: map[string]string{
				"os-release": "NAME=\"Some Distro\"\nID=somedistro\nPRETTY_NAME=\"Some Distro 1.0\"\n",
			},
			vendor:  "Some Distro",
			release: "1.0",
		},
		"lsb only": {
			files: map[string]string{
				"lsb-release": "DISTRIB_ID=Mint\nDISTRIB_RELEASE=\nDISTRIB_DESCRIPTION=\"Mint 21.3\"\n",
			},
			vendor:  "Mint",
			release: "21.3",
		},
		"gentoo without os-release": {
			files: map[string]string{
				"gentoo-release": "Gentoo Base System release 2.14\n",
			},
			vendor:  "Unknown",
			release: "Gentoo Base System  2.14",
		},
		"nothing": {
			vendor:  "Unknown",
			release: "Unknown",
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			require.NoError(t, os.Mkdir(filepath.Join(root, "etc"), 0o755))
			for p, s := range c.files {
				require.NoError(t, os.WriteFile(filepath.Join(root, "etc", p), []byte(s), 0o644))
			}
			probe := newOSProbe(root)
			require.Equal(t, c.vendor, probe.Vendor(), "vendor")
			require.Equal(t, c.release, probe.Release(), "release")
		})
	}
}

// TestOSProbeOpenSUSE verifies the openSUSE distributions identified as
// opensuse-<edition> are told as a reader expects them, where OpenSVC v2
// reports the name of the edition as the vendor, and Unknown as the
// release of Tumbleweed.
func TestOSProbeOpenSUSE(t *testing.T) {
	cases := map[string]struct {
		osRelease string
		vendor    string
		release   string
	}{
		"leap 15.6": {
			osRelease: "NAME=\"openSUSE Leap\"\nVERSION=\"15.6\"\nID=\"opensuse-leap\"\nID_LIKE=\"suse opensuse\"\nVERSION_ID=\"15.6\"\nPRETTY_NAME=\"openSUSE Leap 15.6\"\n",
			vendor:    "SuSE",
			release:   "Leap 15.6",
		},
		"tumbleweed": {
			osRelease: "NAME=\"openSUSE Tumbleweed\"\nID=\"opensuse-tumbleweed\"\nID_LIKE=\"opensuse suse\"\nVERSION_ID=\"20241001\"\nPRETTY_NAME=\"openSUSE Tumbleweed\"\n",
			vendor:    "SuSE",
			release:   "Tumbleweed 20241001",
		},
		"leap micro": {
			osRelease: "NAME=\"openSUSE Leap Micro\"\nVERSION=\"6.0\"\nID=\"opensuse-leap-micro\"\nVERSION_ID=\"6.0\"\nPRETTY_NAME=\"openSUSE Leap Micro 6.0\"\n",
			vendor:    "SuSE",
			release:   "Leap Micro 6.0",
		},
		"no version": {
			osRelease: "NAME=\"openSUSE Tumbleweed\"\nID=\"opensuse-tumbleweed\"\n",
			vendor:    "SuSE",
			release:   "Tumbleweed",
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			require.NoError(t, os.Mkdir(filepath.Join(root, "etc"), 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(root, "etc", "os-release"), []byte(c.osRelease), 0o644))
			probe := newOSProbe(root)
			require.Equal(t, c.vendor, probe.Vendor(), "vendor")
			require.Equal(t, c.release, probe.Release(), "release")
		})
	}
}

package pkgupdate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const aptPackages = `Package: opensvc-server
Version: 3.0.0~rc7.0.gc4856bb6+ubuntu26.04
Architecture: amd64
Filename: pool/main/o/opensvc-server/opensvc-server_3.0.0~rc7_amd64.deb
SHA256: 07
Description: the opensvc agent
 its daemon, on several lines

Package: opensvc-server
Version: 3.0.0~rc56.0.g36ac166f4+ubuntu26.04
Architecture: amd64
Filename: pool/main/o/opensvc-server/opensvc-server_3.0.0~rc56_amd64.deb
SHA256: 56

Package: opensvc-server
Version: 3.0.0~rc57.0.g1+ubuntu26.04
Architecture: arm64
Filename: pool/main/o/opensvc-server/opensvc-server_3.0.0~rc57_arm64.deb
SHA256: 57

Package: opensvc-webapp
Version: 1.0.0~rc9.0.g452f02b-1
Architecture: all
Filename: pool/main/o/opensvc-webapp/opensvc-webapp_1.0.0~rc9-1_all.deb
SHA256: 09
`

func TestParseAptPackages(t *testing.T) {
	l, err := parseAptPackages(strings.NewReader(aptPackages))
	require.NoError(t, err)
	require.Len(t, l, 4)
	l = keepArch(l, "amd64", "all")
	require.Len(t, l, 3, "the packages of another architecture are left out")
	p, ok := Latest(l, "opensvc-server", CompareDeb)
	require.True(t, ok)
	assert.Equal(t, "3.0.0~rc56.0.g36ac166f4+ubuntu26.04", p.Version, "rc56 is newer than rc7")
	assert.Equal(t, "pool/main/o/opensvc-server/opensvc-server_3.0.0~rc56_amd64.deb", p.Path)
	assert.Equal(t, "56", p.SHA256)
	p, ok = Latest(l, "opensvc-webapp", CompareDeb)
	require.True(t, ok)
	assert.Equal(t, "all", p.Arch)
	_, ok = Latest(l, "opensvc-client", CompareDeb)
	assert.False(t, ok)
}

func TestParseRepomd(t *testing.T) {
	href, err := parseRepomd(strings.NewReader(`<?xml version="1.0"?>
<repomd xmlns="http://linux.duke.edu/metadata/repo">
  <data type="filelists"><location href="repodata/f-filelists.xml.zst"/></data>
  <data type="primary"><location href="repodata/p-primary.xml.zst"/></data>
</repomd>`))
	require.NoError(t, err)
	assert.Equal(t, "repodata/p-primary.xml.zst", href)
}

func TestParsePrimary(t *testing.T) {
	l, err := parsePrimary(strings.NewReader(`<?xml version="1.0"?>
<metadata xmlns="http://linux.duke.edu/metadata/common" xmlns:rpm="http://linux.duke.edu/metadata/rpm" packages="3">
<package type="rpm">
  <name>opensvc-server</name><arch>x86_64</arch>
  <version epoch="0" ver="3.0.0~rc7.0.gc4856bb6" rel="1.el9"/>
  <checksum type="sha256" pkgid="YES">aa07</checksum>
  <location href="opensvc-server-3.0.0~rc7.0.gc4856bb6-1.el9.x86_64.rpm"/>
</package>
<package type="rpm">
  <name>opensvc-server</name><arch>x86_64</arch>
  <version epoch="0" ver="3.0.0~rc56.0.g36ac166f4" rel="1.el9"/>
  <checksum type="sha256" pkgid="YES">aa56</checksum>
  <location href="opensvc-server-3.0.0~rc56.0.g36ac166f4-1.el9.x86_64.rpm"/>
</package>
<package type="rpm">
  <name>opensvc-webapp</name><arch>noarch</arch>
  <version epoch="1" ver="1.0.0" rel="1"/>
  <checksum type="sha256" pkgid="YES">bb</checksum>
  <location href="opensvc-webapp-1.0.0-1.noarch.rpm"/>
</package>
</metadata>`))
	require.NoError(t, err)
	l = keepArch(l, "x86_64", "noarch")
	require.Len(t, l, 3)
	p, ok := Latest(l, "opensvc-server", CompareRPM)
	require.True(t, ok)
	assert.Equal(t, "3.0.0~rc56.0.g36ac166f4-1.el9", p.Version)
	assert.Equal(t, "aa56", p.SHA256)
	p, _ = Latest(l, "opensvc-webapp", CompareRPM)
	assert.Equal(t, "1:1.0.0-1", p.Version, "a non-zero epoch is part of the version")
}

func TestDistroRepositories(t *testing.T) {
	read := func(content string) Distro {
		t.Helper()
		p := filepath.Join(t.TempDir(), "os-release")
		require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
		d, err := ReadDistro(p)
		require.NoError(t, err)
		return d
	}
	id, suite, err := read("ID=ubuntu\nVERSION_ID=\"26.04\"\nVERSION_CODENAME=resolute\n").AptRepository("prod")
	require.NoError(t, err)
	assert.Equal(t, "ubuntu", id)
	assert.Equal(t, "prod-opensvc-v3-resolute", suite)

	id, suite, err = read("ID=linuxmint\nID_LIKE=\"ubuntu debian\"\nVERSION_CODENAME=xia\nUBUNTU_CODENAME=noble\n").AptRepository("uat")
	require.NoError(t, err)
	assert.Equal(t, "ubuntu", id, "a derivative takes the packages of the distribution it is made of")
	assert.Equal(t, "uat-opensvc-v3-noble", suite)

	dir, err := read("ID=\"rocky\"\nID_LIKE=\"rhel centos fedora\"\nVERSION_ID=\"9.6\"\n").RPMRepository("prod")
	require.NoError(t, err)
	assert.Equal(t, "prod-opensvc-v3-rhel9", dir)

	dir, err = read("ID=\"opensuse-leap\"\nID_LIKE=\"suse opensuse\"\nVERSION_ID=\"15.6\"\n").RPMRepository("prod")
	require.NoError(t, err)
	assert.Equal(t, "prod-opensvc-v3-sles15", dir)

	_, err = read("ID=\"opensuse-tumbleweed\"\nID_LIKE=\"opensuse suse\"\nVERSION_ID=\"20261102\"\n").RPMRepository("prod")
	assert.Error(t, err, "a rolling release has no repository")

	_, _, err = read("ID=alpine\nVERSION_ID=3.22\n").AptRepository("prod")
	assert.Error(t, err)
}

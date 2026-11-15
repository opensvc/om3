package streamlog

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A journalctl knows --grep from systemd 237, built with PCRE2: rhel7's 219
// does not, nor one built without PCRE2. An output it can not read is taken
// as knowing it.
func TestJournalctlHasGrep(t *testing.T) {
	assert.False(t, journalctlHasGrep("systemd 219\n+PAM +AUDIT +SELINUX +IMA -APPARMOR +SMACK +SYSVINIT +UTMP +LIBCRYPTSETUP +GCRYPT +GNUTLS +ACL +XZ +LZ4 -SECCOMP +BLKID +ELFUTILS +KMOD +IDN\n"))
	assert.False(t, journalctlHasGrep("systemd 237\n+PAM +AUDIT -PCRE2 default-hierarchy=hybrid\n"))
	assert.True(t, journalctlHasGrep("systemd 237\n+PAM +AUDIT +PCRE2 default-hierarchy=hybrid\n"))
	assert.True(t, journalctlHasGrep("systemd 255 (255.4-1ubuntu8.6)\n+PAM +AUDIT +SELINUX +APPARMOR +IMA +SMACK +SECCOMP +GCRYPT -GNUTLS +OPENSSL +ACL +BLKID +CURL +ELFUTILS +FIDO2 +IDN2 -IDN +IPTC +KMOD +LIBCRYPTSETUP +LIBFDISK +PCRE2 -PWQUALITY\n"))
	assert.True(t, journalctlHasGrep(""))
}

// A read filtering on a pattern where journalctl has no --grep is refused,
// as rhel7's: it answered no line, as if none matched (card 304). A read
// filtering on nothing is not.
func TestAGrepUnsupportedIsRefused(t *testing.T) {
	saved := journalctlVersion
	journalctlVersion = func() string { return "systemd 219\n" }
	grepSupportOnce = sync.Once{}
	t.Cleanup(func() {
		journalctlVersion = saved
		grepSupportOnce = sync.Once{}
	})
	pattern := "change state"
	_, err := StreamConfig{Grep: &pattern}.args()
	require.ErrorIs(t, err, ErrGrepUnsupported)
	empty := ""
	_, err = StreamConfig{Grep: &empty}.args()
	require.NoError(t, err)
	_, err = StreamConfig{}.args()
	require.NoError(t, err)
}

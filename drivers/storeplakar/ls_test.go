package storeplakar

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseLsReadsTheCapturedLines pins the reading of real plakar ls
// outputs, an empty one and escaped tag values included.
func TestParseLsReadsTheCapturedLines(t *testing.T) {
	ts := func(s string) time.Time {
		v, err := time.Parse(time.RFC3339, s)
		require.NoError(t, err)
		return v
	}
	cases := map[string]struct {
		fixture string
		want    []snapshot
	}{
		"ls -tags": {
			fixture: "08-ls-tags.txt",
			want: []snapshot{{
				ID:        "1f988d33",
				Timestamp: ts("2026-10-15T16:50:17Z"),
				Size:      64 << 10,
				Root:      "/tmp/fix/data",
				Tags:      map[string]string{"om3": "", "lab": "", "app1": ""},
			}},
		},
		"ls -tag -latest, no tags asked": {
			fixture: "09-ls-tag-latest.txt",
			want: []snapshot{{
				ID:        "1f988d33",
				Timestamp: ts("2026-10-15T16:50:17Z"),
				Size:      64 << 10,
				Root:      "/tmp/fix/data",
				Tags:      map[string]string{},
			}},
		},
		"ls -uuid": {
			fixture: "76-ls-uuid.txt",
			want: []snapshot{{
				ID:        "55d956b3b3a9548e00d86babf03a3e5cbdc3f6dbf59bc76dc91021fc266a5699",
				Timestamp: ts("2026-10-15T17:02:28Z"),
				Size:      64 << 10,
				Root:      "/tmp/fix/data",
				Tags:      map[string]string{"om3": "", "json": ""},
			}},
		},
		"escaped values": {
			fixture: "83-ls-uuid-tags-many.txt",
			want: []snapshot{{
				ID:        "ca695b644315350bdfa5c160d7e61972ac30bd417eb1cc408c1ca01fbb8f01fb",
				Timestamp: ts("2026-10-27T08:36:07Z"),
				Size:      14,
				Root:      "/tmp/fix/data",
				Tags: map[string]string{
					"om3.run":       "r1",
					"om3.note":      "a note=with/slash",
					"om3.created":   "2026-10-27T10:00:00.123Z",
					"om3.member":    "app#1",
					"om3.kind":      "dir",
					"om3.protected": "",
				},
			}},
		},
		"ls -latest -tags": {
			fixture: "63-ls-latest-all.txt",
			want: []snapshot{{
				ID:        "55d956b3",
				Timestamp: ts("2026-10-15T17:02:28Z"),
				Size:      64 << 10,
				Root:      "/tmp/fix/data",
				Tags:      map[string]string{"om3": "", "json": ""},
			}},
		},
		"a marker, empty, read from stdin": {
			fixture: "91-ls-after-marker.txt",
			want: []snapshot{
				{
					ID:        "bba699210cdd8f50163baa2e40680a4e27ac36f732ecbaed7d981424ed6cb51b",
					Timestamp: ts("2026-10-27T08:36:16Z"),
					Size:      0,
					Root:      "/",
					Tags:      map[string]string{"om3.run": "r1", "om3.complete": ""},
				},
				{
					ID:        "ca695b644315350bdfa5c160d7e61972ac30bd417eb1cc408c1ca01fbb8f01fb",
					Timestamp: ts("2026-10-27T08:36:07Z"),
					Size:      14,
					Root:      "/tmp/fix/data",
					Tags: map[string]string{
						"om3.run":       "r1",
						"om3.note":      "a note=with/slash",
						"om3.created":   "2026-10-27T10:00:00.123Z",
						"om3.member":    "app#1",
						"om3.kind":      "dir",
						"om3.protected": "",
					},
				},
			},
		},
		"no match": {
			fixture: "11-ls-tag-nomatch.txt",
			want:    []snapshot{},
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := parseLs(fixture(t, c.fixture))
			require.NoError(t, err)
			assert.Equal(t, c.want, got)
		})
	}
}

// TestParseLsReadsTheLineFromBothEnds pins a root holding a space, and the
// refusal of a line it cannot read.
func TestParseLsReadsTheLineFromBothEnds(t *testing.T) {
	got, err := parseLs([]byte("2026-10-15T16:50:17Z   1f988d33    1.5 MiB        3s /srv/my data tags=om3.run=r1\n"))
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "/srv/my data", got[0].Root, "a root holding a space")
	assert.Equal(t, int64(1.5*(1<<20)), got[0].Size)
	assert.Equal(t, map[string]string{"om3.run": "r1"}, got[0].Tags)

	_, err = parseLs([]byte("garbage\n"))
	assert.Error(t, err)
}

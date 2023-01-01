package instance

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"opensvc.com/opensvc/core/kind"
	"opensvc.com/opensvc/core/provisioned"
	"opensvc.com/opensvc/core/resource"
	"opensvc.com/opensvc/core/resourceid"
	"opensvc.com/opensvc/core/status"
)

func Test_Status_Unmarshal(t *testing.T) {
	var IStatus Status
	path := filepath.Join("testdata", "status.json")
	b, err := os.ReadFile(path)
	require.Nil(t, err)

	err = json.Unmarshal(b, &IStatus)
	require.Nil(t, err)

	expected := Status{
		App:         "default",
		Avail:       status.Down,
		Overall:     status.Down,
		Csum:        "01e51d8e37b378e2281ccf72d09e5e1b",
		Kind:        kind.Svc,
		Provisioned: provisioned.Mixed,
		Updated:     time.Date(2022, time.December, 28, 11, 21, 45, 800780633, time.UTC),
		Resources: []resource.ExposedStatus{
			{
				ResourceID: (*resourceid.T)(nil),
				Rid:        "volume#1",
				Label:      "data2",
				Log: []*resource.StatusLogEntry{
					{
						Level:   "info",
						Message: "vol/data2 avail down",
					},
				},
				Status: status.Down,
				Type:   "volume",
				Provisioned: resource.ProvisionStatus{
					Mtime: time.Date(2022, time.November, 29, 18, 10, 46, 524120074, time.UTC),
					State: provisioned.True,
				},
			},
			{
				ResourceID: (*resourceid.T)(nil),
				Rid:        "fs#1",
				Label:      "flag /dev/shm/opensvc/svc/svc2/fs#1.flag",
				Status:     status.Down,
				Type:       "fs.flag",
				Provisioned: resource.ProvisionStatus{
					Mtime: time.Date(2022, time.November, 28, 21, 46, 25, 853702101, time.UTC),
					State: provisioned.False,
				},
			},
			{
				ResourceID: (*resourceid.T)(nil),
				Rid:        "app#1",
				Label:      "forking app.forking",
				Log: []*resource.StatusLogEntry{
					{
						Level:   "info",
						Message: "not evaluated (fs#1 is down)",
					},
				},
				Status: 1,
				Type:   "app.forking",
				Provisioned: resource.ProvisionStatus{
					Mtime: time.Date(2022, time.November, 28, 21, 46, 25, 849702075, time.UTC),
					State: provisioned.False,
				},
				Restart: 2,
			},
		},
	}
	require.Equalf(t, expected, IStatus, "expected %+v\ngot %+v", expected, IStatus)
}

package instance

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/opensvc/om3/core/provisioned"
	"github.com/opensvc/om3/core/resource"
	"github.com/opensvc/om3/core/resourceid"
	"github.com/opensvc/om3/core/status"
)

func Test_Status_Unmarshal(t *testing.T) {
	var IStatus Status
	path := filepath.Join("testdata", "status.json")
	b, err := os.ReadFile(path)
	require.Nil(t, err)

	err = json.Unmarshal(b, &IStatus)
	require.Nil(t, err)

	expected := Status{
		Avail:       status.Down,
		Overall:     status.Down,
		Provisioned: provisioned.Mixed,
		UpdatedAt:   time.Date(2022, time.December, 28, 11, 21, 45, 800780633, time.UTC),
		Resources: ResourceStatuses{
			"volume#1": {
				ResourceID: (*resourceid.T)(nil),
				Label:      "data2",
				Log: []resource.StatusLogEntry{
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
			"fs#1": {
				ResourceID: (*resourceid.T)(nil),
				Label:      "flag /dev/shm/opensvc/svc/svc2/fs#1.flag",
				Status:     status.Down,
				Type:       "fs.flag",
				Provisioned: resource.ProvisionStatus{
					Mtime: time.Date(2022, time.November, 28, 21, 46, 25, 853702101, time.UTC),
					State: provisioned.False,
				},
			},
			"app#1": {
				ResourceID: (*resourceid.T)(nil),
				Label:      "forking app.forking",
				Log: []resource.StatusLogEntry{
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

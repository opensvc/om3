package commoncmd

import "github.com/opensvc/om3/v3/daemon/api"

// SyncSnapshotsTab is the column spec of the sync snapshot listings.
const SyncSnapshotsTab = "tab=OBJECT:meta.object,RID:data.rid,CREATED_AT:data.created_at,SNAPSHOT:data.id,CLUSTER:data.cluster,NODE:data.node,MEMBERS:data.members[*],LOCATION:data.location,PROTECTED:data.protected,NOTE:data.note"

// LocalSyncSnapshots sets the times of the snapshots in the local zone of
// the requester: the stores hold them in UTC, a node answering in its own,
// and a listing mixing the two is read wrong.
func LocalSyncSnapshots(items api.SyncSnapshotItems) {
	for i := range items {
		items[i].Data.CreatedAt = items[i].Data.CreatedAt.Local()
	}
}

// LocalStoreSnapshots sets the times of the snapshots in the local zone of
// the requester, as LocalSyncSnapshots.
func LocalStoreSnapshots(items []api.StoreSnapshot) {
	for i := range items {
		items[i].CreatedAt = items[i].CreatedAt.Local()
		if items[i].CompletedAt != nil {
			t := items[i].CompletedAt.Local()
			items[i].CompletedAt = &t
		}
	}
}

package omcmd

import (
	"context"
	"errors"
	"time"

	"github.com/opensvc/om3/v3/core/backup"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
)

type (
	CmdStoreSnapshotList struct {
		OptsGlobal
		Store   string
		Path    string
		Cluster string
	}

	// storeSnapshot is a run a store holds, as listed.
	storeSnapshot struct {
		Store       string    `json:"store"`
		Path        string    `json:"path"`
		ID          string    `json:"id"`
		CreatedAt   time.Time `json:"created_at"`
		ClusterID   string    `json:"cluster_id"`
		ClusterName string    `json:"cluster_name"`
		Node        string    `json:"node"`
		Protected   bool      `json:"protected"`
		Note        string    `json:"note"`
		Members     []string  `json:"members"`

		CompletedAt  time.Time `json:"completed_at,omitzero"`
		LogicalBytes int64     `json:"logical_bytes"`
	}
)

func (t *CmdStoreSnapshotList) Run() error {
	n, err := object.NewNode()
	if err != nil {
		return err
	}
	refs := []string{t.Store}
	if t.Store == "" {
		refs = refs[:0]
		for _, item := range n.ListStores() {
			refs = append(refs, item.Ref)
		}
	}
	ctx := context.Background()
	l := make([]storeSnapshot, 0)
	var errs error
	for _, ref := range refs {
		s, err := n.Store(ref, naming.Path{})
		if err != nil {
			errs = errors.Join(errs, err)
			continue
		}
		runs, err := s.Runs(ctx, backup.RunFilter{Path: t.Path, Cluster: t.Cluster})
		_ = s.Close()
		if err != nil {
			errs = errors.Join(errs, err)
		}
		for _, run := range runs {
			l = append(l, storeSnapshot{
				Store:       s.Ref(),
				Path:        run.Path,
				ID:          run.ID,
				CreatedAt:   run.CreatedAt.Local(),
				ClusterID:   run.ClusterID,
				ClusterName: run.ClusterName,
				Node:        run.Node,
				Protected:   run.Protected,
				Note:        run.Note,
				Members:     run.MemberNames(),

				CompletedAt:  localTime(run.CompletedAt),
				LogicalBytes: run.LogicalBytes,
			})
		}
	}
	return errors.Join(errs, output.Renderer{
		DefaultOutput: "tab=STORE:store,PATH:path,CREATED_AT:created_at,SNAPSHOT:id,CLUSTER:cluster_name,NODE:node,MEMBERS:members[*],PROTECTED:protected,NOTE:note",
		Output:        t.Output,
		Sort:          t.Sort,
		Color:         t.Color,
		Data:          l,
		Colorize:      rawconfig.Colorize,
	}.Print())
}

// localTime returns t in the local zone of the requester, the zero time kept
// zero for the listing to leave it out: the stores hold their times in UTC.
func localTime(t time.Time) time.Time {
	if t.IsZero() {
		return t
	}
	return t.Local()
}

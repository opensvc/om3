package daemonapi

import (
	"fmt"

	"github.com/labstack/echo/v4"
	"github.com/opensvc/om3/core/nodeselector"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/objectselector"
	"github.com/opensvc/om3/core/path"
	"github.com/opensvc/om3/daemon/rbac"
)

type (
	Meta struct {
		Context echo.Context
		Path    *string
		Node    *string

		pathMap path.M
		nodeMap nodeselector.ResultMap
		grants  rbac.Grants
	}
)

func (m *Meta) HasNode(s string) bool {
	return m.nodeMap.Has(s)
}

func (m *Meta) HasPath(s string) bool {
	if v := m.pathMap.Has(s); !v {
		return false
	}
	p, err := path.Parse(s)
	if err != nil {
		return false
	}
	grants := m.Grants()
	return grants.Has(rbac.RoleGuest, p.Namespace) || grants.Has(rbac.RoleAdmin, p.Namespace) || grants.HasRole(rbac.RoleRoot)
}

func (m *Meta) Grants() rbac.Grants {
	if m.grants == nil {
		m.grants = grantsFromContext(m.Context)
	}
	return m.grants
}

func (m *Meta) Expand() error {
	if err := m.expandNode(); err != nil {
		return err
	}
	if err := m.expandPath(); err != nil {
		return err
	}
	return nil
}

func (m *Meta) expandPath() error {
	paths := object.StatusData.GetPaths()
	if m.Path != nil {
		selection := objectselector.NewSelection(
			*m.Path,
			objectselector.SelectionWithInstalled(paths),
			objectselector.SelectionWithLocal(true),
		)
		matchedPaths, err := selection.Expand()
		if err != nil {
			return fmt.Errorf("expand path selection %s: %w", *m.Path, err)
		}
		m.pathMap = matchedPaths.StrMap()
	} else {
		m.pathMap = paths.StrMap()
	}
	return nil
}

func (m *Meta) expandNode() error {
	var node string
	if m.Node == nil {
		node = "*"
	} else {
		node = *m.Node
	}
	selection := nodeselector.New(
		node,
		nodeselector.WithLocal(true),
	)
	if nodeMap, err := selection.ExpandMap(); err != nil {
		return fmt.Errorf("expand node selection %s: %w", node, err)
	} else {
		m.nodeMap = nodeMap
	}
	return nil
}

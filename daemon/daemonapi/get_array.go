package daemonapi

import (
	"net/http"
	"sort"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/key"
)

func (a *DaemonAPI) GetArray(ctx echo.Context, params api.GetArrayParams) error {
	var (
		items api.ArrayItems
	)
	if v, err := assertRoot(ctx); !v {
		return err
	}

	ccfg, err := object.NewCluster()
	if err != nil {
		return JSONProblemf(ctx, http.StatusInternalServerError, "Internal Server Error", "get cluster object: %s", err)
	}
	sections := ccfg.Config().SectionStrings()
	for _, section := range sections {
		parts := strings.Split(section, "#")
		if parts[0] != "array" {
			continue
		}
		arrayType := ccfg.Config().Get(key.New(section, "type"))
		item := api.ArrayItem{
			Name: parts[1],
			Type: arrayType,
		}
		items = append(items, item)
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})

	return ctx.JSON(http.StatusOK, api.ArrayList{Kind: "ArrayList", Items: items})
}

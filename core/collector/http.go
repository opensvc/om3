package collector

import (
	"net/http"

	"github.com/opensvc/om3/util/httphelper"
	"github.com/opensvc/om3/util/requestfactory"
)

func NewRequester(dbOpensvc string, auth string, insecure bool) (*httphelper.T, error) {
	server, err := BaseURL(dbOpensvc)
	if err != nil {
		return nil, err
	}
	// prepare default default header
	header := http.Header{}
	header.Set("Content-Type", "application/json")
	header.Set("Authorization", auth)

	factory := requestfactory.New(server, header)

	cli := httphelper.NewHttpsClient(insecure)

	return httphelper.New(cli, factory), nil
}

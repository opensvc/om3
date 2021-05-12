package create

import (
	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/path"
)

func submit(data map[string]interface{}, c *client.T) error {
	req := c.NewPostObjectCreate()
	//req.Restore = true
	req.Data = data
	if _, err := req.Do(); err != nil {
		return err
	}
	return nil
}

func LocalEmpty(p path.T) error {
	o := object.NewFromPath(p)
	return o.(object.Configurer).Config().Commit()
}

/*
Copyright © 2021 OPENSVC SAS <contact@opensvc.com>

This program is free software; you can redistribute it and/or
modify it under the terms of the GNU General Public License
as published by the Free Software Foundation; either version 2
of the License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU Lesser General Public License
along with this program. If not, see <http://www.gnu.org/licenses/>.
*/

// Package cmd defines the opensvc command line actions and options.
package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/render"
	"opensvc.com/opensvc/core/render/cluster"
)

// daemonStatusCmd represents the daemonStatus command
var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Print the cluster status",
	Run: func(cmd *cobra.Command, args []string) {
		monitor()
	},
}

func init() {
	daemonCmd.AddCommand(daemonStatusCmd)
	daemonStatusCmd.Flags().StringVar(&formatFlag, "format", "auto", "output format json|flat|auto (default is auto)")
}

func monitor() {
	render.SetColor(colorFlag)
	api := client.New()
	data, err := api.DaemonStatus()
	if err != nil {
		return
	}
	var b []byte
	b, err = json.MarshalIndent(data, "", "    ")

	switch formatFlag {
	case "flat":
	case "flat_json":
	case "json":
		fmt.Println(string(b))
	default:
		cluster.Render(
			cluster.Data{Current: data},
			cluster.Options{},
		)
	}

}

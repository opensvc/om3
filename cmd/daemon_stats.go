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
)

// daemonStatsCmd represents the daemonStats command
var daemonStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Print the resource usage statistics.",
	Run: func(cmd *cobra.Command, args []string) {
		daemonStats()
	},
}

func init() {
	daemonCmd.AddCommand(daemonStatsCmd)
	daemonStatsCmd.Flags().StringVar(&formatFlag, "format", "auto", "output format json|flat|auto (default is auto)")
}

func daemonStats() {
	render.SetColor(colorFlag)
	api := client.New()
	c := client.NewDaemonStatsCmdConfig()
	data, err := api.DaemonStats(*c)
	if err != nil {
		return
	}
	var b []byte
	b, err = json.MarshalIndent(data, "", "    ")
	fmt.Println(string(b))
}

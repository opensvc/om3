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
	"os"

	"github.com/spf13/cobra"
	"opensvc.com/opensvc/core/client"
	"opensvc.com/opensvc/core/cluster"
	"opensvc.com/opensvc/core/output"
)

// daemonStatsCmd represents the daemonStats command
var daemonStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Print the resource usage statistics.",
	Run:   daemonStatsCmdRun,
}

func init() {
	daemonCmd.AddCommand(daemonStatsCmd)
}

func daemonStatsCmdRun(cmd *cobra.Command, args []string) {
	daemonStats()
}

func daemonStats() {
	var (
		api  client.API
		err  error
		b    []byte
		data cluster.Stats
	)
	api, err = client.New()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	handle := api.NewGetDaemonStats()
	b, err = handle.Do()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	data, err = parseDaemonStats(b)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Print(output.Switch(formatFlag, colorFlag, data, nil))
}

func parseDaemonStats(b []byte) (cluster.Stats, error) {
	type (
		nodeData struct {
			Status int               `json:"status"`
			Data   cluster.NodeStats `json:"data"`
		}
		responseType struct {
			Status int                 `json:"status"`
			Nodes  map[string]nodeData `json:"nodes"`
		}
	)
	var t responseType
	ds := make(cluster.Stats)
	err := json.Unmarshal(b, &t)
	if err != nil {
		return ds, err
	}
	for k, v := range t.Nodes {
		ds[k] = v.Data
	}
	return ds, nil
}

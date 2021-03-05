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
	"github.com/spf13/cobra"

	"opensvc.com/opensvc/core/entrypoints/monitor"
)

var (
	daemonStatusWatchFlag    bool
	daemonStatusSelectorFlag string
)

// daemonStatusCmd represents the daemonStatus command
var daemonStatusCmd = &cobra.Command{
	Use:     "status",
	Short:   "Print the cluster status",
	Long:    monitor.CmdLong,
	Aliases: []string{"statu"},
	Run:     daemonStatusCmdRun,
}

func init() {
	daemonCmd.AddCommand(daemonStatusCmd)
	daemonStatusCmd.Flags().BoolVarP(&daemonStatusWatchFlag, "watch", "w", false, "Watch the monitor changes")
	daemonStatusCmd.Flags().StringVarP(&daemonStatusSelectorFlag, "selector", "s", "**", "Select opensvc objects (ex: **/db*,*/svc/db*)")
}

func daemonStatusCmdRun(cmd *cobra.Command, args []string) {
	m := monitor.New()
	m.SetWatch(daemonStatusWatchFlag)
	m.SetColor(colorFlag)
	m.SetFormat(formatFlag)
	m.SetServer(serverFlag)
	m.SetSelector(daemonStatusSelectorFlag)
	m.Do()
}

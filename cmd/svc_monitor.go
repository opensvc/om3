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

var svcMonitorWatchFlag bool

// svcStatusCmd represents the svcStatus command
var svcMonitorCmd = &cobra.Command{
	Use:     "monitor",
	Aliases: []string{"mon", "moni", "monit", "monito"},
	Short:   "Print selected service and instance status summary",
	Long:    monitor.CmdLong,
	Run:     svcMonitorCmdRun,
}

func init() {
	svcCmd.AddCommand(svcMonitorCmd)
	svcMonitorCmd.Flags().BoolVarP(&svcMonitorWatchFlag, "watch", "w", false, "Watch the monitor changes")
}

func svcMonitorCmdRun(cmd *cobra.Command, args []string) {
	selector := mergeSelector(svcSelectorFlag)
	m := monitor.New()
	m.SetWatch(svcMonitorWatchFlag)
	m.SetColor(colorFlag)
	m.SetFormat(formatFlag)
	m.SetSelector(selector)
	m.SetSections([]string{"objects"})
	m.Do()
}

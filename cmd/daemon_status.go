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
	optDaemonStatusWatch bool
)

// daemonStatusCmd represents the daemonStatus command
var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Print the cluster status",
	Run: func(cmd *cobra.Command, args []string) {
		monitor.Do(optDaemonStatusWatch, colorFlag, formatFlag)
	},
}

func init() {
	daemonCmd.AddCommand(daemonStatusCmd)
	daemonStatusCmd.Flags().StringVar(&formatFlag, "format", "auto", "output format json|flat|auto (default is auto)")
	daemonStatusCmd.Flags().BoolVarP(&optDaemonStatusWatch, "watch", "w", false, "Watch the monitor changes")
}

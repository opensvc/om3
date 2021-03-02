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

	"opensvc.com/opensvc/core/object"
)

// svcStatusCmd represents the svcStatus command
var svcStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Print selected service and instance status",
	Long: `Resources Flags:

(1) R   Running,           . Not Running
(2) M   Monitored,         . Not Monitored
(3) D   Disabled,          . Enabled
(4) O   Optional,          . Not Optional
(5) E   Encap,             . Not Encap
(6) P   Not Provisioned,   . Provisioned
(7) S   Standby,           . Not Standby
(8) <n> Remaining Restart, + if more than 10,   . No Restart

`,
	Run: svcStatusCmdRun,
}

func init() {
	svcCmd.AddCommand(svcStatusCmd)
}

func svcStatusCmdRun(cmd *cobra.Command, args []string) {
	selector := mergeSelector(svcSelectorFlag)
	object.NewSelection(selector).Action("PrintStatus")
}

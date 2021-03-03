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
	"opensvc.com/opensvc/core/entrypoints/action"
)

var (
	svcUnfreezeNodeFlag  string
	svcUnfreezeLocalFlag bool
	svcUnfreezeWatchFlag bool
)

var svcUnfreezeCmd = &cobra.Command{
	Use:     "unfreeze",
	Aliases: []string{"thaw"},
	Short:   "Unfreeze the selected objects.",
	Run:     svcUnfreezeCmdRun,
}

func init() {
	svcCmd.AddCommand(svcUnfreezeCmd)
	svcUnfreezeCmd.Flags().BoolVarP(&svcUnfreezeLocalFlag, "local", "", false, "Unfreeze inline the selected local instances.")
	svcUnfreezeCmd.Flags().BoolVarP(&svcUnfreezeWatchFlag, "watch", "w", false, "Watch the monitor changes")
}

func svcUnfreezeCmdRun(cmd *cobra.Command, args []string) {
	action.ObjectAction{
		ObjectSelector: mergeSelector(svcSelectorFlag, "svc", ""),
		NodeSelector:   svcUnfreezeNodeFlag,
		Action:         "freeze",
		Method:         "Unfreeze",
		Target:         "thawed",
		Watch:          svcUnfreezeWatchFlag,
		Format:         formatFlag,
		Color:          colorFlag,
	}.Do()
}

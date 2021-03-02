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
	"opensvc.com/opensvc/core/entrypoints"
)

var (
	svcFreezeNodeFlag  string
	svcFreezeLocalFlag bool
	svcFreezeWatchFlag bool
)

var svcFreezeCmd = &cobra.Command{
	Use:   "freeze",
	Short: "Freeze the selected objects.",
	Run:   svcFreezeCmdRun,
}

func init() {
	svcCmd.AddCommand(svcFreezeCmd)
	svcFreezeCmd.Flags().BoolVarP(&svcFreezeLocalFlag, "local", "", false, "Freeze inline the selected local instances.")
	svcFreezeCmd.Flags().BoolVarP(&svcFreezeWatchFlag, "watch", "w", false, "Watch the monitor changes")
}

func svcFreezeCmdRun(cmd *cobra.Command, args []string) {
	entrypoints.Action{
		ObjectSelector: mergeSelector(svcSelectorFlag, "svc", ""),
		NodeSelector:   svcFreezeNodeFlag,
		Action:         "freeze",
		Method:         "Freeze",
		Target:         "frozen",
		Watch:          svcFreezeWatchFlag,
		Format:         formatFlag,
		Color:          colorFlag,
	}.Do()
}

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
	"fmt"

	"github.com/spf13/cobra"

	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/output"
)

// svcStatusCmd represents the svcStatus command
var svcLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "Print the selected objects.",
	Run: func(cmd *cobra.Command, args []string) {
		selector := mergeSelector(svcSelectorFlag)
		results := object.NewSelection(selector).Action("List")
		data := make([]string, 0)
		for _, r := range results {
			buff, ok := r.Data.(string)
			if !ok {
				continue
			}
			data = append(data, buff)
		}
		human := func() string {
			s := ""
			for _, r := range data {
				s += r + "\n"
			}
			return s
		}
		s := output.Switch(formatFlag, colorFlag, data, human)
		fmt.Print(s)
	},
}

func init() {
	svcCmd.AddCommand(svcLsCmd)
}

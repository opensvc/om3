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
)

var volSelectorFlag string

// volCmd represents the vol command
var volCmd = &cobra.Command{
	Use:   "vol",
	Short: "Manage volumes",
	Long: `A volume is a persistent data provider.
	
A volume is made of disk, fs and sync resources. It is created by a pool,
to satisfy a demand from a volume resource in a service.

Volumes and their subdirectories can be mounted inside containers.

A volume can host cfg and sec keys projections.
`,
}

func init() {
	rootCmd.AddCommand(volCmd)
	volCmd.PersistentFlags().StringVarP(&volSelectorFlag, "selector", "s", "", "The name of the object to select")
}

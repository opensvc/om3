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

// svcCmd represents the svc command
var svcCmd = &cobra.Command{
	Use:   "svc",
	Short: "Manage services",
	Long: `Services are objects serving applications. They can use 
support objects like volumes, secrets and configmaps to have a separate
lifecycle or to abstract cluster-specific knowledge.`,
}

var svcSelector string

func init() {
	rootCmd.AddCommand(svcCmd)
	//svcCmd.PersistentFlags().String("namespace", "root", "The namespace to select")
	//svcCmd.PersistentFlags().String("name", "", "The name of the object to select")
	svcCmd.PersistentFlags().StringVarP(&svcSelector, "selector", "s", "", "The name of the object to select")
	//svcCmd.PersistentFlags().Bool("local", false, "Run action on the local instance of the selected objects")
	//svcCmd.PersistentFlags().String("node", "", "Run action on the instance of the selected objects on <node>")
}

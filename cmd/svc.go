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
)

// svcCmd represents the svc command
var svcCmd = &cobra.Command{
	Use:   "svc",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("svc called")
	},
}

var svcSelector string

func init() {
	rootCmd.AddCommand(svcCmd)
	//svcCmd.PersistentFlags().String("namespace", "root", "The namespace to select")
	//svcCmd.PersistentFlags().String("name", "", "The name of the object to select")
	svcCmd.PersistentFlags().StringVarP(&svcSelector, "selector", "s", "", "The name of the object to select")
	//svcCmd.PersistentFlags().Bool("local", false, "Run action on the local instance of the selected objects")
	//svcCmd.PersistentFlags().String("node", "", "Run action on the instance of the selected objects on <node>")
	// svcCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

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

	"opensvc.com/opensvc/core/client"
)

// daemonStatusCmd represents the daemonStatus command
var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		monitor()
	},
}

func init() {
	daemonCmd.AddCommand(daemonStatusCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// svcStatusCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// svcStatusCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func monitor() {
	api := client.New(client.Config{
		URL: "raw://opt/opensvc/var/lsnr/lsnr.sock",
	})
	//requester := client.New(client.Config{
	//	URL: "https://127.0.0.1:1215"
	//	InsecureSkipVerify: true, // get from config
	//})
	data, err := api.DaemonStatus()
	if err != nil {
		return
	}
	fmt.Println(data)
}

package arraysymmetrix

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test(t *testing.T) {
	arr := New()
	arr.SetName("sym")
	t.Run("symaccess list view -detail", func(t *testing.T) {
		b, err := os.ReadFile("./testdata/21-symaccess_list_view_detail")
		require.Nil(t, err)
		data, err := arr.parseSymAccessListViewDetail(b)
		require.Nil(t, err)
		b, err = json.MarshalIndent(data, "", "    ")
		require.Nil(t, err)
		fmt.Println(string(b))
	})
	t.Run("symcfg list", func(t *testing.T) {
		b, err := os.ReadFile("./testdata/02-symcfg_list")
		require.Nil(t, err)
		data, err := arr.parseSymCfgList(b)
		require.Nil(t, err)
		b, err = json.MarshalIndent(data, "", "    ")
		require.Nil(t, err)
		fmt.Println(string(b))
	})
	t.Run("symcfg list -dir all -v", func(t *testing.T) {
		b, err := os.ReadFile("./testdata/04-symcfg_list_dir_all")
		require.Nil(t, err)
		data, err := arr.parseSymCfgDirectorList(b)
		require.Nil(t, err)
		b, err = json.MarshalIndent(data, "", "    ")
		require.Nil(t, err)
		fmt.Println(string(b))
	})
	t.Run("symcfg list -rdfg all -v", func(t *testing.T) {
		b, err := os.ReadFile("./testdata/03-symcfg_list_rdfg")
		require.Nil(t, err)
		data, err := arr.parseSymCfgRDFGList(b)
		require.Nil(t, err)
		b, err = json.MarshalIndent(data, "", "    ")
		require.Nil(t, err)
		fmt.Println(string(b))
	})
	t.Run("symcfg list -pool -v", func(t *testing.T) {
		b, err := os.ReadFile("./testdata/14-symcfg_list_pool")
		require.Nil(t, err)
		data, err := arr.parseSymCfgPoolList(b)
		require.Nil(t, err)
		b, err = json.MarshalIndent(data, "", "    ")
		require.Nil(t, err)
		fmt.Println(string(b))
	})
	t.Run("symcfg list -slo -detail", func(t *testing.T) {
		b, err := os.ReadFile("./testdata/17-symcfg_list_slo_detail")
		require.Nil(t, err)
		data, err := arr.parseSymCfgSLOList(b)
		require.Nil(t, err)
		b, err = json.MarshalIndent(data, "", "    ")
		require.Nil(t, err)
		fmt.Println(string(b))
	})
	t.Run("symcfg list -srp -detail", func(t *testing.T) {
		b, err := os.ReadFile("./testdata/16-symcfg_list_srp_detail")
		require.Nil(t, err)
		data, err := arr.parseSymCfgSRPList(b)
		require.Nil(t, err)
		b, err = json.MarshalIndent(data, "", "    ")
		require.Nil(t, err)
		fmt.Println(string(b))
	})
	t.Run("symdisk list -dskgroup_summary", func(t *testing.T) {
		b, err := os.ReadFile("./testdata/13-symdisk_list_dskgrp_summary")
		require.Nil(t, err)
		data, err := arr.parseSymDiskListDiskGroupSummary(b)
		require.Nil(t, err)
		b, err = json.MarshalIndent(data, "", "    ")
		require.Nil(t, err)
		fmt.Println(string(b))
	})
	t.Run("symdev show xxx", func(t *testing.T) {
		b, err := os.ReadFile("./testdata/06-symdev_show_wwn")
		require.Nil(t, err)
		data, err := arr.parseSymDevShow(b)
		require.Nil(t, err)
		b, err = json.MarshalIndent(data, "", "    ")
		require.Nil(t, err)
		fmt.Println(string(b))
	})
	t.Run("symdev list", func(t *testing.T) {
		b, err := os.ReadFile("./testdata/05-symdev_list")
		require.Nil(t, err)
		data, err := arr.parseSymDevList(b)
		require.Nil(t, err)
		b, err = json.MarshalIndent(data, "", "    ")
		require.Nil(t, err)
		fmt.Println(string(b))
	})
	t.Run("symsg list", func(t *testing.T) {
		b, err := os.ReadFile("./testdata/01-symsg_list")
		require.Nil(t, err)
		data, err := arr.parseSymSGList(b)
		require.Nil(t, err)
		b, err = json.MarshalIndent(data, "", "    ")
		require.Nil(t, err)
		fmt.Println(string(b))
	})
	t.Run("symdev create -tdev", func(t *testing.T) {
		b, err := os.ReadFile("./testdata/22-symdev_create_tdev")
		require.Nil(t, err)
		data, err := arr.getDevsFromCreateThinDevOutput(b)
		require.Nil(t, err)
		require.Equal(t, data, []string{"0DCD9"})
	})
	t.Run("symcli", func(t *testing.T) {
		b, err := os.ReadFile("./testdata/25-symcli")
		require.Nil(t, err)
		major, minor, err := arr.parseSymcliVersion(b)
		require.Nil(t, err)
		require.Equal(t, 10, major)
		require.Equal(t, 1, minor)
	})
}

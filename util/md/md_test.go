package md

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMD(t *testing.T) {
	blbidOutput := `/dev/sdo: UUID="cb312cd6-bb35-4163-37a5-82650d46acbf" UUID_SUB="62a7ebe8-7dde-864e-0c5b-f1f60720ba27" LABEL="qarh9c19n1:c19mdadm.disk.11" TYPE="linux_raid_member"
/dev/md127: UUID="bbb9d530-2d3a-bf14-be50-6cba3debc489" UUID_SUB="e2ea7f57-456e-11bf-3467-87b73d8f63ae" LABEL="qarh9c19n1:c19mdadm.disk.12" TYPE="linux_raid_member"
/dev/mapper/36001405ac42630253a64595b612c878c: UUID="736f5713-9405-42de-de5d-b10c9ac1667f" UUID_SUB="d32c27b1-602e-aeb3-3491-ea3f9cea11c9" LABEL="qarh9c19n1:c19mdadm.disk.10" TYPE="linux_raid_member"
/dev/sdm: UUID="8b003c2d-50fa-2679-d1e9-2fed607e7821" UUID_SUB="f29097b8-cac5-7bde-9fa6-7ae1fcb4e1b8" LABEL="qarh9c19n1:c19mdadm.disk.10" TYPE="linux_raid_member"
/dev/mapper/36001405f26424345db2471786803a34f: UUID="736f5713-9405-42de-de5d-b10c9ac1667f" UUID_SUB="77985356-cbad-a830-e87f-16d89cd68799" LABEL="qarh9c19n1:c19mdadm.disk.10" TYPE="linux_raid_member"
/dev/sdk: UUID="8b003c2d-50fa-2679-d1e9-2fed607e7821" UUID_SUB="74520ba8-ef23-dcdc-88bc-686768caf56a" LABEL="qarh9c19n1:c19mdadm.disk.10" TYPE="linux_raid_member"
/dev/sdq: UUID="cb312cd6-bb35-4163-37a5-82650d46acbf" UUID_SUB="c8de97cf-8fdb-f6ef-ed09-d4fbeeb9a87a" LABEL="qarh9c19n1:c19mdadm.disk.11" TYPE="linux_raid_member"
/dev/sdg: UUID="8b003c2d-50fa-2679-d1e9-2fed607e7821" UUID_SUB="74520ba8-ef23-dcdc-88bc-686768caf56a" LABEL="qarh9c19n1:c19mdadm.disk.10" TYPE="linux_raid_member"
/dev/sdn: UUID="cb312cd6-bb35-4163-37a5-82650d46acbf" UUID_SUB="c8de97cf-8fdb-f6ef-ed09-d4fbeeb9a87a" LABEL="qarh9c19n1:c19mdadm.disk.11" TYPE="linux_raid_member"
/dev/md126: UUID="bbb9d530-2d3a-bf14-be50-6cba3debc489" UUID_SUB="116cc327-ec3d-9626-db48-1fb4dcdf9fed" LABEL="qarh9c19n1:c19mdadm.disk.12" TYPE="linux_raid_member"
/dev/mapper/36001405c07bfde2b6b143129ffbb65b5: UUID="808a2785-6b4e-2147-7eb9-fb04df6b380a" UUID_SUB="5bffef82-b888-3548-c8be-08d4c7e97c67" LABEL="qarh9c19n1:c19mdadm.disk.11" TYPE="linux_raid_member"
/dev/sdl: UUID="cb312cd6-bb35-4163-37a5-82650d46acbf" UUID_SUB="62a7ebe8-7dde-864e-0c5b-f1f60720ba27" LABEL="qarh9c19n1:c19mdadm.disk.11" TYPE="linux_raid_member"
/dev/sdj: UUID="8b003c2d-50fa-2679-d1e9-2fed607e7821" UUID_SUB="f29097b8-cac5-7bde-9fa6-7ae1fcb4e1b8" LABEL="qarh9c19n1:c19mdadm.disk.10" TYPE="linux_raid_member"
/dev/mapper/360014052b1d369de466462cad34e9741: UUID="808a2785-6b4e-2147-7eb9-fb04df6b380a" UUID_SUB="0b512e4a-7126-4891-b881-9e0d6bbdebda" LABEL="qarh9c19n1:c19mdadm.disk.11" TYPE="linux_raid_member"`

	type (
		testCase struct {
			name string
			uuid string

			// expectations
			found bool
			pathL []string
		}
	)

	cases := map[string]testCase{
		"no matching name, no matching uuid": {name: "c19mdadm.disk.a15", uuid: "808a2785:6b4e2147:7eb9fb04:9ac16688",
			found: false},

		"matching name": {name: "c19mdadm.disk.10", uuid: "found from name...",
			found: true,
			pathL: []string{
				"/dev/mapper/36001405ac42630253a64595b612c878c",
				"/dev/mapper/36001405f26424345db2471786803a34f",
				"/dev/sdm", "/dev/sdk", "/dev/sdj", "/dev/sdg"}},

		"matching uuid only": {
			// empty name, can only match by uuid,
			// but empty name for md is not expected
			name: "",
			uuid: "736f5713:940542de:de5db10c:9ac1667f",

			found: true,
			pathL: []string{
				"/dev/mapper/36001405ac42630253a64595b612c878c",
				"/dev/mapper/36001405f26424345db2471786803a34f",
			},
		},

		"matching uuid1 & name1": {name: "c19mdadm.disk.10", uuid: "736f5713:940542de:de5db10c:9ac1667f",
			found: true,
			pathL: []string{
				"/dev/mapper/36001405ac42630253a64595b612c878c",
				"/dev/mapper/36001405f26424345db2471786803a34f",
				"/dev/sdm", "/dev/sdk", "/dev/sdj", "/dev/sdg"}},

		"matching uuid2 & name2": {name: "c19mdadm.disk.11", uuid: "808a2785:6b4e2147:7eb9fb04:df6b380a",
			found: true,
			pathL: []string{
				"/dev/mapper/36001405c07bfde2b6b143129ffbb65b5",
				"/dev/mapper/360014052b1d369de466462cad34e9741",
				"/dev/sdl", "/dev/sdq", "/dev/sdo", "/dev/sdn"}},

		"md on md match": {name: "c19mdadm.disk.12", uuid: "bbb9d530:2d3abf14:be506cba:3debc489",
			found: true,
			pathL: []string{"/dev/md127", "/dev/md126"}},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			res := T{name: tc.name, uuid: tc.uuid}
			t.Logf("checking %s", res)
			found := res.ContainsUUIDOrName(blbidOutput)
			assert.Equalf(t, tc.found, res.ContainsUUIDOrName(blbidOutput), "ContainsUUIDOrName")
			if found {
				devL := res.devsFromBlkidOutput(blbidOutput)
				assert.ElementsMatch(t, tc.pathL, devL, "devsFromBlkidOutput() expected %v, got %v for %s", tc.pathL, devL, res)
			}
		})
	}
}

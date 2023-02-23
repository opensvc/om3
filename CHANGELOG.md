# opensvc agent Changelog

## v3.0.0

### core

* **breaking change:** remove the --eval flag of the get command.

	users need to use the "eval" command instead.

* **breaking change:** remove the --unprovision flag of the delete command.

	users need to use the "unprovision && delete" sequence instead.

* **breaking change:** command flags that accept a duration now require a unit.

	change --waitlock=60 to --waitlock=1m
	change --time=10 to --time=10s

* **breaking change:** drop support for deprecated driver group names:

	drbd: disk.drbd
	vdisk: disk.vdisk
	vmdg: disk.ldom
	pool: disk.zpool
	zpool: disk.zpool
	loop: disk.loop
	md: disk.md
	zvol: disk.zvol
	lv: disk.lv
	raw: disk.raw
	vxdg: disk.vxdg
	vxvol: disk.vxvol

For example, a [md#1] section needs reformatting as:

	[disk#1]
	type = md

* **breaking change:** stop matching DEFAULT.<string> for "<string>:" object selector expressions. Match only sections basename (like in [<basename>#<index>]).

* **breaking change:** drop backward compatibility for the always_on=<nodes> keyword.

* New fields in print schedule json format: node, path

* **breaking change:** new cgroup layout. The previous organization allowed conflicts between different object types, and was hard to read.

* Change the "print status" instance-level errors and warnings (to no-whitespace words):

	part provisioned => mix-provisioned
	not provisioned => not-provisioned
	node frozen => node-frozen
	constraints violation => constraints-violation
	daemon down => daemon-down

* **breaking change:** Rename the create --config flag to --from, and merge --template into --from.

	Support the following template selector syntaxes:

		--from 111
		--from template://111
		--from "template://my tmpl 111"

*  **breaking change:** Rename commands

	node scan capabilities => node capabilities scan
	node print capabilities => node capabilities list


### driver fs

* **breaking change:** keywords `size` and `vg` are no longer supported, and a logical volume can no longer be created by the fs provisioner. Use a proper disk.lv to do that.

### driver sync

* **breaking change:** The "sync drp" action is removed. Use "sync update --target drpnodes" instead.

* **breaking change:** The "sync nodes" action is removed. Use "sync update --target nodes" instead.

* The "sync all" action is deprecated. Use "sync update" with no --target flag instead.

* The "sync full" and "sync update" now both accept a "--target nodes|drpnodes|node_selector_expr" flag

### driver app

* **breaking change:** keyword `environment` now keep var name unchanged (respect mixedCase)
  
        environment = Foo=one bar=2 Z=u
        =>
        Foo=one     was previsouly changed to FOO=one
        bar=2       was previsouly changed to BAR=2
        Zoo=u       was previously changed to ZOO=u

* **breaking change:** Remove support on some deprecated env var

  Following env var are not anymore added to process env var during actions
  * OPENSVC_SVCNAME
  * OPENSVC_SVC_ID

* **breaking change:** Fix OPENSVC_ID var value on app resources

  In the app drivers, the object id is now exposed as the OPENSVC_ID environment variable.
  In 2.1, OPENSVC_ID was set to the object path name (for example "foo" from "test/svc/foo").
  
* The kill keyword is removed. The default behaviour is now to kill all processes with the matching OPENSVC_ID and OPENSVC_RID variables in their environment.
  In 2.1 the default behaviour was to try to identify the topmost process matching the start command in the process command line, and having the matching env vars, but this guess is not accurate enough as processes can change their cmdline via PRCTL or via execv.
  If the new behaviour is not acceptable, users can provide their own stopper via the "stop" keyword.


### daemon

* **breaking change:** switch to time.Time in RFC3389 format in all internal and exposed data

	A unix timestamp was previously used, but it was tedious for users to understand the json data. And go makes the time.Time type unavoidable anyway, so the performance argument for timestamps doesn't stand anymore.

* **breaking change:** change instance status resources type

	In 2.1 the instance status resources was a dict of rid to exposed status
  	now it is a list of exposed status, rid is now a property of exposed status

* **breaking change:** replace relay heartbeat secret keyword with username and password.

	The password value is the sec object path containing the actual relay password encoded in the password key.


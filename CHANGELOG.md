# opensvc agent Changelog

## v3.0.0

### core

* **breaking change:** Drop the constraints svc keyword. Use host label selectors instead.

* **breaking change:** The "om daemon dns dump" command is deprecated (with backward compatibility) in favour of "om dns dump". As a consequence, the "dns" object path, if used, is now masked. The root/svc/dns identifier can still be used to help with the transition to a new object name.

* The set and unset commands are superseded by update --set ... --unset ... --delete. This new command allow to have a single commit for different kind of changes. The set and unset commands are now hidden so users don't get tempted to use them anymore.

* **breaking change:** set/unset/get/eval now need --local to operate on the local node without api calls.

* New placement policy `last start`. Use the mtime of `<objvar>/last_start` as the candidate sort key. More recent has higher priority.

* **breaking change:** Drop the --dry-run flag.

* **breaking change:** Drop the `default_mon_format` node keyword. It should be a user-level setting, not a node-level config.

* **breaking change:** Drop the `reboot` node command and associated keywords: `reboot.schedule`, `reboot.pre`, `reboot.once`, `reboot.blocking_pre`

* **breaking change:** Drop the `rotate root password` node command and associated keywords: `rotate_root_pw.schedule`

* **breaking change:** Drop the `pushstats` node command and associated keywords: `stats_collection.schedule`, `stats.schedule`, `stats.disable`

* **breaking change:** Deny object path name and namespaces longer than 63 character.

* **breaking change:** replace the --debug flag with --log debug|info|warn|error|fatal|panic

* Add --quiet to disable both the progress renderer and the console logging

* **breaking change:** remove the --eval flag of the get command.

	users need to use the "eval" command instead.

* **breaking change:** remove the --unprovision flag of the delete command.

	users need to use the "unprovision && delete" sequence instead.

* **breaking change:** Remove the --rid flag of the delete command.

  Users can use the "unset --section <name>" command instead.

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
	daemon down => daemon-down

* **breaking change:** Rename the create --config flag to --from, and merge --template into --from.

	Support the following template selector syntaxes:

		--from 111
		--from template://111
		--from "template://my tmpl 111"

*  **breaking change:** Rename commands

	node scan capabilities => node capabilities scan
	node print capabilities => node capabilities list


*  **breaking change:** In previous releases, om node get --kw node.env returned the keyword's raw string value from cluster.conf if it is not defined in node.conf. In this release, this get command returns the empty string. The eval command is unchanged though: it still falls back to cluster.conf.

	In v2:

	=============== =============== =============== =============== ================ =================
	 node.conf       cluster.conf    om node get     om node eval    om cluster get   om cluster eval 
	=============== =============== =============== =============== ================ =================
	 fr              kr              fr              fr              kr               kr              
	 fr              -               fr              fr              -                -               
	 -               kr              kr              kr              kr               kr              
	 -               -               -               -               -                -               
	=============== =============== =============== =============== ================ =================


	In v3:

	=============== =============== =============== =============== ================ =================
	 node.conf       cluster.conf    om node get     om node eval    om cluster get   om cluster eval 
	=============== =============== =============== =============== ================ =================
	 fr              kr              fr              fr              kr               kr              
	 fr              -               fr              fr              -                -               
	 -               kr              -               kr              kr               kr              
	 -               -               -               -               -                -               
	=============== =============== =============== =============== ================ =================

*  **breaking change:** The raw protocol is dropped. `echo <json> | socat - /var/lib/opensvc/lsnr/lsnr.sock`

### objects

* **breaking change:** drop support of some DEFAULT keywords:
  * `svc_flex_cpu_low_threshold`
  * `svc_flex_cpu_high_threshold`

### commands

* **breaking change:** "om node freeze" is now local only. Use "om cluster freeze" for the orchestrated freeze of all nodes. Same applies to "unfreeze" and its hidden alias "thaw".

* **breaking change:** "om cluster abort" replaces "om node abort" to abort the pending cluster action orchestration.

* **breaking change:** "om ... set|unset" no longer accept --param and --value. Use --kw instead, which was also supported in v2.

* **breaking change:** "om node logs" now display only local logs. A new "om cluster logs" command displays all cluster nodes logs.

* "unset" now accepts "--section <name>" to remove an cluster, node or object configuration section.

* "om monitor" instance availability icons changes:

	standby down: s => x
	standby up:   S => o

### driver ip

* **breaking change:** Drop the `dns_name_suffix`, `provisioner`, `dns_update` keywords. The zone management feature of the collector will be dropped in the collector too.

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

  The following env var are not anymore added to process env var during actions:

	OPENSVC_SVCNAME
	OPENSVC_SVC_ID

* **breaking change:** Fix OPENSVC_ID var value on app resources

  In the app drivers, the object id is now exposed as the OPENSVC_ID environment variable.
  In 2.1, OPENSVC_ID was set to the object path name (for example "foo" from "test/svc/foo").
  
* The kill keyword is removed. The default behaviour is now to kill all processes with the matching OPENSVC_ID and OPENSVC_RID variables in their environment.
  In 2.1 the default behaviour was to try to identify the topmost process matching the start command in the process command line, and having the matching env vars, but this guess is not accurate enough as processes can change their cmdline via PRCTL or via execv.
  If the new behaviour is not acceptable, users can provide their own stopper via the "stop" keyword.

### object sec

* **breaking change:**  Remove the `fullpem` action. Add the `fullpem` key on `gencert` action.

### daemon

* Add a 60 seconds timeout to pre_monitor_action. The 2.1 daemon waits forever for this callout to terminate.

* Earlier local object instance orchestration after node boot

    * In 2.1 local object instance orchestration waits for all local object instances boot action done
    * Now object instance <foo@localhost> orchestration only waits for <foo@localhost> boot action completed. Each instance has a last boot id.

* **breaking change:** switch to time.Time in RFC3389 format in all internal and exposed data

	A unix timestamp was previously used, but it was tedious for users to understand the json data. And go makes the time.Time type unavoidable anyway, so the performance argument for timestamps doesn't stand anymore.

* **breaking change:** change instance status resources type

	In 2.1 the instance status resources was a dict of rid to exposed status
  	now it is a list of exposed status, rid is now a property of exposed status

* **breaking change:** replace relay heartbeat secret keyword with username and password.

	The password value is the sec object path containing the actual relay password encoded in the password key.

#### logging

* **breaking change** OpenSVC no longer logs to private log files. It logs to journald instead. So the log entries attributes are indexed and can be used to filter logs very fast. Use `journalctl _COMM=om3` to extract all OpenSVC logs. Add OBJ_PATH=svc1 to filter only logs relevant to an object.

* The **sc** log entries attribute is replaced with **origin=daemon/scheduler**.

* The **origin=daemon** log entries attribute is replaced with **origin=daemon/monitor**

### cluster config
#### arbitrator

* The new keyword **uri** replaces **name**.

* When the uri scheme is http or https, the vote checker is based on a GET request, else it is based on a TCP connect.
  When the port is not specified in a TCP connect uri, the 1215 port is implied.

  Examples:

      uri = https://arbitrator.opensvc.com/check
      uri = arbitrator1.opensvc.com:1215
      uri = arbitrator1.opensvc.com               # implicitly port 1215

* The new keyword **insecure** disables the server certificate validation when the uri scheme is https, the default is false.

* The **name** keyword is deprecated. Aliased to **uri** to ease transition.

* The **timeout** keyword is removed to avoid users setting a value greater than the ready period,
  which would let the service double start before the end of the vote.
  The internal timeout value is now set to a third of the ready period.

* The **secret** keyword is now ignored.

#### cluster section

##### cluster.name

* **breaking change:** keyword `cluster.name` has no default value. It has
  previously the default value *default*. Now daemon startup will automatically
  replace undefined cluster.name with a random value.

## upgrade from b2.1
### cluster config

* Need explicit cluster.name because of v3 random cluster name:

		# Ensure cluster.name is defined before upgrade to v3
		om cluster set --kw cluster.name=$(om cluster eval --kw cluster.name)

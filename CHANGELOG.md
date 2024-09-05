# OpenSVC agent v3 Changelog

## Breaking Changes

### Core

* Switch to RFC3389 time format in all internal and exposed data.

	A unix timestamp was previously used, but it was tedious for users to understand the json data. And go makes the time.Time type unavoidable anyway, so the performance argument for timestamps doesn't stand anymore.

* The keyword `cluster.name` has no default value.

    In v2.1, the default cluster name was `default`.
    
    In v3, the startup will automatically replace the undefined `cluster.name` with a random human-readable value.

* The keyword `cluster.name` is no longer scopable.

* Drop the `constraints` svc keyword. Use host label selectors instead.

* The "om daemon dns dump" command is deprecated (with backward compatibility) in favour of "om dns dump". As a consequence, the "dns" object path, if used, is now masked. The root/svc/dns identifier can still be used to help with the transition to a new object name.

* `set`, `unset`, `get`, `eval` now need `--local` to operate on the local node without api calls.

* Drop the --dry-run flag.

* Drop the `default_mon_format` node keyword. It should be a user-level setting, not a node-level config.

* Drop the `reboot` node command and associated keywords: `reboot.schedule`, `reboot.pre`, `reboot.once`, `reboot.blocking_pre`

* Drop the `rotate root password` node command and associated keywords: `rotate_root_pw.schedule`

* Drop the `pushstats` node command and associated keywords: `stats_collection.schedule`, `stats.schedule`, `stats.disable`

* Deny object path name and namespaces longer than 63 character.

* Replace the `--debug` flag with --log debug|info|warn|error|fatal|panic

* Remove the `--eval` flag of the get command.

	Users need to use the `eval` command instead.

* Remove the `--unprovision` flag of the `delete` command.

	Users need to use the `unprovision` and `delete` sequence instead, or `purge`.

* Remove the `--rid` flag of the `delete` command.

  Users can use the `unset --section <name>` command instead.

* Command flags that accept a duration now require a unit.

	change --waitlock=60 to --waitlock=1m
	change --time=10 to --time=10s

* Drop support for driver group names already deprecated in v2.1:

    ```
	drbd   disk.drbd
	vdisk  disk.vdisk
	vmdg   disk.ldom
	pool   disk.zpool
	zpool  disk.zpool
	loop   disk.loop
	md     disk.md
	zvol   disk.zvol
	lv     disk.lv
	raw    disk.raw
	vxdg   disk.vxdg
	vxvol  disk.vxvol
    ```

    For example, a [md#1] section needs reformatting as:

      [disk#1]
      type = md

* Stop matching `DEFAULT.foo` with the `om foo: ls`.

    Match only objects with `foo` as a section basename (eg. `[foo#bar]`).

* Drop backward compatibility for the `always_on=<nodes>` keyword.

    The `standby=true` keyword is the target since v2.1.

* New cgroup layout.

    The previous layout allowed conflicts between different object types (eg. `vol` and `svc`).

* Change the `print status` instance-level errors and warnings (to no-whitespace words):

    ```
	part provisioned  ->  mix-provisioned
	not provisioned   ->  not-provisioned
	node frozen       ->  node-frozen
	daemon down       ->  daemon-down
    ```

* Simplify the `om create` flags
 
    ```
    --config           ->  --from
    --template         ->  --from
    ```

	Support the following template selector syntaxes:

    ```
    --from 111
    --from template://111
    --from "template://my tmpl 111"
    ```

*  Rename commands

    ```
	node scan capabilities   ->  node capabilities scan
	node print capabilities  ->  node capabilities list
    ```


*  In previous releases, `om node get --kw node.env` returned the keyword's raw string value from `cluster.conf` if it is not defined in `node.conf`.

    In this release, this command returns the empty string. The `eval` command is unchanged though: it still falls back to `cluster.conf`.

	In v2:
    ```
	node.conf cluster.conf om node get om node eval om cluster get om cluster eval 
	--------- ------------ ----------- ------------ -------------- ---------------
	fr        kr           fr          fr           kr             kr              
	fr        -            fr          fr           -              -               
	-         kr           kr          kr           kr             kr              
	-         -            -           -            -              -               
    ```


	In v3:

    ```
	node.conf cluster.conf om node get om node eval om cluster get om cluster eval 
	--------- ------------ ----------- ------------ -------------- ---------------
	fr        kr           fr          fr           kr             kr              
	fr        -            fr          fr           -              -               
	-         kr           -           kr           kr             kr              
	-         -            -           -            -              -               
    ```

* The `raw` jsonrpc protocol API is dropped.

    For example, this v2.1 API call is no longer supported:
    ```
    echo '{"action": "daemon_status"}' | socat - /var/lib/opensvc/lsnr/lsnr.sock
    ```
    
    To keep using a root Unix Socket in v3, you can switch to:
    ```
    curl -o- -X GET -H "Content-Type: application/json" --unix-socket /var/lib/opensvc/lsnr/http.sock http://localhost/daemon/status
    ```

* Propagate the task run and sync errors to a non-zero exitcode.

    The `task` and `sync` resources are now `optional=false` by default, but their status is not aggregated in the instance availability status whatever the `optional` value. Errors in the run produce a non-zero exitcode if optional=false, zero if optional=true.

* Drop support of some `DEFAULT` section keywords:

  * `svc_flex_cpu_low_threshold`
  * `svc_flex_cpu_high_threshold`

* Key-Value stores (cfg, sec, usr kinded objects) `change` action is no longer failing if the key does not exist. The key is added instead.

* `om node freeze` is now local only. Use `om cluster freeze` for the orchestrated freeze of all nodes. Same applies to `unfreeze` and its hidden alias `thaw`.

* `om cluster abort` replaces `om node abort` to abort the pending cluster action orchestration.

* `om ... set|unset` no longer accept ``--param`` and ``--value``. Use ``--kw`` instead, which was also supported in v2.

* `om node logs` now display only local logs. A new `om cluster logs` command displays all cluster nodes logs.

* `om <sel> unset` now accepts `--section <name>` to remove a cluster, node or object configuration section.

* `om monitor` instance availability icons changes:

    ```
	standby down: s => x
	standby up:   S => o
    ```
    
### Driver: ip

* Drop the `dns_name_suffix`, `provisioner`, `dns_update` keywords. The zone management feature of the collector will be dropped in the collector too.

### Driver: fs

* Keywords `size` and `vg` are no longer supported, and a logical volume can no longer be created by the fs provisioner. Use a proper disk.lv to do that.

### Driver: sync

* The `sync drp` action is removed. Use `sync update --target drpnodes` instead.

* The `sync nodes` action is removed. Use `sync update --target nodes` instead.

* The `sync all` action is deprecated. Use `sync update` with no `--target` flag instead.

* The `sync full` and `sync update` now both accept a `--target nodes|drpnodes|node_selector_expr` flag

### Driver: app

* The keyword `environment` now keeps the variable names unchanged and accepts mixedCase.
  
    ```
    With:
      environment = Foo=one bar=2 Z=u
      
    Foo=one     was previsouly changed to FOO=one
    bar=2       was previsouly changed to BAR=2
    Zoo=u       was previously changed to ZOO=u
    ```

* Remove support of some deprecated environment variables.

    The following variables are no longer added to process environment during actions:
    
    ```
	OPENSVC_SVCNAME
	OPENSVC_SVC_ID
    ```

* Fix `OPENSVC_ID` environment variable value in `app` resources

  In the `app` drivers, the object id is now exposed as the `OPENSVC_ID` environment variable.
  
  In 2.1, `OPENSVC_ID` was set to the object name (for example `foo` in `test/svc/foo`).
  
* The `kill` keyword is removed.

    The default behaviour is now to kill all processes with the matching `OPENSVC_ID` and `OPENSVC_RID` variables in their environment.
    
    In 2.1 the default behaviour was to try to identify the topmost process matching the start command in the process command line, and having the matching env vars, but this guess is not accurate enough as processes can change their cmdline via PRCTL or via execv.
    
    If the new behaviour is not acceptable, users can provide their own stopper via the "stop" keyword.

### Object: sec

* Remove the `fullpem` action. Add the `fullpem` key on `gencert` action.

### Logging

* OpenSVC no longer logs to private log files.

    It logs to journald instead. So the log entries attributes are indexed and can be used to filter logs very fast. Use `journalctl _COMM=om3` to extract all OpenSVC logs. Add OBJ_PATH=svc1 to filter only logs relevant to an object.

* The `sc` log entries attribute is replaced with `origin=daemon/scheduler`.

* The `origin=daemon` log entries attribute is replaced with `origin=daemon/monitor`.

### Heartbeat: relay

The v3 agent needs to address a v3 relay.

The v3 relay must have a user with the `heartbeat` grant that the client will need to use.
```
om system/usr/relayuser create --kw grant=heartbeat
om system/usr/relayuser add --key password --value $PASSWORD
```

On the cluster nodes, store the relay password in a secret:
```
om system/sec/relay-v3 create
om system/sec/relay-v3 add --key password --value $PASSWORD
```

And the heartbeat configuration:
```
[hb#1]
type = relay
relay = relay-v2
secret = 3aaf0dae606212349b7123eb8cc7e89b
```

Becomes:
```
[hb#1]
type = relay
relay = relay1-v3
username = relayuser
password = system/sec/relay-v3
```

Where the password is the value of the `þassword` key in `system/sec/relay-v3`.

### Arbitrator

* The new keyword `uri` replaces `name`.

* When the uri scheme is http or https, the vote checker is based on a GET request, else it is based on a TCP connect.
  When the port is not specified in a TCP connect uri, the 1215 port is implied.

  Examples:

      uri = https://arbitrator.opensvc.com/check
      uri = arbitrator1.opensvc.com:1215
      uri = arbitrator1.opensvc.com               # implicitly port 1215

* The new keyword `insecure` disables the server certificate validation when the uri scheme is https, the default is false.

* The `name* keyword is deprecated. Aliased to `uri` to ease transition.

* The `timeout` keyword is removed to avoid users setting a value greater than the ready period,
  which would let the service double start before the end of the vote.
  The internal timeout value is now set to a third of the ready period.

* The `secret` keyword is now ignored.

## Enhancements

### Core
    
* The `set` and `unset` commands are complemented by `update --set ... --unset ... --delete`. This new command allow to have a single commit for different kind of changes. The set and unset commands are now hidden so users don't get tempted to use them anymore.
    
* New placement policy `last start`. Use the mtime of `<objvar>/last_start` as the candidate sort key. More recent has higher priority.

* Add --quiet to disable both the progress renderer and the console logging

* New fields in print schedule json format: node, path

### Daemon

* Add a 60 seconds timeout to `pre_monitor_action`. The 2.1 daemon waits forever for this callout to terminate.

* Earlier local object instance orchestration after node boot

    In 2.1 local object instance orchestration waits for all local object instances boot action done
    
    Now object instance <foo@localhost> orchestration only waits for <foo@localhost> boot action completed. Each instance has a last boot id.

## Upgrade from b2.1

### Cluster Config

* Need to set explicitely the `cluster.name` because the v3 daemon will generate a random cluster name if none is set:

    ```
    # Ensure cluster.name is defined before upgrade to v3
    om cluster set --kw cluster.name=$(om cluster eval --kw cluster.name)
    ```


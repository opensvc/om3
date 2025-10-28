# OpenSVC Agent v3 Changelog

## Migration Prerequisites

* Upgrade to the latest v2.1 before installing v3. The preinst, postinst, prerm, postrm scripts are received patches to make the 2-to-3 transition safer:

    * [2.1-1890] Don't purge /etc/opensvc and /var/lib/opensvc on `apt [auto]remove --purge`

## Breaking Changes

### Cluster and Node Configuration

* **Hook**
    The event is no longer passed via stdin.
    The om3 daemon feeds the event as a json-formatted EVENT environment variable instead.
    The events name and payload structure changed too.
    See `om node events --help` for the list of event names.

* **Time format change:**
    OpenSVC now uses RFC3339 time format for all internal and exposed data, replacing the Unix timestamps.

* **`cluster.name` default value:**
    In v2.1, the default cluster name was `default`.
    In v3, if `cluster.name` is undefined at startup, it will be automatically replaced with a randomly generated human-readable value.

* **`cluster.name` scope:**
    This keyword is no longer scopable.

  * **`system/sec/hb` added:**
    In v2.1 `cluster.secret` keyword defines both sec and heartbeat encrypting key.
    In v3 The `system/sec/hb` secret store holds the encryption keys for heartbeat messages.
    
    The `system/sec/hb` secret store holds the following keys:
          
        |---------------|-------------------------------------------|
        | Key           | Description                               |
        |---------------|-------------------------------------------|
        | `secret`      | Active secret to encrypt heartbeat msg    |
        | `version`     | Version of the `secret`                   |
        | `alt_secret`  | Alternate secret to decrypt heartbeat msg |
        | `alt_version` | Version of the alternate secreat          |
        |---------------|-------------------------------------------|

* **`node.default_mon_format` removed:**
    It should be a user-level setting, not a node-level config.

* **`listener.openid_well_known` removed:**
    Now the **`listener.openid_issuer`** has replaced `listener.openid_well_known`. Previously, `listener.openid_well_known` was used to specify the .well-known/openid-configuration endpoint. Now, `listener.openid_issuer` defines the base URL from which the OpenID configuration is derived.
    The value is used to define the auth strategy `jwt-openid` (to verify token against the `jwks_uri` and `issuer`).
    The strategy will pickup `grants` from the claim `entitlements`.
    It is available from `GET /api/auth/info`:
        
      GET /api/auth/info
      {
        "methods": [ "openid" ],
        "openid": {
          "authority": "https://auth.example.com/realms/myrealm/",
          "client_id": "clientxxx"
        }
      }

* **`listener.openid_client_id` added:**
   The value is used to define the auth strategy `jwt-openid` (to verify token against the `aud`).
   The value is served threw the GET /api/auth/info.

* **`reboot` section removed:**
    * `reboot.schedule`
    
    * `reboot.pre`
    
    * `reboot.once`
    
    * `reboot.blocking_pre`

* **`rotate_root_pw` section removed:**
    * `rotate_root_pw.schedule`
    
* **`stats_collection` section removed:**
    * `stats_collection.schedule`
    
    * `stats.schedule`
    
    * `stats.disable`
    
### Object Configuration

* **References**

    * Drop support for arithmetic expressions in references

* **Keywords removed:**
    * `svc_flex_cpu_low_threshold`
    
    * `svc_flex_cpu_high_threshold`

    * `constraints`
        Replaced by host label selectors in `nodes`.
        Example:
        ```
        [DEFAULT]
        nodes = az=fr1 az=us1
        ```
        
    * `always_on=<nodes>`
        Replaced by `standby=true`.
        This keyword was already marked deprecated in v2.1.

* **Driver Group Names Removed:**

    Drop support for driver-group names:**

	* `drbd`
	   Replaced by `disk#foo.type=drbd`
       
	* `vdisk`
	   Replaced by `disk#foo.type=vdisk`
       
	* `vmdg`
	   Replaced by `disk#foo.type=vmdg`
       
	* `pool`
	   Replaced by `disk#foo.type=zpool`
       
	* `zpool`
	   Replaced by `disk#foo.type=zpool`
       
	* `loop`
	   Replaced by `disk#foo.type=loop`
       
	* `md`
	   Replaced by `disk#foo.type=md`
       
	* `zvol`
	   Replaced by `disk#foo.type=zvol`
       
	* `lv`
	   Replaced by `disk#foo.type=lv`
       
	* `raw`
	   Replaced by `disk#foo.type=raw`
       
	* `vxdg`
	   Replaced by `disk#foo.type=vxdg`
       
	* `vxvol`
	   Replaced by `disk#foo.type=vxvol`

    For example, a `[md#1]` section needs reformatting as:
    ```
    [disk#1]
    type = md
    ```
      
    These driver group names were already deprecated in v2.1.

### Commands

* **Configuration updates and queries use the daemon api by default:**
    `om set`, `om unset`, `om get`, `om eval` now need `--local` to operate on the local configurations without api calls.

* `om xx status`
    This command no longer accepts a selector, because:
    1/ it is documented the exitcode is the instance status so we can not be ambiguous.
    2/ it is optimized for efficiency as the daemon executes this frequently to refresh instances status data.

* **Removed:**
    * `om node reboot`

    * `om node rotate root password`

    * `om node pushstats`

	* `node scan capabilities`
        Replaced by `node capabilities scan`
        
	* `node print capabilities`
        Replaced by `node capabilities list`
        
    * `om node abort`
        Replaced by `om cluster abort` to abort the pending cluster action orchestration.

* **Moved** (with backward compatibility)
    * `om daemon status` => `om cluster status`
    * `om xx edit` => `om xx config edit`
    * `om xx set` => `om xx config update --set`
    * `om xx unset` => `om xx config update --unset`
    * `om xx delete --section` => `om xx config update --delete`
    * `om xx eval` => `om xx config eval`
    * `om xx get` => `om xx config get`
    * `om xx update` => `om xx config update`
    * `om xx validate` => `om xx config validate`
    * `om xx print config` => `om xx config show`
    * `om xx print config mtime` => `om xx config mtime`
    * `om xx print schedule` => `om xx instance schedule`
    * `om xx print status` => `om xx instance status`
    * `om xx print devs` => `om xx instance device`
    * `om xx print resinfo` => `om xx resource info list`
    * `om xx push resinfo` => `om xx resource info push`
    * `om xx clear --local` => `om xx instance clear`
    * `om xx delete --local` => `om xx instance delete`
    * `om xx provision --local` => `om xx instance provision`
    * `om xx prstart --local` => `om xx instance prstart`
    * `om xx prstop --local` => `om xx instance prstop`
    * `om xx restart --local` => `om xx instance restart`
    * `om xx run --local` => `om xx instance run`
    * `om xx shutdown --local` => `om xx instance shutdown`
    * `om xx start --local` => `om xx instance start`
    * `om xx startstandby --local` => `om xx instance startstandby`
    * `om xx stop --local` => `om xx instance stop`
    * `om xx freeze --local` => `om xx instance freeze`
    * `om xx unfreeze --local` => `om xx instance unfreeze`
    * `om xx unprovision --local` => `om xx instance unprovision`

* **Flags Added:**

    * `om <selector> instance <action> -q|--quiet`
        Don't print the logs on the console.

* **Flags Removed:**

    * `om get --eval`
        Replaced by `om <selector> config eval`

    * `om <selector> config set|unset --param ... --value`
        Replaced by `--kw`, which was also supported in v2.

    * `om <selector> instance delete --unprovision`
        Replaced by the `om <selector> instance unprovision` and `om <selector> instance delete` sequence or by `om <selector> purge`.

    * `om <selector> instance delete --rid <name>`
        Replaced by `om <selector> config unset --section <name>`.
        
    * `om <selector> <action> --dry-run`

* **Duration flags now require a unit:**
    ```
	--waitlock=60  ->  --waitlock=1m
	--time=10      ->  --time=10s
    ```
    
* **Instance state strings in `instance status`**:
    Change the instance-level errors and warnings (to no-whitespace words):
    ```
	part provisioned  ->  mix-provisioned
	not provisioned   ->  not-provisioned
	node frozen       ->  node-frozen
	daemon down       ->  unknown
    ```

* **`om <selector> config show`:**

    * Only one node config can be shown at a time
    * Only one object config can be shown at a time
    * The config can not longer be rendered as map[section]map[option]string, only text output is supported
    * These commands not longer support --impersonate and --eval. Use `config get` for those.

* **`om <path> create`:**
    * Simplify the flags
        ```
        --config           ->  --from
        --template         ->  --from
        ```

	* Support the following template selector syntaxes:
        ```
        --from 111
        --from template://111
        --from "template://my tmpl 111"
        ```

    * Drop the "--from <multi-source selector>" feature. Only single-source selector is allowed by om3.

*  **`om node get|eval`:**
    In previous releases, `om node get --kw node.env` returned the keyword's raw string value from `cluster.conf` if it is not defined in `node.conf`:

    ```
	node.conf cluster.conf om node get om node eval om cluster get om cluster eval 
	--------- ------------ ----------- ------------ -------------- ---------------
	fr        kr           fr          fr           kr             kr              
	fr        -            fr          fr           -              -               
	-         kr           kr          kr           kr             kr              
	-         -            -           -            -              -               
    ```


    In this release, this command returns the empty string. The `eval` command is unchanged though (it still falls back to `cluster.conf`):

    ```
	node.conf cluster.conf om node get om node eval om cluster get om cluster eval 
	--------- ------------ ----------- ------------ -------------- ---------------
	fr        kr           fr          fr           kr             kr              
	fr        -            fr          fr           -              -               
	-         kr           -           kr           kr             kr              
	-         -            -           -            -              -               
    ```

* **`om <selector> run` and `om <selector> sync *`:**
    Propagate the task run and sync errors to a non-zero exitcode.
    
    The `task` and `sync` resources are now `optional=false` by default, but their status is not aggregated in the instance availability status whatever the `optional` value. Errors in the run produce a non-zero exitcode if optional=false, zero if optional=true.


* **`om <kvstore> key change`:**
    This action is no longer failing if the key does not exist. The key is added instead.

* **`om <selector> instance freeze|unfreeze`:**
    If none of --slave, --slaves or --master is specified, act on both encap and host instances. The v2 agent only acted on the host instance.
    This aligns freeze|unfreeze with all other actions encap behaviour.

* **`om node freeze`:**
    This command is now local only.
    Use `om cluster freeze` for the orchestrated freeze of all nodes.
    Same applies to `om node unfreeze` and its hidden alias `om node thaw`.

* **`om node logs`:**
    Now display only local logs.
    A new `om cluster logs` command displays all cluster nodes logs.

* **`om <selector> config unset`:**
    Now accepts `--section <name>` to remove a cluster, node or object configuration section.

* **`om monitor`:**
    Instance availability icons changes:
    ```
	standby down: s => x
	standby up:   S => o
    ```
 
### Core

* **Object Names policy change:**
    Deny names and namespaces longer than 63 character.

* **Object selector policy:**
    Stop matching `DEFAULT.foo` with the `om foo: ls`.
    Match only objects with `foo` as a section basename (eg. `[foo#bar]`).

* **New cgroup layout:**

    Previous layout:
    <cgroupmnt>/opensvc.slice/<name>.slice/<rid>.slice
    <cgroupmnt>/opensvc.slice/<name>.slice/<resourcesetname>/<rid>.slice

    New layout:
    <cgroupmnt>/opensvc.slice/<kind>.<name>.slice/<rid>.slice
    <cgroupmnt>/opensvc.slice/<kind>.<name>.slice/subset.<name>/<rid>.slice

    The previous layout allowed conflicts between different object types (eg. `vol` and `svc`), and conflicts between resourceset names and rid.

* **The `raw` jsonrpc protocol socket is dropped.**
    For example, this v2.1 API call is no longer supported:
    ```
    echo '{"action": "daemon_status"}' | socat - /var/lib/opensvc/lsnr/lsnr.sock
    ```
    
    To keep using a root Unix Socket in v3, you can switch to:
    ```
    curl -o- -X GET -H "Content-Type: application/json" --unix-socket /var/lib/opensvc/lsnr/http.sock http://localhost/daemon/status
    ```

* **Change the quotation mark in the `flat` renderer keys**
    
    Use double quotes instead of quotes, as the strings in the value part already use double quotes.
    Not mixing single and double quotes helps formatting the --filter for `om node events`.

### Driver: disk.lvm

* **Removed feature:**

    The `disk.lvm` driver no longer automatically converts regular files into loopback devices for use as physical volumes (pvs). The pvs parameter must now explicitly reference actual block devices.
    `exposed_devs` can be used as replacement.

    Example:

        [disk#0]
        type = loop
        file = /files/disk0.img
        ...
        [disk#1]
        type = lvm
        vgname = vg1
        # pvs = {disk#0.file}            <- ⛔️: pvs evaluation is not a block device
        pvs = {disk#0.exposed_devs[0]}   <- ✅: pvs evaluation is /dev/loop... block device

* Drop support for `metad` era pvscan. Always use `--cache`.

### Driver: ip

* **Removed keywords:**
    * `dns_name_suffix`
    * `provisioner`
    * `dns_update`
   
* **Removed feature:**
    The interface macvtap mode is dropped from the `ip.host` driver (ie the `dev=eth0@foo` syntax).

* **Changed default:**
    The `alias` keyword default value is now `true`, activating the ip stacking behaviour.
    Setting `dev=eth0:0` still forces the address labelling mode.

* **Collector DNS zone:**
    This feature of the collector, used by the ip driver for one of its provisioning methods, is deprecated.

* **The ip.netns driver mode no longer can be set by tags**
    The `mode` keyword must be used for mode setting.

### Driver: fs

* **Removed keywords:**
    * `size`
        Configure a disk.lv resource
        
    * `vg`
        Configure a disk.lv resource
 
### Driver: sync

* **Removed actions:**
    * `om foo sync drp`
    Replaced by `om foo sync update --target drpnodes`.

    * `om foo sync nodes`
    Replaced by `om foo sync update --target nodes`.

    * `om foo sync all`
    Replaced by `om foo sync update`.

* **`sync full` and `sync update`:**
    Now both accept a `--target nodes|drpnodes|node_selector_expr` flag.

### Driver: app

* **`environment`**
    Now keeps the variable names unchanged and accepts mixedCase.
    ```
    With:
      environment = Foo=one bar=2 Z=u
      
    Foo=one     was previously changed to FOO=one
    bar=2       was previously changed to BAR=2
    Zoo=u       was previously changed to ZOO=u
    ```

* **Removed environment variables:**
    The following variables are no longer added to process environment during actions:
	* `OPENSVC_SVCNAME`
    
	* `OPENSVC_SVC_ID`

* **Changed environment variables:**
    * `OPENSVC_ID`
      In 2.1, `OPENSVC_ID` was set to the object name (for example `foo` in `test/svc/foo`).
      In v3 , `OPENSVC_ID` is set to the `DEFAULT.id` value.
  
* **Removed keywords:**
    * `kill`
        The default behaviour is now to kill all processes with the matching `OPENSVC_ID` and `OPENSVC_RID` variables in their environment.
    
        In 2.1 the default behaviour was to try to identify the topmost process matching the start command in the process command line, and having the matching env vars, but this guess is not accurate enough as processes can change their cmdline via PRCTL or via execv.
    
        If the new behaviour is not acceptable, users can provide their own stopper via the "stop" keyword.

### Object: sec

* **Removed actions:**
    * `om sec fullpem`
        The `fullpem` key is added to the sec by the `certificate create` action, in addition to `certificate`, `private_key` and `certificate_chain`.

### Logging

* **No more private log files:**
    The agent logs to journald instead. So the log entries attributes are indexed and can be used to filter logs very fast. Use `journalctl _COMM=om3` to extract all OpenSVC logs. Add OBJ_PATH=foo/svc/svc1 to filter only logs relevant to an object.

* **Log entries key changes:**
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

* The `monitor_action` now accepts a secondary action, allowing the very useful `freezestop reboot` configuration.

* The `om node update ssh keys --node=...` command is deprecated in favor of `o[mx] cluster ssh trust` (configure the trust mesh on all cluster nodes) and `o[mx] node ssh trust` (trust the node's peers)

### Daemon

* The daemon process name is changed from `/usr/bin/python3 -m opensvc.daemon` to `om daemon run`. Monitoring checks may need to adapt.

* Add a 60 seconds timeout to `pre_monitor_action`. The 2.1 daemon waits forever for this callout to terminate.

* Earlier local object instance orchestration after node boot

    In 2.1 local object instance orchestration waits for all local object instances boot action done
    
    Now object instance <foo@localhost> orchestration only waits for <foo@localhost> boot action completed. Each instance has a last boot id.

* The daemon now resets the local_expect=started instance monitor state when a sysadmin stops a resource, preventing automatic resource restarts.

    In version 2.1, a partially stopped instance caused by executing om foo stop --rid xx could inadvertently be restarted by the resource monitoring subsystem.

### sec

* Add "o[mx] rename --key old --to new" commands

### svc

* Support kvm live migration with data on `disk.disk` and `disk.drbd`. The commands to trigger a live migration are

    om svc1 switch --live
    om svc1 takeover --live

## Upgrade from b2.1

### Cluster Config

* Need to set explicitly the `cluster.name` because the v3 daemon will generate a random cluster name if none is set:

    ```
    # Ensure cluster.name is defined before upgrade to v3
    om cluster set --kw cluster.name=$(om cluster eval --kw cluster.name)
    ```

* Rename the stonith sections `cmd` option to `command`. Backward compatibility is implemented.

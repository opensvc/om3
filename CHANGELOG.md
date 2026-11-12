# OpenSVC Agent v3 Changelog

## Migration Prerequisites

* Upgrade to the latest v2.1 before installing v3. The preinst, postinst, prerm, postrm scripts are received patches to make the 2-to-3 transition safer:

    * [2.1-1890] Don't purge /etc/opensvc and /var/lib/opensvc on `apt [auto]remove --purge`

* Define `[pool#default]` with `type = directory` to keep the former default pool.

    The `default` pool a node has with no configuration is a `loop` pool, and `loop`
    is the default of the `type` keyword of a pool: a volume is a file holding a
    filesystem of its size, which bounds what it holds. A directory has no size of
    its own, so a volume could fill the disk of the node whatever a namespace
    claimed. Volumes already provisioned keep what they were made with. A node with
    no loop devices, a container say, needs a pool set for its volumes. The `fs_type`
    of a `loop` pool is `ext4` by default, `xfs` refusing a device under 300m.

* Bound the pools the namespaces claim.

    A pool claim counts the size of the volumes, so a new volume is refused in a
    claimed pool not bounding it: a `directory` pool with no `quota = true`.

* Set `node.oci = docker` on the nodes running both podman and docker whose `oci` resources run on docker.

    The `oci` container and task resources run with the engine `node.oci` names, else podman when the node has it, else docker, as in v2. Earlier v3 releases ignored `node.oci` and preferred docker: on a node with both engines, an `oci` resource started with docker would be looked for in podman, read down, and started again there, its docker container left running.

* Set a explicit unit to every `max_delay` keyword.

    ```
    [sync#1]
    max_delay = 10   # interpreted as 10m in om2, but 10s in om3 => Bad
    max_delay = 10m  # interpreted as 10m in om2 and om3         => Good
    ```

* Set an explicit `kill = parent` to every `app.simple` resource whose processes, started by the start command, must outlive a stop, for example the sessions of a server.

    ```
    [app#1]
    type = simple
    start = /usr/sbin/sessiond
    # no kill        # parent in om2, but tree in om3 => Bad
    kill = parent    # parent in om2 and om3          => Good
    ```

## What's New in v3

OpenSVC v3 is a major evolution, rebuilt in Go for performance, reliability, and maintainability. Beyond the migration considerations, v3 introduces exciting new capabilities:

### Architecture & Performance

* **Complete Go rewrite**: The agent is now written in Go, delivering a compiled, statically-linked binary with improved performance, memory safety, and lower resource footprint compared to the Python v2 agent.

* **Simplified deployment**: Single binary distribution with minimal dependencies.

* **OpenAPI 3 based API**: Modern RESTful API with OpenAPI 3 specification, served over both Unix socket and inet, with bearer token authentication, replacing the custom raw JSON-RPC protocol.

### User Experience

* **New `ox` terminal UI**: A dedicated terminal interface for real-time cluster monitoring and management, complementing the `om` CLI. Open an `ox` session to watch services move around the cluster interactively.

* **Object status board**: `om <path> status`, `ox <path> status`, and the enter key on an object path in the terminal UI show the object cluster-wide: its resources by node in the order the actions run them, grouped by subset, the instance states, what differs from a node to another, as a node-scoped address, and the notes saying what needs attention, the resource logs included.

* **Driver catalog**: `om node drivers`, `ox node drivers` and `GET /api/node/name/{nodename}/drivers` describe each driver: a line saying what it does, its features in a documented vocabulary (`provision`, `unprovision`, `start`, `stop`, `run`, `sync`, `snapshot`, `resize`, `signal`, `abort`, `move`, `encap`), the drivers a virtual one selects among, as `container.oci` selects `container.podman` then `container.docker`, the driver an alias names, and whether the node has what it needs.

* **Enhanced logging with journald**: All agent logs now integrate with systemd-journald, enabling fast filtering with `journalctl _COMM=om` and adding indexed attributes like `OBJ_PATH` for precise log queries.

### Orchestration & Management

* **Live migration for KVM**: Full support for live migration of KVM instances with data on `disk.disk` and `disk.drbd` resources using `om <svc> switch --live` and `om <svc> takeover --live` commands.

* **New placement policy**: `last start` policy uses the mtime of `<objvar>/last_start` as the candidate sort key, with more recent starts having higher priority.

* **Improved object labels**: Enhanced support for labeling objects, enabling better organization and selection. Labels help third-party solutions like backup tools, ingress gateways, and monitoring systems identify and handle objects appropriately.

* **Better container management**: New commands for container operations including `om <svc> container list`, `om <svc> container enter <id>`, and `om <svc> container logs <id>`.

* **Task management**: New commands for task operations including `om <svc> task list` and `om <svc> task run <id>`.

* **Event-triggered tasks**: A task's new `on` keyword runs it on events of its node, `NetworkPeersUpdated` when the peers of a network changed, `FileInstalled` when an install of a resource of its object wrote files, whatever ran it, paced by `pace` (`@30s backoff @1m`), with the `run_requires` and the frozen policy of its scheduled runs. A task's new `install = <rid>...` installs the files of volumes and filesystems again before its own work, a host task's command then being optional, so files rendered from the networks follow them.

* **Resource group commands**: New per-driver-group command sets for more targeted resource management.

* **The master reaches a kvm guest through vsock**:

    With a vsock device on the guest, and `listener.vsock_port` set in it, the
    master reads the status, installs the config and runs the actions of the
    encap instances through the api of the guest daemon, with no network in
    the guest and no key. The context id is read from the running domain, so
    no registry is needed, and a live migration may change it. A guest daemon
    that does not answer is reached through `qga` or `rcmd`, as before. The
    `container.kvm` `vsock_port` keyword sets the port, `1215` by default, and
    `0` turns this off. A provision adds the device to a `virt-install`
    command knowing `--vsock`, on a hypervisor having `/dev/vhost-vsock`. A
    hypervisor with none starts a guest whose synced definition has the
    device without it, and refuses to take one running with it in a live move.

    `om <path> container enter` opens a shell in such a guest the same way,
    with no network and no key, and `rcmd` otherwise. The session is a process
    of its own in the guest, so restarting the agent from it does not end it.

### Configuration & Operations

* **Unified configuration update**: The new `update` command with `--set`, `--unset`, and `--delete` flags allows atomic commits for different types of configuration changes.

* **Quiet mode**: New `--quiet` flag for instance actions disables both progress rendering and console logging, useful for scripts and automation.

* **Check a configuration write before making it**: `om|ox <path> create --dry-run` and `om|ox <path> config update --dry-run` have the daemon run every check of the write, the rbac policy for the grants of the requester, the validation, the claims and the holds, and write nothing. The `dry_run` parameter of the `POST` and `PUT` on `/object/path/{namespace}/{kind}/{name}/config/file` and of the `PATCH` on `.../config` does the same through the api. A dry run answers the alerts of its checks, as `om validate config` prints them: every problem the write would be refused for is an error, a keyword the grants do not allow of kind `denied` naming the keyword and the reason, and the warnings of the validation come with them, which a write does not answer. The commands print the alerts, and exit non-zero when one is an error. A write the policy refuses lists every keyword the grants do not allow, with its reason, rather than the first one: one pass shows what a namespace administrator can not deploy of a template.

* **Validate a draft configuration**: `POST /object/path/{namespace}/{kind}/{name}/config/validate` takes a configuration, of an object that exists or not, and answers the alerts a write of it would raise, errors and warnings, writing nothing: the checks of a replace for an object that exists, of a create for one that does not. An editor validates what is typed as it is typed, with the grants of the user. `om|ox <path> config validate --config <file|->` does the same from the command line.

* **A namespace administrator may start the resources of a subset in parallel**: the `parallel` keyword of a `subset#<group>:<name>` section needed the root grant, as every keyword of a driver group the policy said nothing about. It orders the resources of the object, and takes nothing from the node. The other keywords subsets may gain are refused until weighed.

* **Enhanced secret management**: New commands like `om <kvstore> key rename` for better key management in secret stores.

* **A template creates its own secrets**: an install line and an `environment` reference name the source of the key they read, `source <uri>`, and `source random` generates a password:

    ```
    [env]
    oc3_environment = OC3_DB_PASSWORD from ./sec/{name} key db_password source random

    [container#db]
    environment = {env.oc3_environment}
        MARIADB_ROOT_PASSWORD from ./sec/{name} key db_root_password source random
    ```

    The provision of the object on its leader node creates the sec or the cfg in the namespace of the object, and generates the keys missing, once for all the nodes: the provisions of the other nodes wait for them to reach their node, and fail naming the key when they do not. A start, a standby start, a task run, or an install, not finding a key generated fails naming it and its datastore, rather than starting without it or making a password of its own. An `http://` or `https://` source is fetched, and a file source read, at each provision and start not finding the key, the `environment` references like the install lines. A key there is never written again, and an unprovision leaves the keys.

    An object whose resources have nothing to provision, as apps alone, is never provisioned: it takes a `fs.flag` resource to have its keys generated.

    The validation of the configuration refuses a key two keywords seed from two sources, wherever they are written, in a section, in `DEFAULT` or read through a reference, a source on an install line naming its keys by a pattern, `key conf/*`, and `source random` in the datastores of a pattern, `from ./sec/haproxy-*`, a datastore made later never having its key. `environment` and `create_environment` reading a file of the node, `source <path>`, require the root grant, as the install lines do: `denied: container#db.environment=…: a server-local source uri requires the root grant`.

    `om|ox <sec|cfg> key add --from random` generates a key the same way. `--from /dev/urandom` still does, deprecated: it warns to use `--from random`.

    A key seeded from an url is fetched as long as the action seeding it lasts, and holds `node.max_key_size` at most: a body announced or sent larger fails the fetch, rather than being read whole for the key add to refuse it.

* **An install line fetches a file from an url, with no datastore**: a file too large for a key, as the web ui of an application, is fetched straight into the volume:

    ```
    [volume#1]
    install = /web/html/index.html.gz required source https://github.com/opensvc/oc3-frontend/releases/latest/download/index.html.gz
    ```

    The url is an `https://` one, for the root grant too: a line naming no datastore with any other source, an `http://` url, a path or `random`, is refused, and so are a `key`, a `template` and a destination ending with a `/`. Such a line made a directory of the name of the file before, the url ignored. The file is fetched by a start, a provision or an install when it is missing or its url changed, and a start finding the file of the url fetches nothing: a url naming the latest release is fetched again once the file is deleted, or the url changed for the one of a version. The file is written aside and renamed in place, so a fetch failing leaves the file fetched before. A fetch holds the new `node.max_install_size`, `64mb` by default, at most, and `sha256 <hex>` refuses a content of another checksum. The file is marked installed from the url: the volume file listing says so in `installed_from`, with no `installed_key`, and an upload does not replace it, the `409` naming the url to change.

    A file kept is checked at each install against the `sha256` of the line, else the digest of the content written, held by its mark and recorded by the node: a file written over since is fetched again, with a warning.

* **Keyword groups and strategies**: every keyword belongs to a group, a topic a wizard shows it under, and a group may have strategies, the ways to configure it that are not to be set together. The keyword definitions, `GET /api/cluster/config/keywords`, `/api/node/name/{nodename}/config/keywords` and `/api/object/path/{namespace}/{kind}/{name}/config/keywords`, give each keyword its `group`, and list in `groups` the groups of the keywords they return, with their driver, name, text, order and strategies: what each strategy uses, excludes, and the selector values choosing it. `om|ox … config doc` shows the group of each keyword. The keywords every resource has are grouped as availability, monitoring, provisioning, requirements, triggers, reservation (the SCSI persistent reservation of the disks, the lock of a rados disk), tags and subsets, and encapsulation, and the others are in the `other` group until sorted.

    The `ip.host` and `ip.netns` drivers group `name`, `netmask`, `network` and `addr` as their addressing, with two strategies: `static`, an address written in `name`, and `ipam`, an address drawn from the om `network`. The validation of the configuration warns of a section setting both, as `name and network are both set, but name is the static addressing strategy and network the ipam one`. A `netmask` set next to a `network` is not warned of: it overrides the one of the network.

    A group may be required, `required` in the keyword definitions: a section of its driver follows one of its strategies, and the validation refuses one setting none of the keywords they use, as `no addressing strategy is followed: set name (static), or network or addr (ipam)`, a `missing strategy` error. A strategy may require keywords, `requires`, which a section following it sets, in any scope or inherited from `DEFAULT`, as `dev is not set, but the static addressing strategy requires it`, a `missing keyword` error. The addressing of `ip.host` and `ip.netns` is required, and its `static` strategy requires `dev`: an ip naming neither an address nor a network, or a static address naming no device of the node, was accepted by the validation, and its start failed on the device of an empty name, as `mtu: route ip+net: invalid network interface name`. The `ip.netns` start now refuses such a configuration with what its status says, before it sets anything up.

    A keyword set where a condition it depends on does not hold, as its `depends` in the keyword definitions say, is warned of as an `ignored keyword`: `flex_min is set, but it applies only where topology=flex, and topology is failover: it is ignored`. The keywords of a flex service on a failover one, `stonith` on a flex one, a `pg_*` keyword with `create_pg = false`, and the `vlan_tag` and `vlan_mode` of an `ip.netns` whose `mode` is not `ovs`. The condition is judged on the value the configuration reads, inherited from `DEFAULT` and defaulted, as `topology` is `failover` when not set, and not judged on a key with no value, as the `mode` a network chooses when the address starts.

    The container drivers group all their keywords: identity, storage, network, guest (how the agent of the node reaches the system in the container) and timeouts for all of them, image, process, environment, isolation and privileges, and quiesce for `docker`, `podman` and `oci`, creation for `lxc`, virtual machine for `kvm` and `vbox`, and installation for `kvm`. The installation of a `kvm` guest has three strategies, the ones its provision refuses to mix: `image`, a cloud image and its `cloud_init`, `cdrom`, an installer, and `virtinst`, a command saying all of the guest, which excludes every keyword describing it, as `virtinst and vcpus are both set, but the virtinst install strategy uses virtinst and excludes vcpus`. The quiesce of an oci container has two, chosen by the value of `quiesce`: `pause`, which reads none of `quiesce_input`, `quiesce_ready`, `resume_input` and `quiesce_timeout`, and `command`: the validation checks a strategy chosen by a value only where the section has that value, so `quiesce = pause` with a `quiesce_input` warns.

    The task drivers group all their keywords too: scheduling (`schedule`, `on`, `pace`), runs (`confirmation`, `max_parallel`, `log`, `on_error`, `snooze`), status (`check`, `retcodes`), install and timeouts for all of them, certificates for `acme`, process (`command`, `container`, `stop`, `cwd`, `user`, `group`, `umask`), environment and limits for `host`. The `docker`, `podman` and `oci` tasks describe the container a run starts in the groups of the oci containers, the same declarations: identity, image, process, environment, storage, network, isolation and privileges, and guest. The certificates of an `acme` task are required, with two strategies that exclude nothing of each other, as one run renews both: `secs`, the secs named, and `domains`, the domains read from `domains_from` into the secs `domains_to` makes, which requires both keywords. The validation refuses a task naming neither, which its run failed with `no sec to renew`, and one setting `domains_from` or `domains_to` alone, which its run refused. No other task group has strategies: a `host` task running its command in a `container` still runs its `on_error` and `stop` commands on the node, as the `cwd`, `user`, `group` and limits say.

    The other resource drivers group all their keywords as well, none left in the `other` group. The filesystems group their device, mount, creation, size, status, owner and permissions, and install keywords, `fs.directory` its directory and size, `fs.sgcp_nfs` its access, provider api and timeouts. A `volume` groups the volume it asks for and the pool it draws it from, a `share.nfs` its export. The disks group their device, creation, replication, access and timeouts, `disk.crypt` its passphrase and key derivation, and `zpool` `multihost` joins the reservation group. The syncs group their source and target, array, snapshots, transfer, scheduling, status, retention and timeouts. The `app` drivers group their process, status, environment, limits, quiesce and timeouts, which a `host` task shares, and the ip drivers their interface, routing and dns, `ip.cni` its addressing, `ip.sgcp_dnsalias` its dns and provider api. A group of the same concept keeps the same id, name and order across the drivers, as `timeouts`, `status`, `install` or `process`. None of these groups has strategies: the alternatives found are not exclusive, as a `disk.disk` provisioned from a `pool` that records its `disk_id`, or the `devs` and `devs_from` of a `sync.symsnapvx`, which merge.

    The object, node and cluster configurations group their keywords too, and no keyword is left in the `other` group, which a test enforces. The `DEFAULT` section of a service or a volume groups its identity, nodes, topology, placement, orchestration, high availability, scheduling, dependencies, sharing, process group, deployment, actions and timeouts, a volume its volume, a `sec` or `cfg` its subject and certificate, a `usr` its grants, a namespace configuration its rootless containers, claims and process group. The node and cluster configurations group each section by what it says, with the ids of the resources where the concept is the same: identity, endpoint, listener, transport, nodes, credentials, quorum, timing, scheduling, fetch, logging and others. The groups of a section type of these configurations are listed with it as their driver, as `array.hp3par`, whose credentials have two strategies: `ssh`, a `key` and a `username`, and `cli`, a `pwf` when `method = cli`. The validation of the node and cluster configurations now checks the strategies of their typed sections, as `pwf and key are both set, but …` warns.

* **Keyword references**: a keyword whose value names resources of the same object says so in the keyword definitions, `GET /api/cluster/config/keywords`, `/api/node/name/{nodename}/config/keywords` and `/api/object/path/{namespace}/{kind}/{name}/config/keywords`, by its `references`: the driver `groups` of the resources it names, empty for any, and the `shape` of the value, `rid` for one resource id, `rids` for resource ids separated by whitespaces, and `requires` for the `<rid>[(<state>,...)] ...` form of the `*_requires` keywords. A designer offers the sections of the configuration as their values. `om|ox … config doc` shows them, as `references: container rid`. The keyword definitions leave `references` out for a keyword naming no resource.

    The keywords naming resources are the `netns` of `ip.netns`, `ip.cni`, `ip.route` and `ip.rule`, the `netns`, `pidns` and `ipcns` of the `docker`, `podman` and `oci` containers and tasks, the `userns` of a `podman` container or task, the `container` of a `host` task, all a `container`; the `netns` of an `app`, an `ip`, a `container` or an `app`; the six `*_requires` keywords, any resource; the `rids` of a `sync.store` and the `install` of a task, a `fs` or a `volume`; and the `devs_from` of a `sync.symsnapvx`, a `disk`, `fs`, `volume` or `container` listing its sub-devices.

    The validation of the configuration warns of a resource named that no section of the configuration is, as `container#9 names no section of the configuration`, a configuration being possibly written in steps, and refuses a resource of a group the keyword does not take, as `fs#1 is no container resource`. A value written with no `#` is no resource id and is not judged so, as the `userns` modes of podman, `keep-id` or `auto`. A value set in `DEFAULT` is checked for the resources it names, not their group, as resources of different drivers inherit it.

    The `netns` of the `docker`, `podman` and `oci` containers and tasks takes as candidates empty, `none` and `host`, `pidns` empty and `host`, and `ipcns` empty, `none`, `host`, `private` and `shareable`, besides a `container#<n>`: an engine value written as it is, as `container:<name>`, is refused by the validation, the root grant still able to pass it with `run_args`. The example of `utsns` is `host`, as its candidates say.

* **Act on the configuration you just wrote**: A configuration write answers with the timestamp the configuration now carries, in the `OM-Last-Modified` header of the `POST` and `PUT` on `/object/path/{namespace}/{kind}/{name}/config/file`, and every instance action accepts that timestamp as the `config_updated_at` parameter.

    An instance whose configuration is older than the one named answers `409 Conflict`, saying which configuration it holds, rather than running the action on the configuration the write was replacing. A write is acknowledged by the node that received it and reaches the peer nodes a moment later, so a client that writes a configuration and immediately acts on the instances would otherwise race that propagation, silently, on every node but one.

    The parameter is optional: an action asking for no configuration in particular runs on whatever the node holds, as before.

* **Cluster join keeps the collector registration**: `om cluster join` moves the node configuration aside, its `node.uuid` registration id included. The node it joins through now registers it on the oc3 collector with that id, in its own app and cluster, and the joining node stores the new id before its daemon starts, so it is fed to the collector right away. Once its daemon has started and has installed the configurations of the svc and vol objects scoped on it, the joined node also pushes the asset, package, disk, sysreport and checks inventory `om node register` pushes. Past 5 minutes of wait, the checks and disks, which describe these objects, are left to the scheduled pushes. `om cluster enroll --wait` then shows, from the collector state of the node, whether it is registered. A node without a registration id is left unregistered, and a refused one is warned to run `om node register`, which now sends the cluster id too, so the collector places the node in its cluster before its first daemon status.

### Security

* **SSRF protection for HTTP fetches**: 

    All configuration, cfg, and secret 
    values fetched from HTTP(S) URLs are now validated against SSRF policies. 
    By default, only what the opensvc organization publishes is allowed,
    `https://raw.githubusercontent.com/opensvc/*` and `https://github.com/opensvc/*/releases/*`,
    while all other URLs and private/internal CIDR ranges 
    (including loopback, RFC 1918, link-local, TEST-NET, and IPv6 ULA) are blocked. 
    The `node.allowed_urls` and `node.allowed_cidrs` keywords add to what is allowed, for
    all the nodes of a cluster.

    A download follows a redirect, at most 5 times and never from https to
    http, and a request carrying data, as the calls of an array api, never: the
    url lists judge the url a configuration names, and the target of each
    redirect is held to the cidr lists, its address checked and the
    connection pinned to it. A release download is so trusted by its url,
    `https://github.com/<owner>/<repo>/releases/*`, wherever its storage
    redirects to. Administrators can override these defaults 
    via environment variables in `/etc/default/opensvc` or `/etc/sysconfig/opensvc`,
    `OSVC_SSRF_ENABLE_REDIRECTS=false` refusing the redirects. The url a
    command is given, `om|ox <path> create --config <url>` and
    `om|ox <path> key add|change --from <url>`, is fetched by the command
    itself, with no SSRF policy: whoever runs it could fetch it with curl
    and pass it on the standard input. The policy is for the urls a node
    fetches from a configuration, as the sources of the keys and the images
    of the virtual machines:
  ```
  OSVC_SSRF_ALLOWED_URL
  OSVC_SSRF_BLOCKED_URL
  OSVC_SSRF_ALLOWED_CIDR
  OSVC_SSRF_BLOCKED_CIDR
  OSVC_SSRF_ENABLE_REDIRECTS
  ```

* **Cluster certificate authority lifecycle**:

    The cluster ca, `system/sec/ca`, is created valid 10 years. The daemon
    speaker renews its certificate with the same key a fifth of its validity
    before it expires, a year at most, and issues `system/sec/cert` again
    from it: the certificates it signed stay valid. Every node installs the
    cluster authorities again when one of them changes, with no restart. A
    node warns when the certificate of `system/sec/cert` expires within 14
    days, or the one of `system/sec/ca` within 90.

    `om cluster ca rotate` replaces the key of the cluster ca, waiting at
    each step for every node, so no node refuses another meanwhile. The
    former ca stays trusted, through the cluster `ca` keyword, until the
    certificates it signed are renewed.

* **User socket**:

    The local accounts of the members of the `listener.ux_owner_group`
    group, `opensvc` by default, reach the api through
    `/run/opensvc-ux/http.sock` with no credential, the daemon reading who
    they are from the connection: they get the grants of the
    `[uxgroup#<group>]` sections naming their groups, and of the `usr`
    object whose `uxuser` names their login or uid. `ox` uses it when run
    by a user other than root with no context. A sidecar container the
    socket is bind mounted into gets the `uxgrant` of its resource, told by
    its cgroup. Root is not root on this socket. Prefer it to `sudo om`
    rules, which are root grants whatever arguments they allow.

* **Node to node authentication**:

    A node calls the api of its peers with a token it signs with the key
    of the cluster ca, valid 5 seconds, instead of the cluster secret. The
    token is signed once the connection to the peer is up, after the tcp
    connect and the tls handshake, so a peer slow to reach, as when its
    network comes back, does not receive a token already expired.

    The 401 answers of the api carry a `WWW-Authenticate: Bearer`
    challenge, as RFC 6750 describes it, with `error="invalid_token"` for a
    bearer token refused, and an `error_description` saying when the token
    is expired.

### Network & Storage

* **Modern firewall management**: Configuration now uses nftables exclusively, with better support for large subnets including IPv6 via the new `mask_per_node` keyword.

* **Improved mount monitoring**: New daemon manager (`mntmon`) for monitoring mount events and refreshing instance status when mounts change.

* **Network event handling**: New daemon network monitor (`netmon`) relays netlink events to pubsub, enabling faster response to network changes.

* **Floating addresses from a cluster network**: The new `lan` network type is a range of a segment the nodes share, whose addresses om hands out to the cluster rather than to a node. An `ip.host` resource naming such a network and no address draws one, the same on every node: a failover service takes it along to the node it moves to, and the address other resources hold anywhere in the cluster is never drawn twice.

* **New install keyword**: For fs and volume resources, the new `install` keyword enables deployment of complex file trees on start, with support for sec keys, cfg keys, local files or remote URIs, file/directory nesting, and user/group/permission setup. A file it installs takes the mode its line sets, else the `perm` keyword of the resource, else `0600` for a sec key and `0644` for a cfg key, as the `secrets` and `configs` keywords read `perm`. A `template` line renders its key on each node at each install, with the `[env]` section as data and functions reading the node's references, other datastore keys and network peers, and a change of a key it read installs the file again, as a change of the template does. A host task's new `container` keyword runs its command in a container resource, whatever its runtime, so a container reads again the files an install changed.

* **Namespace claims on cluster resources**: A namespace can be capped on what it takes of a resource its peers share, declared in its configuration as a `claim` section:

    ```
    [claim#1]
    type = pool
    name = tank
    limit = 250m

    [claim#2]
    type = network
    name = backend2
    limit = 10
    ```

    A pool claim counts the size each volume of the namespace was created or resized with, and an allocation or a resize taking the namespace over its limit is refused. A network claim counts the addresses the namespace holds cluster-wide, and an allocation taking it over its limit is refused, while an address already held is never re-claimed, so an object at the limit still restarts.

    A namespace declaring no claim on a resource is not capped on it, and the limit is read from the namespace configuration on the node doing the allocation, so the common case asks nothing of the daemon.

* **Thin logical volumes**: A `disk.lv` resource naming a `thin_pool` of its volume group is provisioned as a thin volume of that pool, its `size` being the size it is seen with. A thin volume is snapshotted in its pool, with no size of its own, so a `sync.store` resource copies a filesystem on it from a snapshot taken under the quiesce. The resource warns when the data or the metadata of its pool is 80% used, a full pool failing the writes of all its volumes. A `vg` pool naming a `thin_pool` makes its volumes thin volumes of it, has the `snap` capability, and reports the usage of the thin pool. The thin pool is made by the sysadmin, as the volume group is:

    ```
    lvcreate --type thin-pool -L 100g -n pool1 data

    [pool#thin]
    type = vg
    name = data
    thin_pool = pool1
    ```

* **Light key derivation for the encrypted disks om keys**: A `disk.crypt` resource whose passphrase om generates, `manage_passphrase = true` as by default, formats its volume with `--pbkdf pbkdf2 --iter-time 100`. The default LUKS2 derivation, argon2, takes up to a gigabyte of memory and several threads for seconds at each provision and each start, which stalled a small node long enough for its daemon to miss the systemd watchdog, and makes a random 256-character passphrase no harder to guess. A passphrase the user supplies, `manage_passphrase = false`, keeps the derivation cryptsetup chooses. The new provisioning keywords `pbkdf` (`pbkdf2`, `argon2i` or `argon2id`), `pbkdf_memory` (a count of KiB or a size), `pbkdf_parallel` and `iter_time` (milliseconds) set the derivation of both, `pbkdf_memory` and `pbkdf_parallel` being refused with `pbkdf = pbkdf2`. The volumes formatted already keep their derivation, which the open reads from their header.

## Breaking Changes

### Cluster and Node Configuration

* **hp3par `method = proxy` removed:**
    The proxy method of v2, a node of the zone of the array serving the
    commands of the nodes of a dmz, is not ported. The validation refuses it,
    saying what replaces it: the `ssh` method, its `manager` a bastion of the
    zone of the array whose authorized key forces a command passing on to the
    array the remote copy commands it allows. The hp3par dmz howto of the book
    shows the key line and the command. The ssh client of the `ssh` method
    never prompts and gives up a `manager` it can not reach in 10 seconds,
    `-o BatchMode=yes -o ConnectTimeout=10`, so a failover fails rather than
    hangs.

* **Hook**
    The event is no longer passed via stdin.
    The om3 daemon feeds the event as a json-formatted EVENT environment variable instead.
    The events name and payload structure changed too.
    See `om node events --help` for the list of event names.

* **Time format change:**
    OpenSVC now uses RFC3339 time format for all internal and exposed data, replacing the Unix timestamps.

* **Sysreport:**
    `om node sysreport` reports to the oc3 feeder, `POST /api/node/sysreport`, and no longer to the jsonrpc `send_sysreport` method of the old collector: an om3 node is fed to oc3.
    `om node sysreport --force` sends a full report, which the collector replaces what it holds of the node with, rather than a list of deletions computed from the jsonrpc `sysreport_lstree` of the old collector.
    A report is a full one until the collector accepts one: the first report of a node after the upgrade sends every tracked file once, and a report the collector refused has the next one full, since the local cache records the files as reported when it collects them.

* **Collector keywords:**
    The oc3 collector settings move to a `[collector]` section, usually set in `cluster.conf`, and lose their `collector_` prefix:

    | Deprecated               | Replaced by               |
    |--------------------------|---------------------------|
    | `node.collector`         | `collector.url`           |
    | `node.collector_feeder`  | `collector.feeder`        |
    | `node.collector_server`  | `collector.server`        |
    | `node.collector_ping_interval`, `node.db_min_ping_interval`   | `collector.ping_interval` |
    | `node.collector_status_delay`, `node.db_min_update_interval`  | `collector.status_delay`  |
    | `node.collector_timeout` | `collector.timeout`       |

    A `[node]` keyword is still read where the `[collector]` section does not set its replacement, and the configuration validation warns about it.

    ```
    [collector]
    url = https://collector.opensvc.com
    ```

* **The former collector is no longer fed:**
    om feeds and reads an oc3 collector only, set by `collector.url`, or `collector.feeder` and `collector.server`. The jsonrpc and rest calls to the former collector are dropped, the last ones with them: `om node register` with or without `--user` on a node with no oc3 collector, which now fails saying what to set.
    `node.dbopensvc` and `node.dbcompliance` are ignored, and `om node config migrate` drops them. `node.dbinsecure` still disables the certificate verification of the calls to oc3, for a test collector.
    The `{collector_api}` reference names the api of the oc3 server, `<collector.server>/api`.

* **The compliance objects are no longer shipped with om:**
    The `compobj` binary and the compliance modules it installed in `share/opensvc/compliance` are gone from the om packages. The compliance modules come from the archive of the collector, which `om node compliance update` installs in the same place.

* **SAN switches:**
    A switch is inventoried by `om node push switch [<name>]`, and on the `schedule` of its `switch#<name>` section, with the `pushswitch` scheduler action: the v2 `om node pushbrocade` command and the `[brocade] schedule` keyword are gone.
    The `telnet` method of a brocade switch is refused, as telnet sends the password in clear: set `method = ssh`, with a `key` or a `password` secret.
    The switch key is trusted on the first connection and recorded in the root known hosts, where v2 connected with `StrictHostKeyChecking=no`: a switch presenting another key later is refused.
    The report goes to the oc3 switch feed.

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

* **`node.min_avail_mem` and `node.min_avail_swap` read as in v2:**
    They take a percentage, `10` or `10%`, or a size, `512m` or `2Gi`, which is read as the whole percentage of the memory or swap of the node it amounts to, and as 50% of the memory at most, as v2 did. A number with no unit is a percentage, where v2 read it as a count of bytes. `min_avail_mem_pct` and `min_avail_swap_pct` are read as them.
    A node without swap, or whose memory om can not read, is no longer held overloaded by them.

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
    
* **`dequeue_actions` section removed:**
    * `dequeue_actions.schedule`

### Object Configuration

* **Keywords renamed** (with backward compatibility)

    * `resinfo_schedule` => `info_schedule`
        The old name is still accepted as an alias, so existing
        configurations keep working. The schedule entry it drives is now
        named `info` instead of `push_resinfo`, and its last run is recorded
        in `<objvar>/last_info`. An upgraded object therefore has no last run
        for the new name and refreshes its resource info once, shortly after
        the upgrade.

* **Keywords deprecated** (still accepted, the validation warns of them):

    * `configs`, `secrets`, `directories` and `signal` of the fs and volume
        resources, replaced by `install`, which installs the keys of the
        datastores, makes the directories and sends the signals with one
        line per item.

    * `promote_rw` of the disk and container resources, which no disk or
        container driver reads. The fs resources still read theirs.

    * `alias` of `ip.netns`, which the driver does not read. The one of
        `ip.host` stays.

    * `gateway` of `ip.host`, a provisioning parameter of the solaris zones
        of v2, which the driver does not read.

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

* **Resource ids are letters, digits, `_`, `-` and `.`**

    The index of a resource section, after the `#`, names the directory of
    the resource in the var directory of its object, and the files its
    driver makes there and on the peers. A section named with another
    character, as `fs#a/b` or `app#a b`, is no resource: `config validate`
    reports it as an invalid section name, and a configuration write naming
    one is refused. Rename such a section before upgrading.

* **Driver Group Names Removed:**

    Drop support for driver-group names:**

	* `drbd`
	   Replaced by `disk#foo.type=drbd`
       
	* `vdisk`
	   Dropped, and so is the `disk.vdisk` driver: see Drivers removed.
       
	* `vmdg`
	   Dropped, and so is the `disk.ldom` driver it named: see Drivers removed.
       
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
	   Dropped, and so is the `disk.vxdg` driver: see Drivers removed.
       
	* `vxvol`
	   Dropped, and so is the `disk.vxvol` driver: see Drivers removed.

    For example, a `[md#1]` section needs reformatting as:
    ```
    [disk#1]
    type = md
    ```
      
    These driver group names were already deprecated in v2.1.

* **A configuration holding what another object holds is refused**

    A configuration written, by a create, a deploy, a clone, an edit or an
    update, is refused when one of its resources would hold what a resource
    of another object holds on the same node: a mount point, a block device,
    a volume group or logical volume, a zpool, an md array, a container name,
    an ip address, a directory, a volume. A resource `shared = true` holds
    its values on every node of the cluster, a resource `shared = false` on
    the nodes they evaluate to them on. The resources up also say the devices
    they were found holding.
    v2 accepted it, and the provision of the clone took what the source held.

    Root writes it all the same with `--force`. The api answers 409, titled
    "Held by another object".

    A disabled object holds nothing: stopping and disabling an object frees
    its holds for an object restored under another name, or in place of a
    receiver of its replicated data. Enabling it is refused while another
    holds them.

    A volume is used by one object, unless every object using it says
    `exclusive = false` on its volume resource. v2 let any object use a
    volume another used: the objects sharing a volume since v2 keep it, and
    a new one is refused until they all say so.

* **A provision checks every node first**

    Before any node provisions an object, every node checks that nothing on
    it holds what the provision would take: a filesystem mounted on a mount
    point, a device in use or holding a signature, a volume group, a zpool or
    an md array of the name, a container of the name, a kvm domain or an lxc
    container of the name when the provision would create one, an address
    that answers. A conflict ends the
    orchestration as a provision failure, with nothing provisioned. v2
    adopted what it found, and formatted a device not recognized.

    `--adopt` takes over what the provision finds, and `--restore` on a
    create or a deploy implies it. What a provision made is recorded, and not
    checked again: an object provisioned before this version may be refused
    a new provision for what it holds itself, and `--adopt` once records it.

* **Create and deploy go through the api**

    `om <path> create` and `om <path> deploy` send the configuration to the
    daemon, which checks it as any configuration it writes, rather than
    writing the file themselves, unless the daemon is not running. The api
    resets the recorded keywords of a configuration it creates an object
    from, an id, a `disk_id`, a drawn address, an md uuid, whoever sends it,
    unless `restore=true`. `disk_id` is now recorded, so a clone of an object
    holding a pool disk gets a disk of its own.

* **A change a resource up would leak by is refused**

    A change of a keyword saying what a resource holds on a node once
    started, as the `mnt` or `dev` of a fs, the address, `dev`, `netns` or
    `network` of an ip, the `name` of a volume group or a container, a change
    of the type of a resource, its disable, or the removal of its section, is
    refused while the resource is up on a node, by root too. v2 accepted it, and the
    stop of the resource then acted on the new value, leaving the former
    mount, address or device on the node.

    Stop the resource first. Root writes the change all the same with
    `--force`, as to recover from a resource that cannot be stopped. The
    keywords concerned say `stop first` in `om <path> config doc`.

* **Security: a keyword inherited from `DEFAULT` is judged as the keyword of the section inheriting it**

    The keyword policy now judges a keyword written in `DEFAULT` as the
    keyword of every resource section inheriting it, on its evaluated value
    before and after the write. A namespace administrator could write
    `privileged`, `volume_mounts`, `devices`, `run_args`, `netns`, or the
    keywords of a driver, in `DEFAULT`, and have the containers, tasks,
    filesystems, ips, shares and syncs of the object inherit values the
    policy refuses in their own section. Such writes are refused, as is
    adding a section that inherits a root-only value, or removing a section
    keyword that shadowed one. The refusal names both keywords, as
    `denied: container#1.volume_mounts=/etc:/hostetc:ro, inherited from DEFAULT.volume_mounts: host path mounts in container require the root grant`.

    A keyword written in `DEFAULT` that no section inherits is let in, and
    judged when a section comes to inherit it. The keywords a set of
    identical containers share, as `image`, `rm`, `restart`, `environment`
    or a `volume_mounts` of a volume, are written once in `DEFAULT` as
    before.

* **The type of a resource no longer inherits from `DEFAULT.type`**

    A section runs the driver its own `type` names, or the default driver of
    its group, as in v2. `type` written in `DEFAULT` chose the driver of every
    section naming none, a way around the rule on the driver a section runs.

* **A key of a datastore of another namespace is never seeded**

    A datastore sharing its keys with a namespace, `share = <namespace>`,
    lets the objects of that namespace read them. An install line naming a
    `source` seeded the key missing of such a datastore, a write the share
    did not grant. The seeding is refused, whatever the source, and the
    validation of the configuration refuses an install line or an
    `environment` reference naming a source for a key of a datastore of
    another namespace. Seed the key from an object of the namespace of the
    datastore, or add it there with `key add`.

* **`source /dev/urandom` is a path, and refused**

    An install line `source /dev/urandom` read the device as a file, and
    never ended. A source is read when it is a regular file only, and the
    error says to write `source random`, the generator, instead.
    `key add --from /dev/urandom` keeps generating a password, deprecated in
    favor of `--from random`.

* **A trigger is evaluated as it runs, and not run with a reference not available**

    The command of a trigger, as `post_provision`, is evaluated when the
    trigger runs, so a post trigger reads what the action made: the
    `{volume#1.mnt}` of the volume it provisioned. It used to be evaluated
    before the action, and ran with the mount point empty. A reference that
    evaluates only at run time, `{volume#<n>.mnt}`, `{<rid>.exposed_devs}`,
    `{<rid>.capacity}`, `{<rid>.free}` or `{dnsnodes}`, and is not available,
    empty or failing, stops the trigger: a blocking trigger fails its action,
    a non-blocking one is not run, with a warning. A `blocking_pre_provision`
    naming the mount point of the volume it provisions now fails the
    provision, where it ran with an empty path.

### Commands

* **Configuration updates and queries use the daemon api by default:**
    `om set`, `om unset`, `om get`, `om eval` now need `--local` to operate on the local configurations without api calls.

* `om xx status`
    The silent command whose exit code was the instance status now shows the status of the object cluster-wide: a
    board of the resources by node, with the instance states above it and the notes saying what needs attention
    below it, or the dataset with `-o json`. With `--refresh`, every node evaluates again the status of its instances
    first, and the board waits for them.

    The exit code no longer carries a status, with or without `--refresh` or `--monitor`: it is 0 when the command
    shows the status, whatever the status, and non-zero only when the command fails. A script testing the
    availability of an object reads it from the output instead:

    ```
    om svc1 status --refresh -o tab=object.avail
    ```

* **The actions of a disabled instance exit 7:**
    `start`, `stop`, `provision` and `unprovision` of an instance whose object has `DEFAULT.disable=true` do
    nothing, as before, and exit 7 where v2 exited 0: a script tells an action not done because the instance is
    disabled from a success and from a failure. The daemon takes the code as the end of a stop and of an
    unprovision, so a disabled object is purged. The `unprovision` of a disabled instance whose resources are
    provisioned fails: deleted, the object would leave its storage on the node. Enable it to unprovision it.

* `om node register` deprecates `--user` and `--password`. The collector credentials now reach the command as
  `--credential`, naming a file holding the `<username>:<password>` of a collector user, and the
  `OSVC_COLLECTOR_CREDENTIAL` environment variable is read when the option is not set. A password passed as a flag
  value is readable by any user through the process table, and stays in the shell history.

    The two options are still accepted, so the commands written against the previous release keep working, but they
    are hidden from the help. A `--credential` wins over them.

    ```
    om node register --user me --password s3cret   # deprecated, still accepted
    om node register --credential /etc/opensvc/collector.cred
    ```

* **`om node updatepkg`**, now `om node update pkg`, the historical name kept as a hidden alias as the collector
  queues it, upgrades the `opensvc-server`, `opensvc-client` and `opensvc-webapp` packages installed: with the package
  manager (apt-get, dnf, yum or zypper) from its configured repositories when they offer them, else downloading the
  latest version of each from the opensvc package repository. `node.repopkg` now names a repository laid out as
  https://packages.opensvc.com, the default, and `node.branch` a branch of it, `prod` by default; `node.repo` is no
  longer used to update the packages.

* **`om cluster update pkg`** upgrades the opensvc packages of all the cluster nodes, one node at a time. The nodes
  are asked by an orchestration, as `om cluster freeze` asks them, which `--wait` waits for the end of and
  `om cluster abort` stops. Each node runs `om node update pkg` in turn, in the order of `cluster.nodes`, once every
  node of the cluster is live and idle: the upgrade of `opensvc-server` restarts the daemon, and the next node waits
  for the one before it to be back. The upgrade is given up when it fails on a node, and refused while a node of the
  cluster is not live. A node upgrading is in the `upgrade` node monitor state, which the peers treat as a
  maintenance: its data is kept while its daemon restarts, so the services it runs are not taken over.

* `--format` is no longer supported with `print status`. The `om svc1 print config --format json | om svc2 create`
  pattern can be replaced by `om svc1 config show | om svc2 create --config=-`

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

    * `om xx push resinfo`
        Replaced by `om xx instance info --refresh`, which refreshes the
        instance resource info cache. Feeding the collector is no longer the
        pushing node's job: the collector speaker reports the refreshed
        key-values on its own schedule.

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
    * `om xx print resinfo` => `om xx instance info`
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

    * `om <selector> create|update --resource <json>`
        Replaced by `-kw`.

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

* **`om <selector> run --env a=1`:**
    The previous release treated `1` as an override value of the `env.a=0` configuration key, which prevented access to the original `env.a` value.
    Now `om` just propagate the `a=1` in the task command execution environment, so task commands using references like `{env.a}`, expecting the evaluation is `1` if `--env a=1` else `0`, must change to `${a:-{env.a}}`.

* **`om <kvstore> key change`:**
    This action is no longer failing if the key does not exist. The key is added instead.

* **A key write succeeds when a service installing it fails:**
    A key added, changed or seeded refreshes the services installing from its datastore. A refresh failing in one
    of them made the write fail, though the key was written, and kept the services after it from being refreshed.
    Every service is refreshed now, and the write succeeds, each failure warned of with the service and the resource
    it failed in. `om <kvstore> key install` still fails with them: installing is what it is asked to do.

    The api answers of `PATCH /api/object/path/{namespace}/{kind}/{name}/data` and of the `POST` and `PUT` on
    `.../data/key` are `200` with a `Committed` body, `{"is_changed": ..., "warnings": [...]}`, instead of
    `204 No Content`.

* **A key is seeded by the install of its resource only:**
    A `source` of an `install` line seeds the key when the resource installs, at its start, provision or task run.
    The refresh the write of another key runs installs what the datastore holds and seeds nothing: a write fetched
    the sources of the lines of every service installing from the datastore, and an install seeding two keys
    reported the failure of the second as the one of the first.

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

* **Array disk commands (`om array <name> ...`, and `om node array ... -a <name>` as the collector queues them):**
    The commands the collector disk forms queue on an array proxy behave as in v2, with these differences, all made to
    refuse what v2 did to the wrong disk or reported done when it was not:

    * A `--size` unit is a power of 1024 whatever it is written as: `10g`, `10gb` and `10GiB` are all 10 GiB. v2 read
      `gib` as a power of 1000. A size can not be negative: a volume is shrunk by giving its new size.
    * A resize to a size below the current one is refused unless `--truncate` is given (the symmetrix `--force` is an
      alias), as the arrays shrink a volume by dropping its end. A resize to the current size changes nothing.
    * A mapping that can not be made is an error before anything is created, rather than a volume created and left
      unexported. A failure after the volume is created names it and leaves it in place: nothing is deleted on a
      failure.
    * freenas: an existing zvol or extent is refused rather than reused, `del` refuses a dataset that is not a zvol,
      and a disk name no extent or zvol has is an error rather than a success.
    * hds: a device number the driver can not read is refused rather than sent to the array, and a wwid resolves only
      to a volume of the array named, by its ldkc, cu and ldev, where v2 read the last 4 digits of any wwid.
    * symmetrix: `del disk` refuses the R2 of a SRDF pair, which the collector deletes after its R1, as the result of
      deleting an R2 named its R1 for the collector to delete next.
    * xtremio: a volume name made of digits only is refused, as the array reads it as a volume index, and the
      `password` keyword is a secret reference, as for the other arrays.

* **`om monitor`:**
    Instance availability icons changes:
    ```
	standby down: s => x
	standby up:   S => o
    ```
 
### API access to secrets

* **The cluster configuration is read by root only:**
    `GET /api/cluster/config`, and the configuration file of the cluster through
    `/api/object/path/root/ccfg/cluster/config/file` and
    `/api/node/name/{nodename}/instance/path/root/ccfg/cluster/config/file`,
    now need the `root` grant, or the `join` grant of a joining node. They
    were open to a guest of the `root` namespace, and the cluster configuration
    holds `cluster.secret`, the key every sec and usr value is encrypted with.

* **Secrets are redacted for the readers not allowed to see them:**
    The configuration of an object read by a user who is neither an
    administrator of its namespace nor root, through the keyword or the file
    endpoints, shows `********` in place of the values of the keywords
    declared secret and of the keys of a sec or usr object, raw and evaluated.
    The `redact-secrets` parameter still redacts for any reader.

* **A user is given only the grants its writer holds:**
    Writing the `grant` keyword of a usr object refuses the grants the writer
    does not hold, as v2 did, and changing the `cn` a user authenticates by
    with its certificate needs the `root` grant. Writing a key of a usr object,
    its password or its certificate, needs holding every grant of that user,
    which v2 did not ask. An administrator of the `system` namespace manages
    the users up to its own grants, and can no longer make itself root by
    creating a root user or resetting the password of one.

* **The keys of a usr object are read by root only:**
    `GET /api/object/path/system/usr/{name}/data/key` needed the guest role,
    and answered the password and the certificate private key of the user,
    which authenticate as that user.

### Metrics and profiling

* **`/debug/pprof` needs the root grant:**
    The profiling routes of the listener were served with no authentication,
    the command line and the stacks of the daemon included. They now need the
    `root` grant. A local root user can use `lsnr/profile.sock` with no
    credential. Any route not declared public now needs authentication.

* **`/metrics` needs a credential:**
    `/metrics` was served with no authentication, and the detail pages,
    `/metrics/<subsystem>`, to root alone. They all now need the new
    `scraper` role, or `root`. A prometheus scraping with no credential is
    answered `401`: give it a user with the `scraper` grant, or set
    `listener.public_metrics = true` to serve `/metrics` with no
    authentication as before. The detail pages stay protected either way.

    ```
    om system/usr/prometheus create --kw grant=scraper
    om system/usr/prometheus key add --name password --value <password>
    ```

    ```yaml
    scrape_configs:
      - job_name: opensvc
        scheme: https
        tls_config:
          ca_file: /etc/prometheus/opensvc-ca.pem
        basic_auth:
          username: prometheus
          password_file: /etc/prometheus/opensvc.password
        static_configs:
          - targets: ['n1:1215', 'n2:1215']
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

* **Asset `os_vendor` and `os_release` of the openSUSE editions identified as `opensuse-<edition>`:**
    The other distributions report the `os_*` asset values v2 reports, as the collector filtersets match them. v2 did
    not know the `opensuse-leap` and `opensuse-tumbleweed` identifiers of Leap 15 and later and of Tumbleweed: it
    reported the name of the edition as the vendor, and an `Unknown` release on Tumbleweed. v3 reports them as:

    | os-release `ID`       | v2                               | v3                            |
    |-----------------------|----------------------------------|-------------------------------|
    | `opensuse-leap`       | `openSUSE Leap`, `15.6`          | `SuSE`, `Leap 15.6`           |
    | `opensuse-tumbleweed` | `openSUSE Tumbleweed`, `Unknown` | `SuSE`, `Tumbleweed 20241001` |

    The Tumbleweed release carries its snapshot, the `VERSION_ID` of its os-release.

### Drivers removed

These drivers of v2.1 have no v3 counterpart:

* app: `winservice`
* container: `amazon`, `esx`, `hpvm`, `jail`, `ldom`, `lxd`, `openstack`, `ovm`, `srp`, `vcloud`, `vz`, `xen`, `zone`
* disk: `advfs`, `amazon`, `gandi`, `gce`, `hpvm`, `ldom`, `pool`, `vdisk`, `veritas`, `vxdg`, `vxvol`
* fs: `docker`
* ip: `amazon`, `crossbow`, `gce`, `rule`
* sync: `dds`, `docker`, `evasnap`, `hp3par`, `hp3parsnap`, `ibmdssnap`, `necismsnap`, `netapp`, `nexenta`, `oci`, `radosclone`, `radossnap`, `s3`, `symclone`, `symsnap`
* the `certificate`, `expose`, `hashpolicy`, `route` and `vhost` sections, which
  described the routes of an object to the envoy ingress gateway of v2.

A section of one of them in an upgraded configuration is not a resource: the
object runs without it, and no action touches it. The configuration validation
warns about it, and the instance status lists it as an optional resource with
the warning `the <driver> driver is not supported by this agent`, which makes
the overall status of the instance warn. The availability status is not
changed, so no monitor action is triggered by a resource that never runs.

### Driver: container

These changes apply to the `container.docker`, `container.podman` and
`container.oci` drivers, and to the `task.docker` and `task.podman` drivers,
which share the same executor.

* **Changed behaviour, the resolver of a container:**
    om writes the `/etc/resolv.conf` of the container and bind mounts it, in
    every network mode, instead of asking the container engine for one.

    The engine refuses the dns options in the network modes an object sharing
    a namespace uses, `--net none` and `--net container:...`, and writes no
    resolv.conf of its own for them, so a container of the pause model was
    left with no resolver at all. A file works in every network mode, and is
    the only way to reach a container that has no engine-managed network.

    `--dns`, `--dns-opt`, `--dns-option` and `--dns-search` are dropped from
    `run_args`: they would be a second resolver configuration, in a place that
    no longer decides anything.

* **New keyword, `dns`:**
    The nameservers to write in the container resolv.conf after those of the
    cluster, for a container that also has to resolve names the cluster
    nameservers do not serve.

    The order is not a preference: a resolver tries the nameservers in turn
    and moves on only when one does not answer, and a name it is told does not
    exist is an answer. A nameserver that does not serve the cluster zone,
    asked first, would end the search for every object name.

    A resolver reads the first 3 nameservers of the file, so a cluster already
    naming 3 in `cluster.dns` leaves this keyword no room. The extra
    nameservers past that count are not written, and the start logs which.

* **Changed rbac, `dns` and `dns_search`:**
    Both decide what the names in a container resolve to, now that om writes
    the resolver rather than the engine, so both require the root grant to set
    through the api.

* **Changed rbac, what of the node a container reaches:**
    As in v2, a user holding no root grant may not set, on a container or a
    task, `privileged` to a true value, `devices`, or a `volume_mounts`
    source that is a path of the node. Tasks were not held to the host path
    mounts rule before.

    The namespaces a container or a task joins, `netns`, `pidns`, `ipcns`,
    `utsns` and `userns`, are for a user holding no root grant the engine's
    private ones, empty or `none`, or the ones of a container of the object,
    `container#<n>`, as the `netns` of an ip resource. Any other value goes
    to the engine as it is, and needs the root grant: `host`, a container of
    another object, `container:<name>`, a namespace of the node by its path,
    `ns:/proc/1/ns/net` with podman, and the podman `userns` modes, as
    `auto` or `keep-id`. Only `netns = host` was refused before, and the
    other namespace keywords were open.

### Driver: container.docker

    * `stop_timeout`
    Updated the default value for the stop_timeout keyword from 120 seconds to 
    10 seconds to align with the default behavior of Docker and Podman.

### Driver: container.podman

    * `stop_timeout`
    Updated the default value for the stop_timeout keyword from 120 seconds to 
    10 seconds to align with the default behavior of Docker and Podman.

* **Changed behaviour, the network of a container with no `netns`:**
    A container with no `netns` keyword, and no network named in its `run_args`,
    runs with `--net none`, as the `container.docker` driver does: a private
    network namespace, with no podman network. v2 ran it on the first network
    podman offered.

    An om network ip plugged in its namespace kept the default route of the
    podman network, which the replies took, so the other nodes got no answer.
    A container with om network ips on an engine network is not supported.

    The ports `run_args` publishes need an engine network, which podman
    discards them without: name it in `run_args`, as
    `run_args = --network podman -p 2222:22`. `expose` publishes nothing, it
    makes SRV records.

* **Changed behaviour, the ports a process not root listens on:**
    A container with its own netns, the private one of a container with no
    `netns` or the one of an engine network named in `run_args`, runs with
    `--sysctl net.ipv4.ip_unprivileged_port_start=0`, as docker sets it: a
    process not root, as a haproxy run as user 99, listens on ports under
    1024 in it. podman left the kernel default, 1024. The netns of the node
    or of another container is not set, nor a setting of the `sysctl`
    keyword or of `run_args` overridden, nor a kernel older than 4.11, which
    has no such sysctl.

### Driver: container.oci

    * `stop_timeout`
    Updated the default value for the stop_timeout keyword from 120 seconds to 
    10 seconds to align with the default behavior of Docker and Podman.

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
    * `provisioner`
    * `dns_update`
   
* **Removed feature:**
    The interface macvtap mode is dropped from the `ip.host` driver (ie the `dev=eth0@foo` syntax).

* **Changed default:**
    The `alias` keyword default value is now `true`, activating the ip stacking behaviour.
    Setting `dev=eth0:0` still forces the address labelling mode.

* **`ip.host` draws its address from an om network:**
    The `network` keyword names the om network the address is drawn from when `name` is not set, as it does on
    `ip.netns`. The address is drawn at the first start and written to the new `addr` keyword, which every node then
    uses, so a failover service keeps its address on a node it moves to after the others crashed. It is kept until the
    resource is unprovisioned, a stop keeping it. `dev`
    and `netmask` are taken from the network when not set, so `dev` is no longer required. The keyword used to hold
    the address of the network in dotted notation, which nothing read: such a value is reported and ignored.

* **`ip.host` IPv6 addresses are added with `nodad` and announced:**
    An IPv6 address is usable as soon as it is added, rather than tentative for the second the duplicate address
    detection takes, which failed the services binding it at start: the start already checks nothing answers it. It
    is announced by an unsolicited neighbor advertisement, as an IPv4 address is by a gratuitous arp, so the neighbors
    of an address moved to another node send to it there at once. v2 announced neither.

* **`ip.cni` addresses are announced at start:**
    The start sends, in the network namespace of the container and over the card the CNI ADD named, 3 gratuitous arp
    a second apart for each IPv4 address the plugin set, and 3 unsolicited neighbor advertisements for each IPv6 one,
    as `ip.netns` does for its IPv4 address. A restart reusing the address puts it on a new card with a new mac, which
    the node and the peers on the bridge learn at once, rather than send to the mac of the removed card for the 30 to
    60 seconds their neighbor entry lives. A failed announce fails the start.

* **Changed default DNS search list:**
    The fqdn of the object is no longer the first domain a container searches a
    shortname in. The default list is now the domain of the object and each of
    its parents: `<namespace>.<kind>.<clustername> <kind>.<clustername>
    <clustername>`.

    In v2 the fqdn was first, which let a container reach a container of the
    same object by its hostname alone. The record is still published as
    `<hostname>.<objectfqdn>`, so a configuration relying on the shortname must
    now name that fqdn in the `dns_search` keyword of the container.

    Containers of an instance are expected to share a netns and to reach each
    other over 127.0.0.1, which needs no name. One ip resource carries one
    hostname, the one of the container its `netns` keyword points at, so a
    shared netns has one name for the whole instance rather than one per
    container.

    The `dns_name_suffix` keyword of the ip drivers is kept, and is how an
    object with more than one address names them apart: the suffix is appended
    to the hostname the record is published under, so a second address of the
    same container answers to `<hostname><suffix>.<objectfqdn>`.

* **Changed keyword, `ip.netns.network`:**
    It names the om network the address is drawn from, as it does on `ip.cni`.
    An object attaches to a cluster network with one line:

    ```
    [ip#0]
    type = netns
    netns = container#0
    network = default
    ```

    The `dev`, `netmask` and `gateway` of the resource are read from that
    network when the configuration does not set them, and an empty `name` has
    the address allocated from it. An explicit value still wins, so a
    configuration setting them keeps working unchanged. `dev` is no longer a
    required keyword: a network names it.

    The keyword used to hold the address of the network, in dotted notation,
    which set the destination of the route `del_net_route` removes. That
    destination is the connected route the kernel adds along with the address,
    which is the address masked, and the keyword could name no other: the
    prefix length has always come from `netmask`. It is derived now. A value
    still in that form is reported as obsolete and ignored, and a value that is
    neither an address nor the name of a network is refused, naming the
    networks that exist.

* **Address allocation:**
    om allocates the addresses of its `bridge` and `routed_bridge` networks
    itself, rather than leaving them to the `host-local` cni plugin. A resource
    draws the same address every time it starts, from a hash of the object and
    the rid, so an object is not renumbered by a restart and the name it is
    published under keeps resolving to the same place. `host-local` allocates
    the first free address instead, which moves as the neighbours of an object
    come and go.

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

* **`mnt` is an absolute path:**
    A relative mount point is refused by the configuration validation. It resolved against the working directory of whichever process ran the action, and the status never saw it mounted.

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
    The peers selected are the ones the `target` keyword of the resource
    reaches, and a node selector expression selecting none of them is an
    error.

* **Changed Keyword:**
  * `max_delay` (a.k.a. `sync_max_delay`) default unit is changed from minutes to seconds, so all duration keywords use the same default unit. Set a explicit unit before migration.

* **A sync is sent from the node whose reference resources are up:**
    As in v2, the resources an object holds its data on or is reached by,
    every resource but the app, sync and task ones, must be up in aggregate on
    the node sending, and an object with none of them is not synced. `--force`
    sends from a node where they are warn. A standby node no longer sends its
    copy over the one of the active node when the object has no such
    resource.

* **`max_delay` defaults to the schedule:**
    Unset, a copy is stale once the first scheduled sync due after the last
    one is half a schedule period late, instead of the fixed 27 hours of v2.

* **The `update` and `full` actions are gated by `update_requires`:**
    v2 had a `<action>_requires` keyword per sync action. The one of the
    update, `sync_update_requires`, is renamed `update_requires`, and gates
    the `full` action too: a full copy is allowed where an update is.
    `sync_update_requires`, `sync_nodes_requires` and `sync_drp_requires` are
    read as aliases, the actions they gated being `update` now. A scheduled
    update is not scheduled while `update_requires` is not met.

* **The `sync_update` and `sync_full` actions are named `update` and `full`:**
    As `om <path> instance update` and `om <path> instance full` name them.
    The logs, the scheduled job list and the `OPENSVC_ACTION` environment
    variable of the triggers read `update` and `full`. Triggers testing
    `OPENSVC_ACTION` for `sync_update` or `sync_full` need updating.

* **A stop or a switch waits for the syncs running on the instance:**
    Stopped under a sync, the instance would hand the peers a copy the sync
    was writing. An orchestrated stop or switch now waits, in the new `wait
    syncs` monitor state, for the syncs running on the instance to end, for
    `wait_syncs_timeout` at most, 10 minutes by default. Past it, the stop
    fails and the instance keeps running, its monitoring on. No scheduled
    sync starts while an orchestration is in progress on the object.
    `--interrupt-syncs` on `stop`, `switch` and `instance stop`, or
    `interrupt_syncs` in the api, ends the syncs instead of waiting for them.
    A local `instance stop` finding a sync holding the object lock says so,
    and waits for it `wait_syncs_timeout`, or the lock timeout if longer, or
    the `--waitlock` given.

* **A shutdown, as the one of a drained node, interrupts the syncs:**
    It does not wait for the syncs running on the instance, and does not leave
    them sending to a peer that takes over: it ends them, the processes they
    started included, before stopping the resources. No scheduled sync starts
    on a node being drained.

* **New `log` keyword:**
    `log = true` logs the scheduled updates of the resource, as a run asked
    by a user is. It defaults to false.

### Driver: sync.rsync

* **`snap = true` copies a snapshot of the filesystem holding `src`:**
    v2 snapshotted the logical volume of the filesystem resource holding `src`, on Linux. The filesystem is now the
    one mounted deepest under `src`, whether a resource of the service mounts it or not, and it is snapshotted as a
    logical volume, thick or thin, as a btrfs subvolume, or as a zfs dataset. v2 copied `src` itself when it could not
    snapshot it; the update now fails. A filesystem mounted below `src` fails the update too, unless the rsync options
    keep the copy on one filesystem, as the default `-x` does: the snapshot does not hold it, and `--delete` would
    remove its copy on the peers. The snapshots are removed when the update ends, and the ones an interrupted update
    left are removed by the next, the status warning of them meanwhile.

### Driver: sync.zfs

* **Each peer is synced from its own base snapshot:**
    The source no longer rotates the `<rid>.sent` and `<rid>.tosend` snapshots
    shared by all the peers. Each run takes a
    `<rid>.<YYYYmmddTHHMMSS.ffffffZ>` snapshot, for example
    `sync.1.20260928T154211.402318Z`, and sends each peer the
    changes since the newest snapshot the peer holds in common with the
    source, found by guid. A peer that missed runs catches up at the next
    one, and a peer failing no longer stops the others. The snapshots of an
    upgraded agent are used as the base of its peers, then destroyed once no
    peer needs them. Scripts reading the old snapshot names need updating.

* **New keywords `max_lag_age` and `max_lag_size`:**
    The source keeps the base snapshot of a lagging peer. Once the peer lags
    for longer than `max_lag_age` (default `24h`), or the snapshots kept for
    it hold more than `max_lag_size` (a size, or a percentage of the free
    space of the pool, default `20%`), the source destroys its base and stops
    sending to it. The resource status then warns with the command that syncs
    it again, `om <path> instance full --rid <rid> --target <peer>`. A peer
    holding snapshots with none in common with the source is also left
    alone rather than overwritten. A peer holding no snapshot of the
    resource is still sent a full copy without asking.

### Driver: sync.btrfs

* **Each peer is synced from its own base run, and a failover is synced incrementally both ways:**
    The source no longer rotates the `last`, `next` and `temp` snapshots shared by all the peers. Each run takes
    read-only snapshots in a run directory of its own, `<root of the filesystem>/.osync/<path>/<rid>/<YYYYmmddTHHMMSS.ffffffZ>/`,
    and sends each peer the changes since the newest run the peer holds in common with the source, the snapshots being
    matched by uuid and received uuid. So after a failover, the new source sends the old one its writes incrementally,
    and so does the failback: v2 sent a full copy whenever the source changed. A peer that missed runs catches up at
    the next one, and a peer failing no longer stops the others. The snapshots of an upgraded agent are not used: the
    first sync after the upgrade sends each peer a full copy, then deletes them.

* **A peer running the service is not replaced:**
    The destination subvolume of a peer is replaced by the run it receives, staged and then swapped in. A peer with its
    destination subvolume mounted is refused, as it runs the service.

* **New keyword `max_lag_age`:**
    The source keeps the base run of a lagging peer. Once the peer lags for longer than `max_lag_age` (default `24h`),
    the source deletes it and stops sending to it. The resource status then warns with the command that syncs it again,
    `om <path> instance full --rid <rid> --target <peer>`. A peer holding runs with none in common with the source is
    also left alone rather than overwritten, where v2 sent it a full copy. A peer holding no run of the resource is
    still sent a full copy without asking.

* **The root of the filesystem is mounted for the time of a sync only:**
    v2 mounted the root of the filesystem on `<var>/btrfs/<label>` and left it mounted, which kept the device busy for
    the stop of the service. A sync mounts it on a mount point of its own and unmounts it when it ends.

### Driver: sync.btrfssnap

* **The snapshots are taken where the subvolume is mounted:**
    A subvolume not mounted on the node, as on a node the service does not run on, is not snapshotted, and the
    resource status is `n/a` there. The snapshots are named as in v2, `<subvol>/.snap/<UTC datetime>Z[,<name>]`.

### Driver: fs.btrfs

* **New keywords `subvol` and `label`:**
    The subvolume to mount and the label of the filesystem have keywords of their own, which the sync resources
    reference rather than repeat: `src = {fs#1.label}:{fs#1.subvol}`. The subvolume is created when the resource is
    provisioned, as in v2. A `subvol=` option of `mnt_opt` is still read when the `subvol` keyword is not set; one naming
    another subvolume than the keyword, or a `subvolid=` option, is refused, and so is a `-L` option of `mkfs_opt` naming
    another label than the `label` keyword.

* **The label is not chosen by the provision:**
    v2 labeled the filesystem `<name>.<rid>` when it formatted it, and set the `dev` keyword to `LABEL=<label>`. The
    filesystem is now labeled with the `label` keyword, or a `-L` option of `mkfs_opt`, and the provision does not change
    the configuration.

### Driver: share.nfs

* **The `path` keyword takes the forms of the other paths of a service:**
    `<vol name>/<path>` in a vol of the namespace, and `volume#1:/<path>` or `fs#1:/<path>` under the mount point of a
    resource of the service, besides a path of the node. A path in a volume never leads above its mount point. A share
    is stopped even when its volume is no longer available, on the path it was exported from.

* **A namespace administrator may export a volume:**
    The `share` sections needed the root grant. A user holding none may now export the mount point of a volume, written
    `<vol name>` or `volume#<n>:/`, with the export options that keep the clients to it: `rw`, `ro`, `sync`, `async`,
    `wdelay`, `no_wdelay`, `subtree_check`, `no_subtree_check`, `secure`, `hide`, `root_squash`, `all_squash`,
    `no_all_squash`, `sec`, and `anonuid` and `anongid` other than 0. A directory in a volume is refused, as the
    namespace can turn it into a link out of the volume, and so are the other options, as `no_root_squash`, `fsid`,
    `crossmnt` or `nohide`.

* **A path exported to other clients only is down:**
    v2 reported a path exported to none of the clients of `opts` as `warn` when it was exported to another client. Such
    an export is not the resource's, and a stop leaves it in place, so the resource is `down`. A path exported to a
    client of `opts` with missing options, or not exported by the kernel, is `warn`.

### Driver: fs.directory

* **The `path` keyword takes the forms of the other paths of a service:**
    `<vol name>/<path>` in a vol of the namespace, and `volume#1:/<path>` under the mount point of a volume resource of
    the service, besides a path of the node, as a directory with a quota of its own in a volume shared by containers. A
    path in a volume never leads above its mount point. A path in another resource, as `fs#2:/<path>`, is refused, as
    a directory is itself a resource paths are written in.

### Driver: disk.crypt, disk.md, disk.raw, disk.vg, fs

* **A device can be the name of a vol of the namespace, or the id of a resource:**
    The `dev` of `disk.crypt`, the `devs` of `disk.md`, and the sources of the `devs` of `disk.raw` take the name of a
    vol, which stands for the device the vol exposes, as the `dev` of `fs` and the `pvs` of `disk.vg` did:
    `dev = data` encrypts the block volume a pool served. All five also take the id of a resource of the service,
    as `disk#1`, which stands for the devices it exposes, rather than a `{disk#1.exposed_devs[0]}` reference. A keyword
    naming one device needs a resource exposing one, and a resource can not name itself, directly or through others.
    The device of a vol that is not available, or of a resource not started, is not there: the resource is `down`, and
    its unprovision leaves the storage to the unprovision of the vol or the resource below.

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
  
* **`kill` defaults to `tree`:**
    The default behaviour is now to kill all processes with the matching `OPENSVC_ID` and `OPENSVC_RID` variables in their environment, and the status is up while one runs.

    In 2.1 the default was `parent`, and both modes tried to identify the processes matching the start command in the process command line, and having the matching env vars, but this guess is not accurate enough as processes can change their cmdline via PRCTL or via execv.

    With `kill = parent`, the stop terminates, and the status looks for, the process the start command launched, which the start records by its pid. The processes it started are left running.

    An app relying on the 2.1 default to leave the processes it started running, for example a server starting a process per session, sets `kill = parent`.

* **Status**
    The instance container resources are no longer considered when evaluating if the instance is sufficiently started to execute the app checker. Services that have only a container resource as witness can add a `fs.flag`.

### Driver: task

* **`log` defaults to false:**
    In v2, the outputs of a task run were logged by default. They are logged
    with `log = true` only, which logs the scheduled runs of the task too.

* **A container task logs its run output as a host task does:**
    With `log = true`, the stdout lines of the container are logged at info
    level and its stderr lines at warn level, at every run, read from the
    logs of the container after it exits. They were logged on the runs
    creating the container only, whatever `log` said.

* **A container task runs a container per run with `max_parallel` above 1:**
    As in v2, each run has a container of its own, named `<name>.<exec id>`,
    the `name` keyword or the default container name suffixed by the exec id
    of the run, which `om daemon exec list` shows. The container is removed at
    the end of its run, whatever its exit code, and such a task needs
    `rm = true`: a run of a task setting `max_parallel` above 1 with `rm`
    false, its default, is refused. A run finding the container of the task
    running, another run or one left behind, fails rather than counting as a
    run that succeeded.

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

* **Action logs reported to the collector:**
    The actions started by the daemon scheduler, as the scheduled task runs and sync updates, are no longer reported to the collector, and the `node.dblogcron` keyword is not supported.

* **Scheduled runs are logged on demand:**
    The jobs the daemon scheduler starts are not logged in the journal, unless
    the daemon runs with `--debug` or `--trace`, or the job asks it: the
    compliance runs, of the objects and of the node, always do, and the task
    runs and the sync updates do when their `log` keyword is true. A job not
    logged leaves its status, its exit code and the exec events.

### Heartbeat: relay

The v3 agent needs to address a v3 relay.

The v3 relay must have a user with the `heartbeat` grant that the client will need to use.
```
om system/usr/relayuser create --kw grant=heartbeat
om system/usr/relayuser key add --name password --value $PASSWORD
```

On the cluster nodes, store the relay password in a secret:
```
om system/sec/relay-v3 create
om system/sec/relay-v3 key add --name password --value $PASSWORD
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

### Orchestration

* A purge unprovisions the instances whatever their provisioned state, as v2 did.

    A purge used to delete an instance read as not provisioned straight away. The provisioned state of an instance aggregates the resources having one, and a resource with none, as the container of an oci resource, may hold what only its unprovision releases: the container was left on the node, and a new object of the same name refused to provision.

    The unprovision of a resource recorded not provisioned stops it and skips its unprovisioner, as in v2: `om <path> instance unprovision` no longer runs the unprovisioner of such a resource, the record saying there is nothing it made to remove.

* A start asked of a frozen object no longer unfreezes it.

    In v2 and in earlier v3, `om <path> start` on a frozen instance removed the freeze and then started. The object ended up started and unfrozen, so a freeze an operator had set was discarded to serve the request.

    It now starts the instance and leaves the freeze as it was found. Freezing means the daemon may not act by itself, which it still does not: a frozen instance is never started by the HA orchestration, and a frozen node is passed over when choosing where to start. What changes is only the start a user asked for, which is honoured rather than used as a reason to thaw.

    Scripts that relied on `start` to clear a freeze must now ask for it: `om <path> unfreeze && om <path> start`. Note also that `om <path> start --wait` no longer waits for the object to be unfrozen, only for it to be up.

* Freezing and unfreezing are now purely operator decisions. No orchestration sets or clears the frozen flag.

    A stop used to freeze every instance of the object, which is how it kept the HA orchestration from starting it back. Creating an object froze it, a configuration fetched from a peer arrived frozen, and a provision unfroze. An operator could no longer tell their own freeze from one of those, and, since a start no longer thaws, an object that had been stopped and started again ran on with its failover and its resource restart silently disabled.

    All of them now use a flag of their own: the instance is flagged **stopped on purpose**, which the daemon reads as "do not start this on my own initiative", and nothing else. `om <path> print status` shows it as `stopped`, `om mon` as a `=`, and the instance status carries it as `stopped_at`. It is a flag file in the instance var directory, like the frozen flag, so it survives a daemon restart and a reboot.

    The flag is raised by `om <path> stop`, by `om <path> instance stop`, and when an object is created or its configuration lands on a node that had no instance of it. It is lowered by a start, a restart, a switch or a provision the user asks for — on every instance of the object, so the ones that stay down are still failover candidates — and by the instance being seen up again.

    What does not change: `monitor_action = freezestop` still freezes, which is its point, and a node shutdown still freezes the node.

    On upgrade, instances frozen by an older version's stop or create stay frozen, and nothing lifts those freezes any more. Run `om <selector> print status` to find them, and `om <path> unfreeze` on the ones you did not freeze yourself.

* A provision off the placement leader leaves the instance as it found it.

    A provision without `--leader` and without `--disable-rollback` ends by stopping what it started, so the leader is the one to start the object. v2 rolled back the starts of the provision; earlier v3 stopped the whole selection, as a stop asked by the user would: the instance was flagged stopped on purpose, a resource provisioned with `--rid` was flagged stopped (`X`), and a resource provisioned with `--rid` on a running instance was stopped there.

    The stop ending a provision is now a step of it, and flags nothing. It stops only the resources the provision found down, and nothing at all on an instance that was running before it, judged on the resources provisioned already, so a resource added to a running instance is provisioned and left started.

* A node coming back from down adopts only a freeze of the cluster or of an object.

    In v2 and in earlier v3, a daemon joining the cluster froze the node when any peer node had been frozen while it was down, and froze an ha instance when any peer instance had. A node frozen alone for its maintenance froze its peers as they rebooted, and a freeze the daemon took on its own, at the end of a rejoin grace period, spread from node to node through their restarts. A daemon restart also published the node as frozen since its start until it had read its frozen flag, so the daemons restarting with it, as `om daemon restart --node='*'` does, froze on a freeze nobody asked for.

    A node now adopts a freeze of the whole cluster, `om cluster freeze`, and an instance a freeze of the whole object, `om <path> freeze`, that a peer took while the node was down. A node or an instance frozen alone, with `om node freeze` or `om <path> instance freeze`, by a drain, at boot, or by the daemon on its own, stays the only one frozen, and so does a freeze adopted. The node and instance statuses say which a freeze is in `frozen_scope`: `cluster` or `node`, `object` or `instance`.

    On upgrade, the flags of the freezes taken before say nothing of their scope, and read as freezes of the node or the instance alone: a node down across the upgrade does not adopt them.

* An orchestration says how it went, and is waited on by its id.

    `ObjectOrchestrationEnd` and `NodeOrchestrationEnd` carry `failed` and `error`, and the orchestration table records them: an orchestration ends when every node is done with it, whether it did what was asked or gave up, so the end was not a verdict and every client had to read the states back and judge for itself. Any node answers, the one that accepted the orchestration from what it published and the others from the state each instance monitor drops the orchestration id with.

    A request the monitor refuses no longer takes the id its requester was handed. Whether the id was taken on used to be decided by whether anything had changed the monitor in the same pass, which is not the same thing: a start asked right after a create, refused because the peer monitors were not known yet, left the monitor naming an orchestration that never ran, with no global expect to reach and so nothing to end it, and every later request on that object was refused as "already in progress" until an `om <path> abort` cleared it.

    A node request that changes nothing, which is what an abort with nothing to abort is, is now refused with its reason rather than accepted silently: the id its requester was handed named an orchestration nobody would ever hear of, and a client waiting on it waited for the whole of its patience. The object monitor already answered this way.

    `om daemon orchestration wait <id>`, `om daemon session wait <id>` and `om daemon exec wait <id>` wait for one to end and report how it went, exiting non-zero when it failed. The daemon holds the request until then — the `wait` query parameter of `GetDaemonOrchestration`, `GetDaemonExec` and `GetDaemonExecs` — and answers 408 when the wait expires with the work still running. One request is held for an hour at most, and a longer wait is that request asked again: what is waited for outlives it, so asking again resumes the wait rather than restarting it. A wait command asked for a duration that is not positive is refused. An orchestration is answered by any node, so a client that reached the cluster through a floating address follows one wherever the address now points.

    `om <path> <action> --wait` and the node actions that wait (`om cluster freeze|unfreeze --wait`, `om node drain|abort --wait`) now wait this way instead of watching the event stream, the object actions for the event that ends the orchestration and the node ones for a node monitor state to go by. An end event missed is missed for good, which is what made a slow or reconnecting client wait for something that had already happened; the orchestration outlives the request in the daemon, which answers late askers with the same verdict. The per-action assertions the wait used to make on the object status are gone with it: they were a second description of what the daemon already knows, and they went stale twice, waiting for a freeze a stop no longer sets and for a thaw a provision no longer does.

* `orchestrate = start` starts the object when its node comes up.

    The value was documented but never implemented: the daemon only ever started an object on its own when `orchestrate = ha`, so an `orchestrate = start` object stayed down after a reboot, whatever it was running before.

    It now starts the object on the first daemon start that follows a node boot, and moves it nowhere afterwards. A failover object is started if it does not already run elsewhere and the local node is the natural placement leader. A flex object is started if fewer than `flex_target` instances are up and the local node is one of the `flex_target` first natural placement leaders. Outside of that boot, nothing: an instance that goes down is not restarted, and `flex_target` is not chased.

    The natural placement leader is asked, and not the leader among the instances that could start now, which is what the ha orchestration asks: handing the object to a peer because this node cannot take it is a failover, which is the half of `ha` that `orchestrate = start` does not want. A frozen node, a frozen instance and an instance flagged stopped on purpose are all left alone, so a stop asked before a reboot outlives it.

* Flex
  * A `flex_target` value under `flex_min` is forced to `flex_min`. A warning is logged.
  * A `flex_target` value above `flex_max` is forced to `flex_max`. A warning is logged.
  * The `flex_target` value default is the value of `flex_min`.
  * The `flex_min` value default is `1`.

## Enhancements

### Core
    
* The `set` and `unset` commands are complemented by `update --set ... --unset ... --delete`. This new command allow to have a single commit for different kind of changes. The set and unset commands are now hidden so users don't get tempted to use them anymore.
    
* New placement policy `last start`. Use the mtime of `<objvar>/last_start` as the candidate sort key. More recent has higher priority.

* Add --quiet to disable both the progress renderer and the console logging

* New fields in print schedule json format: node, path

* The `monitor_action` now accepts a secondary action, allowing the very useful `freezestop reboot` configuration.

* The `om node update ssh keys --node=...` command is deprecated in favor of `o[mx] cluster ssh trust` (configure the trust mesh on all cluster nodes) and `o[mx] node ssh trust` (trust the node's peers)

* New `o[mx] cluster enroll --node <addr> --token <path>` command, moving a node from its cluster to another one
  without a shell on that node.

    The command is run against a node of the target cluster, and posts to the new `POST /cluster/enroll` endpoint. 
    The token is an access token with the join role, created on the node to enroll: its `ca` claim is the trust anchor for that node certificate, and it authenticates the order the target cluster then posts back to it, which forks a `om cluster join` in the background. Only a token carrying the join role holds that claim, so a root token is not enough and no certificate chain is added to a root-only token.

    The node to enroll must be a single node cluster. Enrolling a node that still has peers is refused with a 
    409: nothing in the join flow tells them to drop it from their `cluster.nodes`, so they would keep it forever.

    The `--join-addr` gives the location the enrolled node must use to reach the target cluster, for a node that cannot
    resolve the target nodenames. It is refused when the cluster certificate is not valid for that host, so a mismatch
    is reported by the command instead of failing later inside the join running on the enrolled node. It defaults to a 
    name the certificate is valid for.

    Use `--wait` to block until the enrolled node heartbeat beats in the target cluster, which is what proves the
    join completed. Without it, the command returns as soon as the node has accepted the order.
    Beware, the node is drained: a single node cluster has nowhere to relocate its instances, so they are stopped, stay
    down, and removed from config.

* New `o[mx] cluster evict --node <node>` command, removing a node from the cluster without a shell on that node.

    The command is run against a node that stays, and posts to the new `POST /cluster/evict` endpoint. That node
    orders the node to evict to leave, through the new `POST /node/name/{nodename}/daemon/action/leave` endpoint,
    which forks a `om cluster leave` in the background.

    The node to evict must be drained, and is refused with a 409 when it is not: nothing in the leave flow stops what
    it still runs, so its instances would stay up on a node the cluster no longer knows about, free to start a second
    time elsewhere. Drain it first with `om node drain --node <node> --wait`.

    Evicting the node the command is posted to is refused: it would have to order itself to leave, and a leave needs a
    peer to be removed by. Post to one of its peers instead.

    The `--credential` names a file holding the `<username>:<password>` of a user to create on the evicted node once
    it is alone. That node ends up with a cluster secret of its own, so no user, token or certificate of the cluster
    it left reaches its api anymore: without this, the api is only reachable from a root shell on that node, through
    the unix socket. The user is created in the `system` namespace, with the `root` grant.

    Use `--wait` to block until the cluster nodes are updated. Beware, that only says the cluster dropped the node:
    the daemon restart and the user creation happen afterwards, on a node this cluster no longer observes.

* `ox context login` logs in at the openid issuer the cluster trusts, when it trusts one: in the browser of the machine, the issuer redirecting to `http://127.0.0.1:<port>/callback`, which the issuer must allow for the client the cluster names, or with a device code from a browser anywhere, when the machine has no browser or with `--device`. `--auth password` logs in with the password of the context user, as before. A context logging in at the openid issuer needs no user: `ox context add` takes no `--user` then, and the connection selector of the terminal ui offers it.

    The openid tokens are shared by the contexts whose clusters trust the same issuer and client: a context logging in, or used before any login, where a valid refresh token is cached needs no challenge. They are kept in the keyring of the session, else in a file encrypted with a key derived from a signature of an ed25519 or rsa key of the ssh-agent, else in a file only the user reads. `--cache keyring|agent|file` chooses, and `ox context list` shows the store and the issuer of each context.

* The `om cluster leave` command accepts `--credential <path>`, naming a file holding the `<username>:<password>` of a
   user to create once the daemon has restarted alone, for the reason `o[mx] cluster evict` does. The
   `OSVC_CREDENTIAL` environment variable is read when the option is not set. Without either, no user is created and
   the node api stays reachable from a root shell only.

* New `o[mx] cluster register` command, registering every cluster node on the collector in one call.

    The collector mints a registration id for a nodename, so a node cannot register on behalf of another: the command
    posts to the new `POST /node/name/{nodename}/action/register` endpoint of each node, and each node registers
    itself and stores its own id in its `node.uuid` keyword. A node already registered is registered again, with a
    new id.

    Each node registers in the background, so the command reports the order being taken, not the registration being
    done: a registration sends the initial asset, package and disk inventories, which takes longer than a request
    should be held open for. Follow it with the `ExecSuccess` and `ExecFailed` events carrying the answered exec id.

    The `--credential` names a file holding the `<username>:<password>` of a collector user, and the
    `OSVC_COLLECTOR_CREDENTIAL` environment variable is read when the option is not set, so the password never
    appears in the process table. Without either, each node registers with the id it already holds, as
    `om node register` without a user does. Use `--app` to name the app to register the nodes in.

    Beware, the credential is forwarded to every cluster node, which is what lets each one register itself.

* `o[mx] node register --node <selector>` registers the selected nodes, through the new register endpoint. The
    option was already offered, but no command behind it: it failed with `RemoteFunc is nil`.

* New `POST /cluster/register` endpoint, registering every node of the cluster of the requested api node on the
    collector. It forks a `om cluster register` in the background, so the HTTP response is sent when the command has
    been forked, not when the nodes are registered. Its optional `credential` parameter is handed to that command
    through its environment, never on its command line.

* The `om cluster join` command accepts `--addr` to reach the `--node` at an explicit location, for a node that cannot
   resolve the target nodename. Its `--token` names a file holding the token, and the `OSVC_JOIN_TOKEN` environment
   variable is read when the option is not set, so the token never appears in the process table.

### Daemon

* The daemon process name is changed from `/usr/bin/python3 -m opensvc.daemon` to `om daemon run`. Monitoring checks may need to adapt.

* A node reads the heartbeat messages of a node running a later version.

    A monitor state, an expected state, a status or a placement value a
    node has no name for is read as `unknown`, or `undef`, instead of failing
    the whole message, and a node whose message decrypts is counted alive
    whether or not it decodes, with a warning saying its data is not applied.
    A node used to find a peer running a later version dead as soon as the
    peer published a value it did not know, and the split action or a
    failover followed.

    An orchestration a node does not know is left to the nodes that do: the
    node does not adopt it, and reads as done to them.

    This holds from this release on, for the upgrades to later ones. To
    upgrade a cluster node by node, freeze it first (`om cluster freeze`),
    upgrade every node, and thaw it (`om cluster unfreeze`) once all run the
    same version.

* Add a 60 seconds timeout to `pre_monitor_action`. The 2.1 daemon waits forever for this callout to terminate.

* A switch or a takeover starts the destination with the resource files of the source.

    The resource files, as the definition of a kvm domain or the configuration of a lxc container, were fetched from an instance up only. A file changed shortly before a move is listed first by the status the source refreshes at the end of its stop, when no instance is up any more, and the destination started without it: "definition not found", start failed.

    The source stays the authority for its files while the orchestration that stopped it lasts. The destination waits, in the new `wait files` monitor state, to hold the files the source lists in the version it lists, compared by checksum, and starts once they are fetched. Past one minute, its start fails, the log naming the files and the source.

* Earlier local object instance orchestration after node boot

    In 2.1 local object instance orchestration waits for all local object instances boot action done
    
    Now object instance <foo@localhost> orchestration only waits for <foo@localhost> boot action completed. Each instance has a last boot id.

* The daemon now resets the local_expect=started instance monitor state when a sysadmin stops a resource, preventing automatic resource restarts.

    In version 2.1, a partially stopped instance caused by executing om foo stop --rid xx could inadvertently be restarted by the resource monitoring subsystem.

* The instance resource info is reported to the collector by the collector speaker, like the instance status already was.

    Every node used to post the resource info of its own instances straight to the collector. The refreshing node now only signals its peers, and the speaker fetches the key-values and reports them on its own throttled schedule, so a cluster talks to the collector through a single node and coalesces what it sends.

* The instance actions are reported to the oc3 collector, with their log lines.

    The begin and the end of an instance action are sent to the oc3 feeder `/api/instance/action`, the end with the log lines of the action, info level and above, read from the journal of the node it ran on by `EXEC_ID` and `OBJ_PATH`. As the resource info, they go through the collector speaker: the node running the action keeps the begin and the end in `<var>/node/collector/action_pending/`, and announces them to the speaker until it acknowledges them, so no report is lost to a daemon down during the action, to a speaker change, or to a collector down. A report not acknowledged within 24 hours is dropped.

    A restart, or a provision and the stop ending it, report once. The actions started by the daemon scheduler, and the actions of volatile or disabled objects, are not reported. `node.dblog = false` disables the reports.

    The speaker names the node an action ran on, and the node of an instance resource info, as `nodename`: an oc3 feeder api 3.0.5 or later is required to file them under that node rather than under the speaker.

    The new `collector.action_batch` (default `100`) and `collector.action_log_timeout` (default `10s`) keywords tune the sends, and `collector.timeout` bounds each of them.

* A collector down no longer floods the daemon log.

    Each feed of the collector speaker, the daemon status, the resource info, the instance config and the action logs, logs its failing sends at debug level, and warns once, then at an interval doubling from 10s up to 1 hour, with the count of items pending. A feed sending again tells the collector accepts it again.

    The resource info, instance config and action log sends stop at the first failure, keeping their queue for the next tick, so a collector down costs one send per feed and tick instead of one per queued item. A 4xx response refuses an item for good: it is dropped with a warning instead of being sent again.

* New prometheus metrics of the collector speaker, on `/metrics`:

    * `opensvc_collector_requests_total{feed,code}`: the collector calls, by feed (`daemon_ping`, `daemon_status`, `daemon_change`, `resource_info`, `object_config`, `instance_action`) and status code, `error` for a call without response. Each call carries one item, so its rate is the push rate.
    * `opensvc_collector_pending{feed}`: the items the speaker holds for the collector, what piles up while it is down or not configured yet. Zero on the other nodes.
    * `opensvc_collector_action_pending_files`: on every node, the local instance actions whose begin or end the collector did not acknowledge yet. Summed over the cluster, the action backlog, speaker or not.

    ```
    # push rate accepted by the collector, and backlog
    sum by (feed) (rate(opensvc_collector_requests_total{code="202"}[5m]))
    sum by (feed) (opensvc_collector_pending)
    ```

* The multicast and unicast heartbeats run on IPv6.

    An IPv6 multicast group, as `ff02::1:4141`, did not resolve, and an IPv6 unicast address was dialed, listened on and allowed as `addr:port`, which only an IPv4 address can be written as: the heartbeat never started, or refused all its peers. The addresses are now written `[addr]:port`, and a peer address written in any of its forms, as `FD01:0:0:0::12`, is allowed.

    An IPv6 multicast group is joined on the interface `intf` names, which it requires on every node of the heartbeat: a cluster configuration setting one without, or naming a zone in the group (`ff02::1:4141%eth0`), is refused. An IPv6 link-local unicast address, as `fe80::12%eth0`, is not supported and is refused too.

    A heartbeat whose configuration its driver refuses is listed by `om daemon hb list` as failed, with the error as an alert of its streams, and is tried again every 30 seconds. It was not listed at all.

* A heartbeat stream failing its start is listed failed with the error as an alert, and a stream stopped by `om daemon hb stop` is listed stopped while its section stays: both stayed listed after their section was deleted, until the daemon restarted.

* Two unicast heartbeats listening on the same port of an address both listen on, on a node of both, are refused by the cluster configuration check, as `hb#1` and `hb#u5` both on the default port 10000.

    The rx of the second could not bind, and its tx was seen beating, its frames received by the rx of the first: `om daemon hb list` showed a heartbeat half healthy while the cluster had one network path less. Give the second another `port`, or another `addr` on each node. A cluster configuration already holding such a pair refuses the next writes until it is fixed.

* A disk heartbeat beats on a disk that held something else before it was signed.

    `om daemon hb sign` wrote the header page only: the metadata area kept what the disk held, as a filesystem, and the nodes took any page not starting with a zero as a slot held, claiming slots past the end of the disk, every write failing, with no alert. The sign now zeroes the metadata area, `max_slots` pages of 4 KiB, of a disk with no heartbeat header, and refuses one holding another signature, as a filesystem or a lvm physical volume, unless `--force`: the `force` parameter of `POST /api/node/name/{nodename}/daemon/hb/name/{name}/action/sign`, which answers 409. A disk with a heartbeat header keeps its slots, and the ones of the other clusters sharing it. `om daemon hb wipe` zeroes the metadata area with the header, so a sign after it starts from free slots. A disk smaller than the metadata area and 1 MiB per node, plus 1 MiB, is refused by the sign, and fails the start of the streams.

    The streams use the slots whose data fits on the disk only, read at each claim, and take a page naming no node, as what a filesystem left, as free. A transmitter failing to write, or to claim a slot, is listed failed with the error as an alert, and its peers not beating at once; it was listed running with no alert, the error logged at each interval. The slot a stream writes or reads is listed once claimed or read: it stayed the one of the start, `slot 0` or `[?]`.

    A `dev` of `/dev/dm-<n>` is refused, as in v2, its number changing across reboots, and a `/dev/sd<x>` one is warned of: both checks were inverted, and accepted any path.

### sec

* Add "o[mx] key rename --name old --to new" commands

### svc

* Support kvm live migration with data on `disk.disk` and `disk.drbd`. The commands to trigger a live migration are

    om svc1 switch --live
    om svc1 takeover --live

### Driver
#### disk.sgcp_nfs_cg, fs.sgcp_nfs, ip.sgcp_dnsalias

* New `OSVC_SGCP_CACHE` environment variable to override the sgcp api cache policy.

    By default, the api results are served from a local cache to:

    * the status evaluations run by the daemon scheduler, so the scheduler does not load the provider api.
    * the status evaluations run by an action with a resource selection that neither includes the sgcp resource
      nor requires it via a `<action>_requires` keyword of a selected resource, like `om foo app start`, as the
      action does not depend on it. An action with `--to` is not a resource selection.

    Every other action, an operator status, an action on or requiring the sgcp resource, or the status evaluation
    of the sgcp resource object by an action on another object (e.g. the hard affinity checks of a start) first of
    all, reads the api again.
    The variable overrides this default whatever the action:

    * `1`: serve the cached values, e.g. for tests or repeated manual status checks.
    * `0`: never serve the cached values, the daemon scheduler included, e.g. to debug a stale status.

    Any other value keeps the default policy, with a warning in the resource status.

    A cached value is only served while younger than the `cache.ttl_seconds` value from the sgcp configuration
    file, and a zero ttl disables the cache, whatever `OSVC_SGCP_CACHE` says.

    Set it in `/etc/default/opensvc` or `/etc/sysconfig/opensvc` for the daemon, or in the environment of a single
    command.

## Upgrade from b2.1

### Activation on boot

* systemctl enable --now opensvc-server

### Cluster Config

* Need to set explicitly the `cluster.name` because the v3 daemon will generate a random cluster name if none is set:

    ```
    # Ensure cluster.name is defined before upgrade to v3
    om cluster set --kw cluster.name=$(om cluster eval --kw cluster.name)
    ```

* Rename the stonith sections `cmd` option to `command`. Backward compatibility is implemented.

* `om cluster config migrate` and `om node config migrate` write the cluster and node configurations in the shape om3 reads them in: the keywords under a former name, `ips_per_node`, `listener.openid_well_known`, the `brocade` schedule, and the removal of what om3 no longer reads. `--dry-run` prints the changes first, and the help lists the rules.

### DNS

* Reinstall system/svc/dns if installed. Make sure the recursor and server configuration in system/cfg/dns are reapplied (allow-from is often customized).

### Network

* Flush iptables rules created by om2. om3 now configures the firewall using nft only.

* The nft rules moved to the `osvc` table, one per address family. They used to
  go in `nat` and `filter`, where the base chain om adds sits on a hook and a
  priority the standard chain of those tables already uses, which makes them
  unrepresentable to the iptables compatibility layer: `iptables -t nat -S`
  answered ``table `nat' is incompatible, use 'nft' tool``, and every firewall
  driver reaching the ruleset through iptables went blind on them. netavark is
  one of those, so no podman container of a podman-built network could start on
  a node `om net setup` had run on.

  A setup deletes the chains left in `nat` and `filter` by an earlier om, so a
  node converts itself the first time it runs one.

* Change `ips_per_node` to `mask_per_node`. The former was inadequate for large subnets (ipv6). For example, `ips_per_node=18446744073709551616` is easier expressed as `mask_per_node=64`. Backward compatibility is maintained for this release.



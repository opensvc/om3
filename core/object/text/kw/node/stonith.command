Shoot The Other Node In The Head, aka fence, using a callout.

The callout is triggered after a quorum vote won, when the surviving node is
about to start a local instance of a service that was known to be started on
a unreachable peer node.

The callout is meant to prevent the peer from writing to shared disks, remote
databases, and from responding to clients.

The [Fence Agents](https://github.com/ClusterLabs/fence-agents) project is a
well known bundle of callout used by many clustering tools.

package manifest

var (
	ContextPath = Context{
		Key:  "path",
		Attr: "Path",
		Ref:  "object.path",
	}
	ContextDRPNodes = Context{
		Key:  "drpnodes",
		Attr: "DRPNodes",
		Ref:  "object.drpnodes",
	}
	ContextNodes = Context{
		Key:  "nodes",
		Attr: "Nodes",
		Ref:  "object.nodes",
	}
	ContextObjectID = Context{
		Key:  "objectID",
		Attr: "ObjectID",
		Ref:  "object.id",
	}
	ContextDNS = Context{
		Key:  "dns",
		Attr: "DNS",
		Ref:  "node.dns",
	}
	ContextPeers = Context{
		Key:  "peers",
		Attr: "Peers",
		Ref:  "object.peers",
	}
	ContextTopology = Context{
		Key:  "topology",
		Attr: "Topology",
		Ref:  "object.topology",
	}
	ContextCNIPlugins = Context{
		Key:  "cni_plugins",
		Attr: "CNIPlugins",
		Ref:  "cni.plugins",
	}
	ContextCNIConfig = Context{
		Key:  "cni_config",
		Attr: "CNIConfig",
		Ref:  "cni.config",
	}
)

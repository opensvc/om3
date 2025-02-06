package manifest

var (
	ContextObjectPath = Context{
		Key:  "path",
		Attr: "Path",
		Ref:  "object.path",
	}
	ContextEncapNodes = Context{
		Key:  "encapnodes",
		Attr: "EncapNodes",
		Ref:  "object.encapnodes",
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
	ContextObjectFQDN = Context{
		Key:  "object_fqdn",
		Attr: "ObjectFQDN",
		Ref:  "object.fqdn",
	}
	ContextObjectDomain = Context{
		Key:  "object_domain",
		Attr: "ObjectDomain",
		Ref:  "object.domain",
	}
	ContextObjectID = Context{
		Key:  "objectID",
		Attr: "ObjectID",
		Ref:  "object.id",
	}
	ContextObjectParents = Context{
		Key:  "object_parents",
		Attr: "ObjectParents",
		Ref:  "object.parents",
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

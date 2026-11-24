package commoncmd

import (
	"github.com/spf13/cobra"
)

// CmdTemplateLong documents ox template.
const CmdTemplateLong = `Render a Go template from the cluster data and the datastore keys.

The template is written to the --out file, as a whole: a reader sees the
former content or the new, never a part. The file is written only when its
content changes, and the --reload-command, an executable file run with no
argument and no shell, is run after each write, with OX_TEMPLATE_OUT set to
the file.

With --watch, the command renders the template again each time something it
read changes, after a --delay quiet period, at most --max-delay after the
first change. It follows the events of what the last render read: a read under
a condition, in a range, or of a computed name, is followed when the data led
the render to it. A render failing, as on a key removed, keeps the last file,
and the command goes on watching. SIGHUP reads the template again.

--config renders several templates from one process, sharing its events
stream and its cluster data. Each template has its file, mode, owner and
reload command; all the templates changed are rendered before a file is
written, and a reload command named by several templates runs once, after
their files are written, with OX_TEMPLATE_OUT naming them, space separated.
A template failing to render keeps its file, the others are written. --var
replaces a var of the file; SIGHUP reads the templates again, not the file:

  vars:
    env: prod
  templates:
    - in: /etc/ox/haproxy.cfg.tpl
      out: /etc/haproxy/haproxy.cfg
      mode: "0644"
      reload_command: /usr/local/bin/haproxy-reload
    - in: /etc/ox/certs.pem.tpl
      out: /etc/haproxy/certs.pem
      reload_command: /usr/local/bin/haproxy-reload
      reload_timeout: 30s
    - in: /etc/ox/targets.json.tpl
      out: /etc/prometheus/targets.json
      mode: "0644"

--debug logs the events stream subscription, the reads changed and the events
leading to each render, and the api calls, never the values they return.

Paths:
  A path is a full path, as prod/cfg/db, or relative to --namespace when its
  namespace is ".", as ./cfg/db. A path of two elements, as cfg/db, is of
  the root namespace.

Datastore keys (cfg and sec, read through the api):
  key "path" "name"            the decoded value, an error when missing
  keyOr "path" "name" "def"    the value, def when missing
  keyExists "path" "name"      true when the key exists
  keyNames "path" ["glob"]     the sorted key names

  Reading a sec key needs the admin role on its namespace. A sec read with
  a --mode letting others read the file is warned about.

Keywords, as a node evaluates them:
  config "path" "section.option"         as the node serving the api
  configAs "path" "section.option" "node"

Objects:
  objects ["selector"]         the objects, by path selector
  objectStatus "path"          the object, an error when missing
  objectExists "path"          true when the object has a status
  avail "path"                 its availability, an error when missing
  availOr "path" "def"         its availability, def when missing

Instances and resources:
  instances ["selector"]       the instances, with .monitor
  instanceStatus "path" "node" the instance, an error when missing
  resources ["selector"] ["rid pattern"]
  ips ["selector"]             the ip resources, with .ipaddr .dev .netmask
  disks ["selector"]           the disk resources
  apps ["selector"]            the app resources
  resinfo "path" "rid" "key" ["node"]   a resource information

Nodes and networks:
  nodes ["selector"]           by name pattern or label=value, comma separated
  nodeLabel "node" "name"      the value of a node label
  networks ["name pattern"]    the networks of the nodes

  A selector of objects is made of path patterns, as prod/svc/web*, joined
  by , (or), + (and), ! (not). A range with no selector follows the events
  of every object of the cluster.

  The fields are named as the api names them, in snake case: .path .node
  .avail .status .rid .name .labels ...

Helpers:
  contains hasPrefix hasSuffix lower upper trim trimPrefix trimSuffix replace
  split fields join quote indent nindent default toJSON fromJSON toYAML b64enc
  b64dec sha256sum reMatch fnMatch sortAlpha

Example:
  backend web
  {{- range ips "prod/svc/web" }}{{ if eq .status "up" }}
      server {{ .node }} {{ .ipaddr }}:8080 check
  {{- end }}{{ end }}
`

func NewCmdTemplate() *cobra.Command {
	return &cobra.Command{
		GroupID: GroupIDQuery,
		Use:     "template",
		Short:   "render a template from the cluster data, and watch it",
		Long:    CmdTemplateLong,
		Args:    cobra.NoArgs,
	}
}

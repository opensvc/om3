# opensvc agent Changelog

## v3.0.0

### core

* **breaking change:** stop matching DEFAULT.<string> for "<string>:" object selector expressions. Match only sections basename (like in [<basename>#<index>]).

* **breaking change:** drop backward compatibility for the always_on=<nodes> keyword.

* New fields in print schedule json format: node, path

* **breaking change:** new cgroup layout. The previous organization allowed conflicts between different object types, and was hard to read.

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
  

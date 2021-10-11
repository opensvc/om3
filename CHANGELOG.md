# opensvc agent Changelog

## v3.0.0

### core

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


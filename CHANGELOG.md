# opensvc agent Changelog

## v3.0.0

### driver app
* **breaking change:** keyword `environment` now keep var name unchanged (respect mixedCase)
  
        environment = Foo=one bar=2 Z=u
        =>
        Foo=one     was previsouly changed to FOO=one
        bar=2       was previsouly changed to BAR=2
        Zoo=u       was previously changed to ZOO=u


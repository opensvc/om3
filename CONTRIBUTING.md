# Errors

## Do not use github.com/go/pkg/errors

Use the standard "errors" and "fmt" packages instead, that supports error
wrapping since go 1.13.

The `%w` wildcard in `fmt.Errorf` format triggers the error wrapping.

## Position of %w in fmt.Errorf format

1/ %w at head

   Add detail to a generic error.
   Example:

	fmt.Errorf("%w: %s namespace does not exist", os.ErrExist, ns)

2/ %w at tail
   Hint about the codepath leading to the error, and the context of the error.
   Example:

	fmt.Errorf("Creating %s: %w", objectPath, err) 

## Do not capitalize error strings

https://google.github.io/styleguide/go/decisions.html#error-strings

## Do not capitalize cobra command Short description string

And do not terminate with a dot.

## Do not capitalize pflag description string

And do not terminate with a dot.

# Commands

## Name a command that shows things after what it renders

`list`, aliased `ls`, when the answer is a row per thing: a flat table whose
columns are selectable, that `-o json`, `-o tab` and `-o flat` reshape, and
that a script reads. Most of the tree is this.

`status` when the answer is a composed view of one subject, whose layout
carries meaning that rows cannot: the resource tree of `om <object> instance
status`, the node-column board of `om ccfg status`. Reformatting one of those
as a table would lose information.

The test is mechanical: a command whose renderer is a `tab=` column spec is a
listing, whatever its subject. `om daemon hb status`, `om daemon relay status`
and `om node relay status` were listings wearing the other name, and are
`list` now.

## Keep the name a renamed command answered to

Give the new name to the command, and build the old one from it with `Use`
overridden, the aliases cleared and `Hidden: true`. It stays out of the help
and out of the completion, and the scripts that type it keep working.

# Rendering

## Do not use color.Set

`color.Set` is the imperative form of the fatih/color api: it writes the escape
sequence to the package output, which is the process stdout, as a side effect,
and returns the color to render with. A function composing a string therefore
leaves a bare, never reset sequence on the terminal ahead of whatever it
returns, and the first line rendered afterwards inherits it. That is how a
section holding a comment came out italic in `om <obj> config show`.

Use `color.New`. It builds the same color and writes nothing, and its `Fprint`,
`Sprint` and `SprintFunc` open and close the sequence around the text they
render.

## Paint where the output is

An escape sequence belongs to the code writing to the terminal, not to the type
carrying the value. A daemon type is published, read back by api clients and by
the tui, which paints cells of its own: it hands the state over, and the
renderer draws the icon standing for it.


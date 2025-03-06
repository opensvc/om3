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

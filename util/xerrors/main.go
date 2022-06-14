package xerrors

import (
	"fmt"
	"strings"

	"github.com/hashicorp/go-multierror"
)

func New() *multierror.Error {
	var errs multierror.Error
	errs.ErrorFormat = listFormatFunc
	return &errs
}

func Append(err error, errs ...error) *multierror.Error {
	return multierror.Append(err, errs...)
}

func listFormatFunc(es []error) string {
	switch len(es) {
	case 0:
		return ""
	case 1:
		return fmt.Sprint(es[0])
	default:
		points := make([]string, len(es))
		for i, err := range es {
			points[i] = fmt.Sprintf("* %s", err)
		}
		return fmt.Sprintf(
			"%d errors occurred:\n%s",
			len(es), strings.Join(points, "\n"))
	}
}

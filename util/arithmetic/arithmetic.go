// Package arithmetic evaluates the arithmetic a configuration keyword holds,
// so a size can be written as a share of another size rather than as a number
// somebody worked out by hand.
//
//	size = $(50% * {disk#vg.capacity})
//	size = $({DEFAULT.size} / 2)
//	size = $({disk#vg.capacity} - 10g)
//
// The references are resolved before this is asked anything, so what it reads
// is arithmetic over numbers. A number is written the way every other size in
// a configuration is written, so "10g" and "10GB" are sizes here too.
//
// The operators are "+", "-", "*", "/", "//" and "%". The last two are the
// integer division and the remainder of it, so "5//2" is 2 and "5%2" is 1.
//
// A percent sign is also how a share is written, "50%" being a half, and the
// two are told apart by what follows the sign: a number opens the right side
// of a remainder, and anything else ends a share. So "50% * 10g" is half of
// ten gibibytes, and "50 % 2" is what is left of fifty when divided by two.
package arithmetic

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/opensvc/om3/v3/util/sizeconv"
)

// Open is what introduces an expression, and is looked for before anything
// else is done, so a value holding none is not parsed at all.
const Open = "$("

// Eval replaces every expression in s by the number it computes, and leaves
// the rest of s alone.
func Eval(s string) (string, error) {
	var b strings.Builder
	for {
		i := strings.Index(s, Open)
		if i < 0 {
			b.WriteString(s)
			return b.String(), nil
		}
		j, err := closing(s, i+len(Open)-1)
		if err != nil {
			return "", err
		}
		v, err := EvalExpr(s[i+len(Open) : j])
		if err != nil {
			return "", err
		}
		b.WriteString(s[:i])
		b.WriteString(strconv.FormatInt(v, 10))
		s = s[j+1:]
	}
}

// closing is the index of the parenthesis closing the one at open.
func closing(s string, open int) (int, error) {
	depth := 0
	for i := open; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i, nil
			}
		}
	}
	return 0, fmt.Errorf("%s: no closing parenthesis", s[open:])
}

// EvalExpr computes one expression, written without the "$(" and ")" around
// it.
//
// The arithmetic is done in floating point and the answer rounded, because a
// share of a size is a fraction and the caller is asking for a count of
// bytes: a half of an odd number of them is one of them or the other, not an
// error.
func EvalExpr(s string) (int64, error) {
	p := &parser{s: s}
	v, err := p.expr()
	if err != nil {
		return 0, err
	}
	p.skipSpace()
	if p.pos < len(p.s) {
		return 0, fmt.Errorf("%s: unexpected %q", s, p.s[p.pos:])
	}
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, fmt.Errorf("%s: is not a number", s)
	}
	return int64(math.Round(v)), nil
}

type parser struct {
	s   string
	pos int
}

func (p *parser) skipSpace() {
	for p.pos < len(p.s) && (p.s[p.pos] == ' ' || p.s[p.pos] == '\t') {
		p.pos++
	}
}

func (p *parser) peek() byte {
	p.skipSpace()
	if p.pos >= len(p.s) {
		return 0
	}
	return p.s[p.pos]
}

// expr is a sum of terms.
func (p *parser) expr() (float64, error) {
	v, err := p.term()
	if err != nil {
		return 0, err
	}
	for {
		switch p.peek() {
		case '+':
			p.pos++
			r, err := p.term()
			if err != nil {
				return 0, err
			}
			v += r
		case '-':
			p.pos++
			r, err := p.term()
			if err != nil {
				return 0, err
			}
			v -= r
		default:
			return v, nil
		}
	}
}

// term is a product of factors, or what a division of them leaves.
func (p *parser) term() (float64, error) {
	v, err := p.factor()
	if err != nil {
		return 0, err
	}
	for {
		switch p.peek() {
		case '*':
			p.pos++
			r, err := p.factor()
			if err != nil {
				return 0, err
			}
			v *= r
		case '/':
			p.pos++
			// A second slash makes it the integer division, which is what
			// the division of two counts of something usually means.
			whole := p.pos < len(p.s) && p.s[p.pos] == '/'
			if whole {
				p.pos++
			}
			r, err := p.factor()
			if err != nil {
				return 0, err
			}
			if r == 0 {
				return 0, fmt.Errorf("%s: division by zero", p.s)
			}
			v /= r
			if whole {
				v = math.Floor(v)
			}
		case '%':
			p.pos++
			r, err := p.factor()
			if err != nil {
				return 0, err
			}
			if r == 0 {
				return 0, fmt.Errorf("%s: division by zero", p.s)
			}
			v = math.Mod(v, r)
		default:
			return v, nil
		}
	}
}

// factor is a value, with the sign it may be given.
func (p *parser) factor() (float64, error) {
	switch p.peek() {
	case '-':
		p.pos++
		v, err := p.factor()
		return -v, err
	case '+':
		p.pos++
		return p.factor()
	}
	return p.primary()
}

// primary is a number, or an expression in parentheses.
func (p *parser) primary() (float64, error) {
	switch c := p.peek(); {
	case c == 0:
		return 0, fmt.Errorf("%s: ends where a number is expected", p.s)
	case c == '(':
		p.pos++
		v, err := p.expr()
		if err != nil {
			return 0, err
		}
		if p.peek() != ')' {
			return 0, fmt.Errorf("%s: no closing parenthesis", p.s)
		}
		p.pos++
		return v, nil
	default:
		return p.number()
	}
}

// number is a count, of bytes when it carries a size unit and of hundredths
// when it carries a percent sign.
func (p *parser) number() (float64, error) {
	p.skipSpace()
	start := p.pos
	for p.pos < len(p.s) {
		c := p.s[p.pos]
		isPart := c >= '0' && c <= '9' ||
			c == '.' || c == ',' ||
			c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
		if !isPart {
			break
		}
		p.pos++
	}
	token := p.s[start:p.pos]
	if token == "" {
		return 0, fmt.Errorf("%s: %q is not a number", p.s, p.s[start:])
	}
	if p.isShareSign() {
		p.pos++
		v, err := strconv.ParseFloat(strings.ReplaceAll(token, ",", "."), 64)
		if err != nil {
			return 0, fmt.Errorf("%s: %q is not a share", p.s, token+"%")
		}
		return v / 100, nil
	}
	v, err := sizeconv.FromSize(token)
	if err != nil {
		return 0, fmt.Errorf("%s: %q is not a number", p.s, token)
	}
	return float64(v), nil
}

// isShareSign says the percent sign the number just read is followed by ends a
// share rather than opening the right side of a remainder.
//
// The two are told apart by what comes after the sign: a remainder is taken of
// something, so a number follows it, where a share is followed by an operator,
// a closing parenthesis, or nothing at all.
func (p *parser) isShareSign() bool {
	p.skipSpace()
	if p.pos >= len(p.s) || p.s[p.pos] != '%' {
		return false
	}
	for i := p.pos + 1; i < len(p.s); i++ {
		switch c := p.s[i]; {
		case c == ' ' || c == '\t':
			continue
		case c >= '0' && c <= '9', c == '.', c == '(':
			return false
		default:
			return true
		}
	}
	return true
}

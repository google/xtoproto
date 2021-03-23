package sexpr

import (
	"fmt"
	"go/constant"
	"regexp"
	"strconv"
)

type number struct {
	uint64  *uint64
	int64   *int64
	float64 *float64
	// or big.Int, big.Float
}

func (n number) value() interface{} {
	if n.float64 != nil {
		return *n.float64
	}
	if n.int64 != nil {
		return *n.int64
	}
	if n.uint64 != nil {
		return *n.uint64
	}
	return nil
}

func (n number) constValue() constant.Value {
	if n.float64 != nil {
		return constant.MakeFloat64(*n.float64)
	}
	if n.int64 != nil {
		return constant.MakeInt64(*n.int64)
	}
	if n.uint64 != nil {
		return constant.MakeUint64(*n.uint64)
	}
	panic("invalid number value")
}

var possibleNumberRegexp = regexp.MustCompile(`^\d`)

func parseNumber(s string) (*number, error) {
	u, err := strconv.ParseUint(s, 0, 64)
	if err == nil {
		return &number{uint64: &u}, nil
	}
	i, err := strconv.ParseInt(s, 0, 64)
	if err == nil {
		return &number{int64: &i}, nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err == nil {
		return &number{float64: &f}, nil
	}
	// TODO: IF the token is a possible number, return error.
	if possibleNumberRegexp.MatchString(s) {
		return nil, fmt.Errorf("got possible number %q that failed to parse as a number", s)
	}
	// Not implemented because the go2 parser can't handle imports.
	return nil, nil
}

type simpleReaderMacroResult struct {
	skip bool
	form Form
}

func (r simpleReaderMacroResult) Skip() bool {
	return r.skip
}

func (r simpleReaderMacroResult) Form() Form {
	return r.form
}

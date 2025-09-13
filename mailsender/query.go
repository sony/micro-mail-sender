package mailsender

import (
	"regexp"
	"strings"
)

// Type
type QueryType int

const (
	QueryLeaf = iota
	QueryAnd
	QueryOr
)

// Leaf kind
type QueryKind int

const (
	QuerySender = iota
	QueryReceiver
	QuerySubject
	QueryStatus
	QueryMessageId
)

// Leaf operator
type QueryOperator int

const (
	QueryEqual = iota
	QueryNotEqual
)

type QNode interface {
	getType() QueryType
}

type QExpr struct {
	qtype QueryType
	a     QNode
	b     QNode
}

func (t *QExpr) getType() QueryType {
	return t.qtype
}

type QLeaf struct {
	kind  QueryKind
	op    QueryOperator
	value string
}

func (l *QLeaf) getType() QueryType {
	return QueryLeaf
}

// Must correspond to QueryKind
var validQueryKinds = []string{
	"^from_email\\b",
	"^to_email\\b",
	"^subject\\b",
	"^status\\b",
	"^msg_id\\b",
}

// Must Correspond to QueryOperator
var validOperators = []string{"^=", "^!="}

func skipws(input string) string {
	idx := regexp.MustCompile(`\s*`).FindStringIndex(input)
	return input[idx[1]:]

}

func take(input string, n int) string {
	if len(input) <= n {
		return input
	} else {
		return input[0:n] + " ..."
	}
}

func badmsg(input string) *AppError {
	return AppErr(400, "Invalid query near \""+take(input, 20))
}

func parseQueryConj(input string) (string, string, *AppError) {
	input = skipws(input)
	if input == "" {
		return "", "", nil
	}
	if strings.HasPrefix(input, "AND") {
		return "AND", input[len("AND"):], nil
	}
	if strings.HasPrefix(input, "OR") {
		return "OR", input[len("Or"):], nil
	}
	return "", "", badmsg(input)
}

func parseQueryKind(input string) (QueryKind, string, *AppError) {
	for kind, name := range validQueryKinds {
		idx := regexp.MustCompile(name).FindStringIndex(input)
		if len(idx) > 0 {
			return QueryKind(kind), input[idx[1]:], nil
		}
	}
	return 0, "", badmsg(input)
}

func parseOperator(input string) (QueryOperator, string, *AppError) {
	for op, name := range validOperators {
		idx := regexp.MustCompile(name).FindStringIndex(input)
		if len(idx) > 0 {
			return QueryOperator(op), input[idx[1]:], nil
		}
	}
	return 0, "", badmsg(input)
}

func parseValue(input string) (string, string, *AppError) {
	idx := regexp.MustCompile(`^"([^"]|\\")*"`).FindStringIndex(input)
	if len(idx) >= 2 {
		val := regexp.MustCompile(`\\(.)`).
			ReplaceAllString(input[1:idx[1]-1], "$1")
		return val, input[idx[1]:], nil
	}
	return "", "", badmsg(input)
}

func parseQuerySimple(input string) (QNode, string, *AppError) {
	input = skipws(input)
	qkind, input, apperr := parseQueryKind(input)
	if apperr != nil {
		return nil, "", apperr
	}
	input = skipws(input)
	operator, input, apperr := parseOperator(input)
	if apperr != nil {
		return nil, "", apperr
	}
	input = skipws(input)
	value, input, apperr := parseValue(input)
	if apperr != nil {
		return nil, "", apperr
	}
	return &QLeaf{kind: qkind, op: operator, value: value}, input, nil
}

func parseQueryExpr(input string) (QNode, string, *AppError) {
	qa, input, apperr := parseQuerySimple(input)
	if apperr != nil {
		return nil, "", apperr
	}
	var conj string
	var qb QNode
	for {
		conj, input, apperr = parseQueryConj(input)
		if apperr != nil {
			return nil, "", apperr
		}
		if conj == "" { // end
			return qa, "", nil
		}

		qb, input, apperr = parseQueryExpr(input)
		if apperr != nil {
			return nil, "", apperr
		}
		switch conj {
		case "AND":
			qa = &QExpr{QueryAnd, qa, qb}
		case "OR":
			qa = &QExpr{QueryOr, qa, qb}
		}
	}
}

// Entry point
func parseQuery(input string) (QNode, *AppError) {
	q, _, apperr := parseQueryExpr(input)
	if apperr != nil {
		return nil, apperr
	}
	return q, nil
}

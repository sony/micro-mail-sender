package mailsender

import (
	"regexp"
	"strings"
)

// QueryType represents the type of a query node.
type QueryType int

const (
	// QueryLeaf indicates a leaf node in the query tree.
	QueryLeaf = iota
	// QueryAnd indicates a logical AND operation.
	QueryAnd
	// QueryOr indicates a logical OR operation.
	QueryOr
)

// QueryKind represents the field to query in a leaf node.
type QueryKind int

const (
	// QuerySender queries the sender email field.
	QuerySender = iota
	// QueryReceiver queries the receiver email field.
	QueryReceiver
	// QuerySubject queries the subject field.
	QuerySubject
	// QueryStatus queries the message status field.
	QueryStatus
	// QueryMessageID queries the message ID field.
	QueryMessageID
)

// QueryOperator represents a comparison operator in a leaf node.
type QueryOperator int

const (
	// QueryEqual indicates an equality comparison.
	QueryEqual = iota
	// QueryNotEqual indicates an inequality comparison.
	QueryNotEqual
)

// QNode represents a node in the query tree.
type QNode interface {
	getType() QueryType
}

// QExpr represents an expression node combining two query nodes with a logical operator.
type QExpr struct {
	qtype QueryType
	a     QNode
	b     QNode
}

func (t *QExpr) getType() QueryType {
	return t.qtype
}

// QLeaf represents a leaf node containing a field comparison.
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
var skipwsRegexp = regexp.MustCompile(`\s*`)

// skipws skips leading whitespace and returns the remaining input.
func skipws(input string) string {
	idx := skipwsRegexp.FindStringIndex(input)
	return input[idx[1]:]

}

// take returns the first n characters of input, adding "..." if truncated.
func take(input string, n int) string {
	if len(input) <= n {
		return input
	}
	return input[0:n] + " ..."
}

// badmsg returns an error indicating an invalid query near the given input.
func badmsg(input string) *AppError {
	return AppErr(400, "Invalid query near \""+take(input, 20))
}

// parseQueryConj parses a query conjunction (AND/OR) from the input.
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

// parseQueryKind parses a query field kind (e.g., from_email, to_email) from
// the input.
func parseQueryKind(input string) (QueryKind, string, *AppError) {
	for kind, name := range validQueryKinds {
		idx := regexp.MustCompile(name).FindStringIndex(input)
		if len(idx) > 0 {
			return QueryKind(kind), input[idx[1]:], nil
		}
	}
	return 0, "", badmsg(input)
}

// parseOperator parses a comparison operator (= or !=) from the input.
func parseOperator(input string) (QueryOperator, string, *AppError) {
	for op, name := range validOperators {
		idx := regexp.MustCompile(name).FindStringIndex(input)
		if len(idx) > 0 {
			return QueryOperator(op), input[idx[1]:], nil
		}
	}
	return 0, "", badmsg(input)
}

var parseValueRegexp1 = regexp.MustCompile(`^"([^"]|\\")*"`)
var parseValueRegexp2 = regexp.MustCompile(`\\(.)`)

// parseValue parses a quoted string value from the input.
func parseValue(input string) (string, string, *AppError) {
	idx := parseValueRegexp1.FindStringIndex(input)
	if len(idx) >= 2 {
		val := parseValueRegexp2.ReplaceAllString(input[1:idx[1]-1], "$1")
		return val, input[idx[1]:], nil
	}
	return "", "", badmsg(input)
}

// parseQuerySimple parses a simple query expression (field op value).
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

// parseQueryExpr parses a compound query expression with AND/OR conjunctions.
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

// parseQuery parses a query string and returns the corresponding query tree.
func parseQuery(input string) (QNode, *AppError) {
	q, _, apperr := parseQueryExpr(input)
	if apperr != nil {
		return nil, apperr
	}
	return q, nil
}

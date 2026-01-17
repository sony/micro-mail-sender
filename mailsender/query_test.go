package mailsender

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQueryParser(t *testing.T) {
	q, e := parseQuery("from_email=\"abc@example.com\"")
	require.Nil(t, e)
	require.Equal(t, &QLeaf{QuerySender, QueryEqual, "abc@example.com"},
		q)

	q, e = parseQuery("from_email!=\"abc@example.com\"")
	require.Nil(t, e)
	require.Equal(t, &QLeaf{QuerySender, QueryNotEqual, "abc@example.com"},
		q)

	q, e = parseQuery("  from_email   !=    \"abc@example.com\"  ")
	require.Nil(t, e)
	require.Equal(t, &QLeaf{QuerySender, QueryNotEqual, "abc@example.com"},
		q)

	q, e = parseQuery("to_email=\"abc@example.com\"  ")
	require.Nil(t, e)
	require.Equal(t, &QLeaf{QueryReceiver, QueryEqual, "abc@example.com"},
		q)

	q, e = parseQuery("subject=\"Welcome\" ")
	require.Nil(t, e)
	require.Equal(t, &QLeaf{QuerySubject, QueryEqual, "Welcome"},
		q)

	q, e = parseQuery("status=\"delivered\"")
	require.Nil(t, e)
	require.Equal(t, &QLeaf{QueryStatus, QueryEqual, "delivered"},
		q)

	q, e = parseQuery("from_email=\"abc@example.com\" OR from_email=\"def@example.com\"")
	require.Nil(t, e)
	require.Equal(t, &QExpr{QueryOr,
		&QLeaf{QuerySender, QueryEqual, "abc@example.com"},
		&QLeaf{QuerySender, QueryEqual, "def@example.com"}},
		q)

	_, e = parseQuery("nosuchkind=\"xyz\"")
	require.NotNil(t, e)
	_, e = parseQuery("from_email")
	require.NotNil(t, e)
	_, e = parseQuery("from_email=")
	require.NotNil(t, e)
	_, e = parseQuery("from_email~")
	require.NotNil(t, e)
	_, e = parseQuery("from_email=abc")
	require.NotNil(t, e)

}

func TestQLeafGetType(t *testing.T) {
	leaf := &QLeaf{QuerySender, QueryEqual, "test"}
	require.Equal(t, QueryType(QueryLeaf), leaf.getType())
}

func TestQExprGetType(t *testing.T) {
	expr := &QExpr{QueryAnd, nil, nil}
	require.Equal(t, QueryType(QueryAnd), expr.getType())

	expr = &QExpr{QueryOr, nil, nil}
	require.Equal(t, QueryType(QueryOr), expr.getType())
}

func TestTake(t *testing.T) {
	require.Equal(t, "short", take("short", 10))
	require.Equal(t, "0123456789 ...", take("01234567890123456789", 10))
	require.Equal(t, "exact", take("exact", 5))
}

func TestSkipws(t *testing.T) {
	require.Equal(t, "abc", skipws("   abc"))
	require.Equal(t, "abc", skipws("abc"))
	require.Equal(t, "abc  ", skipws("  abc  "))
	require.Equal(t, "", skipws("   "))
}

func TestBadmsg(t *testing.T) {
	err := badmsg("test input")
	require.NotNil(t, err)
	require.Equal(t, 400, err.Code)
	require.Contains(t, err.Message, "Invalid query near")
	require.Contains(t, err.Message, "test input")
}

func TestParseQueryConj(t *testing.T) {
	// Test AND
	conj, rest, err := parseQueryConj("AND rest")
	require.Nil(t, err)
	require.Equal(t, "AND", conj)
	require.Equal(t, " rest", rest)

	// Test OR
	conj, rest, err = parseQueryConj("OR rest")
	require.Nil(t, err)
	require.Equal(t, "OR", conj)
	require.Equal(t, " rest", rest)

	// Test empty string
	conj, rest, err = parseQueryConj("")
	require.Nil(t, err)
	require.Equal(t, "", conj)
	require.Equal(t, "", rest)

	// Test with leading whitespace
	conj, _, err = parseQueryConj("   AND rest")
	require.Nil(t, err)
	require.Equal(t, "AND", conj)

	// Test invalid conjunction
	_, _, err = parseQueryConj("INVALID")
	require.NotNil(t, err)
	require.Equal(t, 400, err.Code)
}

func TestParseQueryKind(t *testing.T) {
	// Test all valid kinds
	kind, rest, err := parseQueryKind("from_email=")
	require.Nil(t, err)
	require.Equal(t, QueryKind(QuerySender), kind)
	require.Equal(t, "=", rest)

	kind, _, err = parseQueryKind("to_email=")
	require.Nil(t, err)
	require.Equal(t, QueryKind(QueryReceiver), kind)

	kind, _, err = parseQueryKind("subject=")
	require.Nil(t, err)
	require.Equal(t, QueryKind(QuerySubject), kind)

	kind, _, err = parseQueryKind("status=")
	require.Nil(t, err)
	require.Equal(t, QueryKind(QueryStatus), kind)

	kind, _, err = parseQueryKind("msg_id=")
	require.Nil(t, err)
	require.Equal(t, QueryKind(QueryMessageID), kind)

	// Test invalid kind
	_, _, err = parseQueryKind("invalid_kind=")
	require.NotNil(t, err)
}

func TestParseOperator(t *testing.T) {
	// Test equal
	op, rest, err := parseOperator("=\"value\"")
	require.Nil(t, err)
	require.Equal(t, QueryOperator(QueryEqual), op)
	require.Equal(t, "\"value\"", rest)

	// Test not equal
	op, rest, err = parseOperator("!=\"value\"")
	require.Nil(t, err)
	require.Equal(t, QueryOperator(QueryNotEqual), op)
	require.Equal(t, "\"value\"", rest)

	// Test invalid operator
	_, _, err = parseOperator("~=\"value\"")
	require.NotNil(t, err)
}

func TestParseValue(t *testing.T) {
	// Test simple value
	val, rest, err := parseValue("\"hello\" rest")
	require.Nil(t, err)
	require.Equal(t, "hello", val)
	require.Equal(t, " rest", rest)

	// Test empty value
	val, rest, err = parseValue("\"\" rest")
	require.Nil(t, err)
	require.Equal(t, "", val)
	require.Equal(t, " rest", rest)

	// Test invalid value (no closing quote)
	_, _, err = parseValue("\"unclosed")
	require.NotNil(t, err)

	// Test invalid value (no opening quote)
	_, _, err = parseValue("noquotes")
	require.NotNil(t, err)
}

func TestParseQuerySimple(t *testing.T) {
	q, rest, err := parseQuerySimple("from_email=\"test@example.com\"")
	require.Nil(t, err)
	require.Equal(t, "", rest)
	leaf := q.(*QLeaf)
	require.Equal(t, QueryKind(QuerySender), leaf.kind)
	require.Equal(t, QueryOperator(QueryEqual), leaf.op)
	require.Equal(t, "test@example.com", leaf.value)
}

func TestParseQueryExprAND(t *testing.T) {
	q, err := parseQuery("from_email=\"a@b.com\" AND to_email=\"c@d.com\"")
	require.Nil(t, err)
	expr := q.(*QExpr)
	require.Equal(t, QueryType(QueryAnd), expr.getType())
}

func TestParseQueryExprMsgID(t *testing.T) {
	q, err := parseQuery("msg_id=\"12345\"")
	require.Nil(t, err)
	leaf := q.(*QLeaf)
	require.Equal(t, QueryKind(QueryMessageID), leaf.kind)
	require.Equal(t, "12345", leaf.value)
}

func TestParseQueryExprComplex(t *testing.T) {
	// Test complex AND OR combination
	q, err := parseQuery("from_email=\"a@b.com\" AND to_email=\"c@d.com\" OR subject=\"test\"")
	require.Nil(t, err)
	require.NotNil(t, q)
}

func TestParseQueryExprErrorAfterConj(t *testing.T) {
	// Test error when parseQueryExpr fails after a valid conjunction
	// This tests line 195-197 in query.go
	_, err := parseQuery("from_email=\"a@b.com\" AND invalid")
	require.NotNil(t, err)
	require.Equal(t, 400, err.Code)
}

func TestParseQueryExprErrorAfterOR(t *testing.T) {
	// Test error after OR conjunction
	_, err := parseQuery("from_email=\"a@b.com\" OR ")
	require.NotNil(t, err)
	require.Equal(t, 400, err.Code)
}

func TestParseQueryExprMultipleAND(t *testing.T) {
	// Test multiple AND conjunctions
	q, err := parseQuery("from_email=\"a@b.com\" AND to_email=\"c@d.com\" AND subject=\"test\"")
	require.Nil(t, err)
	require.NotNil(t, q)

	// Verify it's a nested AND expression
	expr := q.(*QExpr)
	require.Equal(t, QueryType(QueryAnd), expr.getType())
}

func TestParseQueryExprMultipleOR(t *testing.T) {
	// Test multiple OR conjunctions
	q, err := parseQuery("from_email=\"a@b.com\" OR to_email=\"c@d.com\" OR subject=\"test\"")
	require.Nil(t, err)
	require.NotNil(t, q)

	// Verify it's a nested OR expression
	expr := q.(*QExpr)
	require.Equal(t, QueryType(QueryOr), expr.getType())
}

func TestParseValueWithEscapedContent(t *testing.T) {
	// Test escaped backslash in value
	val, rest, err := parseValue("\"hello\\nworld\" rest")
	require.Nil(t, err)
	require.Equal(t, "hellonworld", val)
	require.Equal(t, " rest", rest)
}

func TestParseQuerySimpleError(t *testing.T) {
	// Test parseQuerySimple with invalid operator
	_, _, err := parseQuerySimple("from_email~\"test\"")
	require.NotNil(t, err)

	// Test parseQuerySimple with invalid value
	_, _, err = parseQuerySimple("from_email=noquotes")
	require.NotNil(t, err)
}

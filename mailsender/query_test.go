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

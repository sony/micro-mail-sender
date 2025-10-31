# Status Query

Query message status.

## Entry Point

`GET /v3/messages?query=<query-expr>`

## Request Parameters

A query parameter `query` is required.
Its value is a *URL-encoded* string of a query expression that must follow this EBNF:

```
<query-expr> : <simple-query> [ <conj> <query-expr> ]*

<simple-query> : <name> <op> <value>

<name> : "from_email"
       | "to_email"
       | "subject"
       | "status"
       | "msg_id"

<op>   : '=' | '!='

<value> : A double-quoted string. Double-quotes and backslashes in the value must be escaped with a backslash.

<conj> : 'AND' | 'OR'

```

Whitespace is allowed around terminals.

## Response

Upon successful operation, a 200 response with the following body is returned.


```
{
    "messages" : [ <SearchResultItem> ...]
}
```

`SearchResultItem` is the following object:

```
{
    "from_email"        : <string>
    "msg_id"            : <string>
    "subject"           : <string>
    "to_email"          : <string>
    "status"            : <string>
    "last_timestamp"    : <int>
}
```

- `from_email` is the sender's email address.
- `msg_id` is the message ID assigned by MailSender.
- `subject` is the subject of the email.
- `to_email` is the recipient's email address.
- `status` is one of `waiting`, `processing`, `sent`, or `abandoned`.
- `last_timestamp` is the last timestamp in Unix time.

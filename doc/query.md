# Status query

Query message status.

## Entry point

`GET /v3/messages?query=<query-expr>`

## Request Parameter

A query parameter `query` is required.  Its value is a *URLencoded*
string of a query expression, which must follow this EBNF:

```
<query-expr> : <simple-query> [ <conj> <query-expr> ]*

<simple-query> : <name> <op> <value>

<name> : "from_email"
       | "to_email"
       | "subject"
       | "status"
       | "msg_id"

<op>   : '=' | '!='

<value> : A double-quoted string.  Double-quote and backslash in
          the value must be escaped by a backslash.

<conj> : 'AND' | 'OR'

```

Whitespaces are allowed around terminals.

## Response

Upon successful operation, a 200 response with the following body
is returned.


```
{
    "messages" : [ <SearchResultItem> ...]
}
```

`SearchResultItem` is the following object:

```
{
    "msg-id"      : <string>
    "status"      : <string>
    "last-update" : <string>
    "request"     : <SendRequest>
}
```

- `msg-id` is the message ID assigned by MailSender.
- `status` is either one of `waiting`, `processing`, `sent`, or `abandoned`.
- `last-update` is RFC3339 format of the timestamp when the status
of this message is last updated.
- `request` is the `SendRequest` passed to `/v3/mail/send` API,
expanded for personalizations.  Note that if the message is already sent
or abandoned, this field is not included since the request packet is discarded.

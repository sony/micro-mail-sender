# REST API

The two main calls are the one to send mails and the one to query their status.
We try to make them a subset of SendGrid API.
We also provide a diagnostic API.

- [POST /v3/mail/send - Mail send](send.md)

- [GET /v3/messages - Status query](query.md)

- [GET /v3/smtplog - View MTA logs](log.md)

All API calls require an `Authorization` header with a value `Bearer [KEY]`
where `[KEY]` is to be replaced with one of the keys specified in the server configuration.

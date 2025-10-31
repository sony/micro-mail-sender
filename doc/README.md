# REST API

The two main API calls are for sending emails and querying their status.
We aim to make them compatible with the SendGrid API.
We also provide a diagnostic API for troubleshooting.

- [POST /v3/mail/send - Mail send](send.md)

- [GET /v3/messages - Status query](query.md)

- [GET /v3/smtplog - View MTA logs](log.md)

All API calls require an `Authorization` header with a value `Bearer [KEY]`
where `[KEY]` should be replaced with one of the keys specified in the server configuration.

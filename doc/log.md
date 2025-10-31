# MTA Log Retrieval

View the latest MTA log for troubleshooting.

## Entry Point

`GET /v3/smtplog?count=<count>`

## Request Parameters

The required `count` parameter specifies how many log entries you want to retrieve.

## Response

Upon successful operation, a 200 response with the following body is returned:

```
{
    "count" : <count>
    "lines" : [<string> ...]
}

```

- `count` shows the actual number of log entries retrieved.
It may be smaller than the number specified in the query.
- `lines` contains an array of log entries.

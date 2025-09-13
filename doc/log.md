# View MTA Logs

View the latest MTA log for troubleshooting.

## Entry point

`GET /v3/smtplog?count=<count>`

## Request parameters

The required `count` parameter specifies how many log entries
you want to get.

## Response

Upon successful operation, a 200 response with the following body
is returned:

```
{
    "count" : <count>
    "lines" : [<string> ...]
}

```

- `count` shows the actual number of log entires retrieved.  It might
be smaller than the one specified in the query.
- `lines` has an array of log entries.

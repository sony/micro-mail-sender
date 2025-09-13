# Mail Send

Send messages.

## Entry point

`POST /v3/mail/send`

## Request Body

The main request body, `SendRequest`, is a JSON object with
the following format:

### SendRequest

```
{
    "personalizations": [ <Personalization> ...],    // required at least one
    "from" : <Address>,                              // required
    "reply_to" : <Address>,                          // optional
    "subject" : <string>,                            // required
    "content" : [ <Content> ...]                     // required at least one
}
```

- `personalizations` is an array of `Personalization` object, each
of which represents a recipient.
- `from`, `reply_to` and `subject` are used for the corresponding field.
- `content` is the mail content.  SendGrid allows alternative multipart
content, but MailSender currently supports only one content,
and only the first `Content` is used.

### Personalization

This is called personalization
because SendGrid allows pre-defined template message to be customized according
to this info, but in our case, it is just recipient info.

You can have multiple recipients in one `Personalization`,
but MailSender treats one `Personalization` as an effective recipients.
Suppose you send one `SendRequest` with three `Personalization`.
Then MailSender creates three email messages, each one having unique
message-id, and pass them to MTA.  It doesn't matter how many actual
emails are included in each `Personalization`.


```
{
    "to"      : [ <Address> ...],         // required
    "cc"      : [ <Address> ...],         // optional
    "bcc"     : [ <Address> ...],         // optional
    "subject" : <string>,                 // optional (to override main subject)
    "headers" : <json-objectt>,           // optional
}
```

### Address

`Address` is a generic object paring an email address and
an accompanied name.

```
{
    "email" : <string>             // required, must be a valid email address
    "name"  : <string>             // optional
}
```

### Content

`Content` is the actual message content.

```
{
    "type"  : <string>             // MIME type
    "value" : <string>             // mail content
}
```

## Response

Upon successful operation, a 200 response with the following 
body is returned:


```
{
    "result" : "ok"
}
```

Note that it merely means the request is now on the hand of MTA, and
does not mean the email is successfully delivered. 

If the input contains invalid parameters, a 400 response with
the following body is returned.

```
{
    "code"    : <http-response-code>
    "message" : <string>   // error message
}
```

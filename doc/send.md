# Mail Delivery

Send messages.

## Entry point

`POST /v3/mail/send`

## Request Body

The main request body, `SendRequest`, is a JSON object with the following format:

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

- `personalizations` is an array of `Personalization` objects, each of which represents a recipient.
- `from`, `reply_to`, and `subject` are used for the corresponding fields.
- `content` is the email content.
SendGrid allows alternative multipart content,
but MicroMailSender currently supports only one content type, and only the first `Content` is used.

### Personalization

This is called personalization
because SendGrid allows pre-defined template messages to be customized according to this information,
but in our case, it is just recipient information.

You can have multiple recipients in one `Personalization`,
but MicroMailSender treats one `Personalization` as one effective recipient.
Suppose you send one `SendRequest` with three `Personalization` objects.
Then MicroMailSender creates three email messages, each with a unique message-id, and passes them to the MTA.
It does not matter how many actual email addresses are included in each `Personalization`.

```
{
    "to"      : [ <Address> ...],         // required
    "cc"      : [ <Address> ...],         // optional
    "bcc"     : [ <Address> ...],         // optional
    "subject" : <string>,                 // optional (to override main subject)
    "headers" : <json-object>,            // optional
}
```

### Address

`Address` is a generic object that pairs an email address with an optional name.

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

Upon successful operation, a 202 response is returned without a body.

Note that this merely means the request is now in the hands of the MTA,
and does not mean the email was successfully delivered. 

If the input contains invalid parameters,
a 400 response with the following body is returned.

```
{
  "errors": [
    {
      "message": "content too large",
      "field": "field"
    }
  ]
}
```

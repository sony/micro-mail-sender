# MicroMailSender - Simplified SendGrid Clone

## Overview

MicroMailSender is a lightweight, self-hosted email sending service that provides a SendGrid-compatible API.
It is designed as a simplified alternative to SendGrid for organizations that need email sending capabilities
with full control over their infrastructure.

## API Documentation

See [here](./doc/README.md) for details.


## Example

```
docker compose up
```

```
# send test email
curl -XPOST http://localhost:8333/v3/mail/send -d "@sample/body.json" -H "Authorization: Bearer apikey" 
curl -XPOST http://localhost:8333/v3/mail/send -d "@sample/multiple-personalizations-body.json" -H "Authorization: Bearer apikey" 
curl -XPOST http://localhost:8333/v3/mail/send -d "@sample/multiple-destination-body.json" -H "Authorization: Bearer apikey" 

# check test email on MailHog web console
open http://localhost:8025/

# query mails
curl http://localhost:8333/v3/messages?query=from_email%3D%22from@example.com%22 -H "Authorization: Bearer apikey"

# read log
curl http://localhost:8333/v3/smtplog?count=1 -H "Authorization: Bearer apikey"

```

## License

The MIT License (MIT)

See [LICENSE](./LICENSE) for details.

# SMTP

`integrations/smtp` wraps the standard library's `net/smtp.SendMail` (frozen
upstream - no new stdlib features coming) as a free function.

```go
import argossmtp "github.com/jhonsferg/argos/integrations/smtp"

err := argossmtp.SendMail(ctx, "smtp.example.com:587", auth, from, []string{to}, msg)
```

## Options

- `WithRecipients(bool)` - records the `to` addresses on the span. Off by
  default - email addresses are personally identifiable information.
- `WithLogger(logging.Logger)`.

## See it running

[`notifications-worker`](https://github.com/jhonsferg/argos/tree/main/samples/notifications-worker)
sends an email per RabbitMQ message via Mailpit.

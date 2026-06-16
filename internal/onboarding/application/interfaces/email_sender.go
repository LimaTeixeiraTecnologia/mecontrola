package interfaces

import "context"

type EmailMessage struct {
	To          string
	Subject     string
	HTMLBody    string
	TextBody    string
	FromAddress string
	FromName    string
	ReplyTo     string
}

type EmailSender interface {
	Send(ctx context.Context, msg EmailMessage) error
}

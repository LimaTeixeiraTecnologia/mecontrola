package notification_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/notification"
)

type MultiChannelGatewaySuite struct {
	suite.Suite
}

func TestMultiChannelGateway(t *testing.T) {
	suite.Run(t, new(MultiChannelGatewaySuite))
}

func (s *MultiChannelGatewaySuite) TestSendText() {
	errBoom := errors.New("boom")

	scenarios := []struct {
		name       string
		channel    string
		externalID string
		text       string
		senders    map[string]notification.ChannelSenders
		expectErr  error
		dispatched string
	}{
		{
			name:       "deve despachar whatsapp",
			channel:    notification.ChannelWhatsApp,
			externalID: "+5511999990000",
			text:       "ola",
			senders: map[string]notification.ChannelSenders{
				notification.ChannelWhatsApp: {Text: func(_ context.Context, _, _ string) error { return nil }},
			},
			dispatched: notification.ChannelWhatsApp,
		},
		{
			name:       "deve falhar com canal desconhecido",
			channel:    "sms",
			externalID: "+5511",
			text:       "x",
			senders:    map[string]notification.ChannelSenders{},
			expectErr:  notification.ErrUnknownChannel,
		},
		{
			name:       "deve falhar com external_id vazio",
			channel:    notification.ChannelWhatsApp,
			externalID: "",
			text:       "x",
			senders: map[string]notification.ChannelSenders{
				notification.ChannelWhatsApp: {Text: func(_ context.Context, _, _ string) error { return nil }},
			},
			expectErr: notification.ErrEmptyExternal,
		},
		{
			name:       "deve falhar com texto vazio",
			channel:    notification.ChannelWhatsApp,
			externalID: "+5511",
			text:       "",
			senders: map[string]notification.ChannelSenders{
				notification.ChannelWhatsApp: {Text: func(_ context.Context, _, _ string) error { return nil }},
			},
			expectErr: notification.ErrEmptyText,
		},
		{
			name:       "deve propagar erro do sender",
			channel:    notification.ChannelWhatsApp,
			externalID: "+5511999990001",
			text:       "x",
			senders: map[string]notification.ChannelSenders{
				notification.ChannelWhatsApp: {Text: func(_ context.Context, _, _ string) error { return errBoom }},
			},
			expectErr: errBoom,
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			var dispatched string
			senders := make(map[string]notification.ChannelSenders, len(scenario.senders))
			for ch, set := range scenario.senders {
				channel := ch
				inner := set.Text
				senders[channel] = notification.ChannelSenders{
					Text: func(ctx context.Context, externalID, text string) error {
						dispatched = channel
						return inner(ctx, externalID, text)
					},
				}
			}
			gw := notification.NewMultiChannelGateway(senders)
			err := gw.SendText(context.Background(), scenario.channel, scenario.externalID, scenario.text)
			if scenario.expectErr != nil {
				s.Require().Error(err)
				s.ErrorIs(err, scenario.expectErr)
				return
			}
			s.Require().NoError(err)
			s.Equal(scenario.dispatched, dispatched)
		})
	}
}

func (s *MultiChannelGatewaySuite) TestSendActivationTemplate() {
	scenarios := []struct {
		name         string
		channel      string
		externalID   string
		templateName string
		token        string
		senders      map[string]notification.ChannelSenders
		expectErr    error
		expectMsgID  string
	}{
		{
			name:         "deve enviar template via whatsapp",
			channel:      notification.ChannelWhatsApp,
			externalID:   "+5511999990001",
			templateName: "activation",
			token:        "tok",
			senders: map[string]notification.ChannelSenders{
				notification.ChannelWhatsApp: {Template: func(_ context.Context, _, _, _ string) (string, error) {
					return "wamid.123", nil
				}},
			},
			expectMsgID: "wamid.123",
		},
		{
			name:         "deve falhar quando canal nao suporta template",
			channel:      notification.ChannelWhatsApp,
			externalID:   "+5511999990002",
			templateName: "activation",
			token:        "tok",
			senders: map[string]notification.ChannelSenders{
				notification.ChannelWhatsApp: {Text: func(_ context.Context, _, _ string) error { return nil }},
			},
			expectErr: notification.ErrTemplateUnsupported,
		},
		{
			name:         "deve falhar com canal desconhecido",
			channel:      "sms",
			externalID:   "x",
			templateName: "x",
			token:        "x",
			senders:      map[string]notification.ChannelSenders{},
			expectErr:    notification.ErrUnknownChannel,
		},
		{
			name:         "deve falhar com template vazio",
			channel:      notification.ChannelWhatsApp,
			externalID:   "+5511",
			templateName: "",
			token:        "x",
			senders: map[string]notification.ChannelSenders{
				notification.ChannelWhatsApp: {Template: func(_ context.Context, _, _, _ string) (string, error) {
					return "x", nil
				}},
			},
			expectErr: notification.ErrEmptyTemplateName,
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			gw := notification.NewMultiChannelGateway(scenario.senders)
			id, err := gw.SendActivationTemplate(context.Background(), scenario.channel, scenario.externalID, scenario.templateName, scenario.token)
			if scenario.expectErr != nil {
				s.Require().Error(err)
				s.ErrorIs(err, scenario.expectErr)
				return
			}
			s.Require().NoError(err)
			s.Equal(scenario.expectMsgID, id)
		})
	}
}

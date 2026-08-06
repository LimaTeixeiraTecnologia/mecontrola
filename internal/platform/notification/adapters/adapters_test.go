package adapters_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/notification"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/notification/adapters"
)

type fakeWhatsApp struct {
	textCalled bool
	tmplCalled bool
	to         string
	text       string
	tmpl       string
	tok        string
	err        error
}

func (f *fakeWhatsApp) SendTextMessage(_ context.Context, to, text string) error {
	f.textCalled = true
	f.to = to
	f.text = text
	return f.err
}

func (f *fakeWhatsApp) SendTemplateMessage(_ context.Context, to string, message notification.TemplateMessage) (string, error) {
	f.tmplCalled = true
	f.to = to
	f.tmpl = message.TemplateName
	if len(message.Components) > 0 && len(message.Components[0].Parameters) > 0 {
		f.tok = message.Components[0].Parameters[0].Text
	}
	if f.err != nil {
		return "", f.err
	}
	return "wamid.test", nil
}

type AdaptersSuite struct {
	suite.Suite
}

func TestAdapters(t *testing.T) {
	suite.Run(t, new(AdaptersSuite))
}

func (s *AdaptersSuite) TestWhatsAppSenderSendText() {
	fake := &fakeWhatsApp{}
	sender := adapters.NewWhatsAppSender(fake)
	err := sender.SendText(context.Background(), "+5511999990000", "ola")
	s.Require().NoError(err)
	s.True(fake.textCalled)
	s.Equal("+5511999990000", fake.to)
	s.Equal("ola", fake.text)
}

func (s *AdaptersSuite) TestWhatsAppSenderSendTemplate() {
	fake := &fakeWhatsApp{}
	sender := adapters.NewWhatsAppSender(fake)
	id, err := sender.SendTemplate(context.Background(), notification.TemplateMessage{
		Channel:      notification.ChannelWhatsApp,
		ExternalID:   "+5511999990000",
		TemplateName: "activation_reminder",
		LanguageCode: "pt_BR",
		Components: []notification.TemplateComponent{
			{
				Type: notification.TemplateComponentBody,
				Parameters: []notification.TemplateParameter{
					{Type: notification.TemplateParameterText, Text: "tok-x"},
				},
			},
		},
	})
	s.Require().NoError(err)
	s.Equal("wamid.test", id)
	s.True(fake.tmplCalled)
	s.Equal("activation_reminder", fake.tmpl)
	s.Equal("tok-x", fake.tok)
}

func (s *AdaptersSuite) TestWhatsAppSenderPropagatesError() {
	fake := &fakeWhatsApp{err: errors.New("meta down")}
	sender := adapters.NewWhatsAppSender(fake)
	err := sender.SendText(context.Background(), "+5511", "x")
	s.Require().Error(err)
	_, err = sender.SendTemplate(context.Background(), notification.TemplateMessage{
		Channel:      notification.ChannelWhatsApp,
		ExternalID:   "+5511",
		TemplateName: "t",
		LanguageCode: "pt_BR",
		Components: []notification.TemplateComponent{
			{
				Type: notification.TemplateComponentBody,
				Parameters: []notification.TemplateParameter{
					{Type: notification.TemplateParameterText, Text: "tok"},
				},
			},
		},
	})
	s.Require().Error(err)
}

func (s *AdaptersSuite) TestWhatsAppAsChannelSendersExposesBoth() {
	fake := &fakeWhatsApp{}
	sender := adapters.NewWhatsAppSender(fake)
	cs := sender.AsChannelSenders()
	s.Require().NotNil(cs.Text)
	s.Require().NotNil(cs.Template)
}

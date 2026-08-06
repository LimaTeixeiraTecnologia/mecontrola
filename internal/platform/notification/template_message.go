package notification

import "fmt"

type TemplateComponentType string

const (
	TemplateComponentBody   TemplateComponentType = "body"
	TemplateComponentHeader TemplateComponentType = "header"
	TemplateComponentButton TemplateComponentType = "button"
)

type TemplateParameterType string

const TemplateParameterText TemplateParameterType = "text"

type TemplateButtonSubType string

const TemplateButtonSubTypeQuickReply TemplateButtonSubType = "quick_reply"

type TemplateMessage struct {
	Channel      string
	ExternalID   string
	TemplateName string
	LanguageCode string
	Components   []TemplateComponent
}

type TemplateComponent struct {
	Type       TemplateComponentType
	SubType    TemplateButtonSubType
	Index      string
	Parameters []TemplateParameter
}

type TemplateParameter struct {
	Type TemplateParameterType
	Text string
}

func (m TemplateMessage) Validate() error {
	if m.Channel == "" {
		return ErrUnknownChannel
	}
	if m.ExternalID == "" {
		return ErrEmptyExternal
	}
	if m.TemplateName == "" {
		return ErrEmptyTemplateName
	}
	for _, component := range m.Components {
		if err := component.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (c TemplateComponent) Validate() error {
	switch c.Type {
	case TemplateComponentBody, TemplateComponentHeader:
	case TemplateComponentButton:
		if c.Index == "" {
			return fmt.Errorf("%w: button.index", ErrEmptyTemplateComponentField)
		}
	default:
		return ErrEmptyTemplateComponentType
	}
	for _, parameter := range c.Parameters {
		if err := parameter.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (p TemplateParameter) Validate() error {
	if p.Type == "" {
		return ErrEmptyTemplateParameterType
	}
	if p.Type == TemplateParameterText && p.Text == "" {
		return ErrEmptyTemplateParameterText
	}
	return nil
}

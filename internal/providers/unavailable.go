package providers

import (
	"context"
	"errors"
	"fmt"
)

var ErrModelConfiguration = errors.New("model configuration")

type unavailableProvider struct {
	model string
	err   error
}

func NewUnavailableProvider(model, reason string) LLMProvider {
	if model == "" {
		model = "gpt-4"
	}
	return &unavailableProvider{model: model, err: fmt.Errorf("%w: %s", ErrModelConfiguration, reason)}
}

func (p *unavailableProvider) Chat(context.Context, []Message, []map[string]interface{}, string) (*Response, error) {
	return nil, p.err
}

func (p *unavailableProvider) ChatStream(_ context.Context, _ []Message, _ []map[string]interface{}, _ string, handler StreamHandler) error {
	if handler != nil {
		handler.OnError(p.err)
	}
	return p.err
}

func (p *unavailableProvider) GetDefaultModel() string        { return p.model }
func (p *unavailableProvider) SupportsImageInput(string) bool { return false }

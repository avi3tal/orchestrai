package state

import (
	"errors"
	"github.com/tmc/langchaingo/llms"
)

type MessagesState struct {
	Messages []llms.MessageContent
}

func (m MessagesState) Validate() error {
	// TODO add proper llms.MessageContent sequence validation
	if len(m.Messages) == 0 {
		return errors.New("no messages")
	}
	return nil
}

func (m MessagesState) Merge(other MessagesState) MessagesState {
	return MessagesState{
		Messages: append(m.Messages, other.Messages...),
	}
}

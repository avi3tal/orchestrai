package state

import (
	"encoding/json"
	"errors"

	"github.com/tmc/langchaingo/llms"
)

var ErrNoMessages = errors.New("no messages")

type MessagesState struct {
	Messages []llms.MessageContent
}

func (m MessagesState) Validate() error {
	// TODO add proper llms.MessageContent sequence validation
	if len(m.Messages) == 0 {
		return ErrNoMessages
	}
	return nil
}

func (m MessagesState) Merge(other GraphState) GraphState {
	o, ok := other.(MessagesState)
	if !ok {
		return m
	}
	return MessagesState{
		Messages: append(m.Messages, o.Messages...),
	}
}

func (m MessagesState) Clone() GraphState {
	return MessagesState{
		Messages: append([]llms.MessageContent{}, m.Messages...),
	}
}

func (m MessagesState) Dump() ([]byte, error) {
	return json.Marshal(m)
}

func (m MessagesState) Load(data []byte) (GraphState, error) {
	var st MessagesState
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, err
	}
	return st, nil
}

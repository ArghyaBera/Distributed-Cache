package protocol

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Command string

const (
	CMDGet Command = "GET"
	CMDSet Command = "SET"
)

type Message struct {
	Cmd   Command
	Key   []byte
	Value []byte
	TTL   time.Duration
}

func (m *Message) ToBytes() []byte {
	if m.Cmd == CMDSet {
		return []byte(fmt.Sprintf("SET %s %s %d", m.Key, m.Value, m.TTL))
	}
	return []byte(fmt.Sprintf("GET %s", m.Key))
}

func ParseCommand(raw []byte) (*Message, error) {
	parts := strings.Fields(string(raw))
	if len(parts) < 2 {
		return nil, errors.New("invalid command")
	}

	msg := &Message{
		Cmd: Command(parts[0]),
		Key: []byte(parts[1]),
	}

	if msg.Cmd == CMDSet {
		if len(parts) != 4 {
			return nil, errors.New("invalid SET command format")
		}
		msg.Value = []byte(parts[2])
		ttl, err := strconv.ParseInt(parts[3], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid TTL: %w", err)
		}
		msg.TTL = time.Duration(ttl)
	}

	return msg, nil
}

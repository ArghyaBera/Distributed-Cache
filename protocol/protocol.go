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
	CMDGet     Command = "GET"
	CMDSet     Command = "SET"
	CMDDel     Command = "DEL"
	CMDHas     Command = "HAS"
	CMDKeys    Command = "KEYS"
	CMDMetrics Command = "METRICS"
	CMDBatch   Command = "BATCH"
)

type Message struct {
	Cmd   Command
	Key   []byte
	Value []byte
	TTL   time.Duration
	Pairs map[string][]byte // For batch operations
}

func (m *Message) ToBytes() []byte {
	switch m.Cmd {
	case CMDSet:
		return []byte(fmt.Sprintf("SET %s %s %d", m.Key, m.Value, m.TTL))
	case CMDGet, CMDHas, CMDDel:
		return []byte(fmt.Sprintf("%s %s", m.Cmd, m.Key))
	case CMDKeys:
		return []byte("KEYS")
	case CMDMetrics:
		return []byte("METRICS")
	case CMDBatch:
		pairs := make([]string, 0, len(m.Pairs))
		for k, v := range m.Pairs {
			pairs = append(pairs, fmt.Sprintf("%s:%s", k, v))
		}
		return []byte(fmt.Sprintf("BATCH %s %d", strings.Join(pairs, ","), m.TTL))
	}
	return nil
}

func ParseCommand(raw []byte) (*Message, error) {
	parts := strings.Fields(string(raw))
	if len(parts) < 1 {
		return nil, errors.New("invalid command")
	}

	msg := &Message{
		Cmd: Command(parts[0]),
	}

	switch msg.Cmd {
	case CMDSet:
		if len(parts) != 4 {
			return nil, errors.New("invalid SET command format")
		}
		msg.Key = []byte(parts[1])
		msg.Value = []byte(parts[2])
		ttl, err := strconv.ParseInt(parts[3], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid TTL: %w", err)
		}
		msg.TTL = time.Duration(ttl)

	case CMDGet, CMDHas, CMDDel:
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid %s command format", msg.Cmd)
		}
		msg.Key = []byte(parts[1])

	case CMDKeys, CMDMetrics:
		if len(parts) != 1 {
			return nil, fmt.Errorf("invalid %s command format", msg.Cmd)
		}

	case CMDBatch:
		if len(parts) < 3 {
			return nil, errors.New("invalid BATCH command format")
		}
		pairs := strings.Split(parts[1], ",")
		msg.Pairs = make(map[string][]byte)
		for _, pair := range pairs {
			kv := strings.SplitN(pair, ":", 2)
			if len(kv) != 2 {
				return nil, errors.New("invalid key-value pair in BATCH")
			}
			msg.Pairs[kv[0]] = []byte(kv[1])
		}
		ttl, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid TTL: %w", err)
		}
		msg.TTL = time.Duration(ttl)
	}

	return msg, nil
}

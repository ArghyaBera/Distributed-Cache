package server

import (
	"context"
	"distributedCache/cache"
	"distributedCache/protocol"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"
)

type Options struct {
	ListenAddr  string
	IsLeader    bool
	LeaderAddr  string
	StoragePath string
}

type Server struct {
	opts       Options
	cache      cache.Cacher
	followers  map[net.Conn]struct{}
	mu         sync.Mutex
	connPool   chan net.Conn
	maxRetries int
	retryDelay time.Duration
}

func New(opts Options, cacher cache.Cacher) *Server {
	return &Server{
		opts:       opts,
		cache:      cacher,
		followers:  make(map[net.Conn]struct{}),
		connPool:   make(chan net.Conn, 10), // Connection pool for followers
		maxRetries: 3,
		retryDelay: time.Second,
	}
}

func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.opts.ListenAddr)
	if err != nil {
		return fmt.Errorf("listen error: %w", err)
	}
	log.Printf("Server started on %s [Leader: %v]", s.opts.ListenAddr, s.opts.IsLeader)

	if !s.opts.IsLeader {
		go s.connectToLeader()
	}

	// Periodic persistence if storage path is specified
	if s.opts.StoragePath != "" {
		go s.periodicSave()
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("Accept error: %v", err)
			continue
		}
		s.connPool <- conn
		go s.handleConnection(conn)
	}
}

func (s *Server) periodicSave() {
	if pc, ok := s.cache.(*cache.PersistentCache); ok {
		ticker := time.NewTicker(5 * time.Minute)
		for range ticker.C {
			if err := pc.SaveToDisk(); err != nil {
				log.Printf("Failed to save cache to disk: %v", err)
			}
		}
	}
}

func (s *Server) connectToLeader() {
	for attempt := 1; attempt <= s.maxRetries; attempt++ {
		conn, err := net.Dial("tcp", s.opts.LeaderAddr)
		if err == nil {
			log.Printf("Connected to leader at %s", s.opts.LeaderAddr)
			s.connPool <- conn
			s.handleConnection(conn)
			return
		}
		log.Printf("Failed to connect to leader (attempt %d/%d): %v", attempt, s.maxRetries, err)
		time.Sleep(s.retryDelay)
	}
	log.Fatalf("Failed to connect to leader after %d attempts", s.maxRetries)
}

func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()
	log.Printf("New connection from %s", conn.RemoteAddr())

	if s.opts.IsLeader {
		s.mu.Lock()
		s.followers[conn] = struct{}{}
		s.mu.Unlock()
	}

	buf := make([]byte, 2048)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			log.Printf("Connection read error from %s: %v", conn.RemoteAddr(), err)
			s.mu.Lock()
			delete(s.followers, conn)
			s.mu.Unlock()
			return
		}
		go s.handleCommand(conn, buf[:n])
	}
}

func (s *Server) handleCommand(conn net.Conn, raw []byte) {
	msg, err := protocol.ParseCommand(raw)
	if err != nil {
		conn.Write([]byte("ERROR: " + err.Error()))
		return
	}

	log.Printf("Command received: %s", msg.Cmd)
	switch msg.Cmd {
	case protocol.CMDSet:
		err = s.handleSet(conn, msg)
	case protocol.CMDGet:
		err = s.handleGet(conn, msg)
	case protocol.CMDDel:
		err = s.handleDelete(conn, msg)
	case protocol.CMDHas:
		err = s.handleHas(conn, msg)
	case protocol.CMDKeys:
		err = s.handleKeys(conn, msg)
	case protocol.CMDMetrics:
		err = s.handleMetrics(conn, msg)
	case protocol.CMDBatch:
		err = s.handleBatch(conn, msg)
	}

	if err != nil {
		conn.Write([]byte("ERROR: " + err.Error()))
	}
}

func (s *Server) handleGet(conn net.Conn, msg *protocol.Message) error {
	val, err := s.cache.Get(msg.Key)
	if err != nil {
		return err
	}
	_, err = conn.Write(val)
	return err
}

func (s *Server) handleSet(conn net.Conn, msg *protocol.Message) error {
	if err := s.cache.Set(msg.Key, msg.Value, msg.TTL); err != nil {
		return err
	}
	if s.opts.IsLeader {
		go s.replicateToFollowers(context.Background(), msg)
	}
	_, err := conn.Write([]byte("OK"))
	return err
}

func (s *Server) handleDelete(conn net.Conn, msg *protocol.Message) error {
	if err := s.cache.Delete(msg.Key); err != nil {
		return err
	}
	if s.opts.IsLeader {
		go s.replicateToFollowers(context.Background(), msg)
	}
	_, err := conn.Write([]byte("OK"))
	return err
}

func (s *Server) handleHas(conn net.Conn, msg *protocol.Message) error {
	has := s.cache.Has(msg.Key)
	_, err := conn.Write([]byte(fmt.Sprintf("%v", has)))
	return err
}

func (s *Server) handleKeys(conn net.Conn, msg *protocol.Message) error {
	keys := s.cache.Keys()
	keyStrings := make([]string, len(keys))
	for i, k := range keys {
		keyStrings[i] = string(k)
	}
	_, err := conn.Write([]byte(strings.Join(keyStrings, ",")))
	return err
}

func (s *Server) handleMetrics(conn net.Conn, msg *protocol.Message) error {
	metrics := s.cache.Metrics()
	data, err := json.Marshal(metrics)
	if err != nil {
		return err
	}
	_, err = conn.Write(data)
	return err
}

func (s *Server) handleBatch(conn net.Conn, msg *protocol.Message) error {
	if err := s.cache.BatchSet(msg.Pairs, msg.TTL); err != nil {
		return err
	}
	if s.opts.IsLeader {
		go s.replicateToFollowers(context.Background(), msg)
	}
	_, err := conn.Write([]byte("OK"))
	return err
}

func (s *Server) replicateToFollowers(ctx context.Context, msg *protocol.Message) {
	raw := msg.ToBytes()
	s.mu.Lock()
	followers := make([]net.Conn, 0, len(s.followers))
	for conn := range s.followers {
		followers = append(followers, conn)
	}
	s.mu.Unlock()

	for _, conn := range followers {
		for attempt := 1; attempt <= s.maxRetries; attempt++ {
			if _, err := conn.Write(raw); err != nil {
				log.Printf("Replication to %s failed (attempt %d/%d): %v",
					conn.RemoteAddr(), attempt, s.maxRetries, err)
				if attempt == s.maxRetries {
					conn.Close()
					s.mu.Lock()
					delete(s.followers, conn)
					s.mu.Unlock()
				}
				time.Sleep(s.retryDelay)
				continue
			}
			break
		}
	}
}

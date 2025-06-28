package server

import (
	"context"
	"distributedCache/cache"
	"distributedCache/protocol"
	"fmt"
	"log"
	"net"
	"sync"
)

type Options struct {
	ListenAddr string
	IsLeader   bool
	LeaderAddr string
}

type Server struct {
	opts      Options
	cache     cache.Cacher
	followers map[net.Conn]struct{}
	mu        sync.Mutex
}

func New(opts Options, cacher cache.Cacher) *Server {
	return &Server{
		opts:      opts,
		cache:     cacher,
		followers: make(map[net.Conn]struct{}),
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

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("Accept error: %v", err)
			continue
		}
		go s.handleConnection(conn)
	}
}

func (s *Server) connectToLeader() {
	conn, err := net.Dial("tcp", s.opts.LeaderAddr)
	if err != nil {
		log.Fatalf("Failed to connect to leader: %v", err)
	}
	log.Printf("Connected to leader at %s", s.opts.LeaderAddr)
	s.handleConnection(conn)
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
	return nil
}

func (s *Server) replicateToFollowers(ctx context.Context, msg *protocol.Message) {
	raw := msg.ToBytes()
	s.mu.Lock()
	defer s.mu.Unlock()
	for conn := range s.followers {
		if _, err := conn.Write(raw); err != nil {
			log.Printf("Replication to %s failed: %v", conn.RemoteAddr(), err)
			conn.Close()
			delete(s.followers, conn)
		}
	}
}

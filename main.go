package main

import (
	"distributedCache/cache"
	"distributedCache/server"
	"flag"
	"log"
)

func main() {
	var (
		listenAddr  = flag.String("listenaddr", ":3000", "Address this server listens on")
		leaderAddr  = flag.String("leaderaddr", "", "Address of the leader (leave blank if this is the leader)")
		storagePath = flag.String("storage", "cache.db", "Path to store persistent cache data")
	)
	flag.Parse()

	isLeader := *leaderAddr == ""
	opts := server.Options{
		ListenAddr:  *listenAddr,
		IsLeader:    isLeader,
		LeaderAddr:  *leaderAddr,
		StoragePath: *storagePath,
	}

	var c cache.Cacher
	var err error
	if *storagePath != "" {
		c, err = cache.NewPersistentCache(*storagePath)
		if err != nil {
			log.Fatalf("Failed to create persistent cache: %v", err)
		}
	} else {
		c = cache.NewCache()
	}

	s := server.New(opts, c)
	if err := s.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

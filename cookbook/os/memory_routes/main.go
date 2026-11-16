// Memory routes (/memories, /memory_topics, /user_memory_stats).
//
// Shows: the Python-compatible memory API backed by an AgentOS Database
// (here the in-memory one). Memories are created, listed with pagination,
// filtered by user/topic/content, updated and deleted.
//
// Needs: nothing external; no model is called.
//
//	go run ./cookbook/os/memory_routes
//	curl -X POST localhost:7777/memories -H 'Content-Type: application/json' -d '{"memory":"Likes espresso","user_id":"alice","topics":["food"]}'
//	curl "localhost:7777/memories?user_id=alice&topics=food&limit=10&page=1"
//	curl localhost:7777/memory_topics
//	curl localhost:7777/user_memory_stats
//	curl -X PATCH localhost:7777/memories/<memory_id> -H 'Content-Type: application/json' -d '{"memory":"Likes ristretto","user_id":"alice"}'
//	curl -X DELETE localhost:7777/memories/<memory_id>
package main

import (
	"log"
	"time"

	agnoos "github.com/devalexandre/agno-golang/agno/os"
)

func main() {
	app, err := agnoos.NewAgentOS(agnoos.AgentOSOptions{
		OSID:     "memory-os",
		DB:       agnoos.NewInMemoryDB("memory-db"),
		Settings: &agnoos.AgentOSSettings{Host: "0.0.0.0", Port: 7777, EnableCORS: true, Timeout: time.Minute},
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Fatal(app.Serve())
}

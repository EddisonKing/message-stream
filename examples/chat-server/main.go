package main

import (
	"log"
	"net"

	ms "github.com/EddisonKing/message-stream"
)

var (
	clients  = make([]*ms.MessageStream, 0)
	messages = make(chan *ms.Message, 100)
)

func main() {
	log.Printf("Starting server...\n")
	listener, err := net.Listen("tcp", "127.0.0.1:4547")
	if err != nil {
		log.Fatalf("failed to start chat server: %v\n", err)
	}

	log.Printf("Server started.\n")

	go func() {
		log.Printf("Waiting for client messages...\n")
		for msg := range messages {
			_, metadata, _ := ms.Unwrap[any](msg)
			username, exists := metadata["username"]
			if !exists {
				username = "unknown"
			}

			log.Printf("Message from %s.\n", username)

			go func() {
				for _, client := range clients {
					client.ForwardMessage(msg)
				}
			}()
		}
	}()

	log.Printf("Waiting for clients...\n")
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("failed to accept connection: %v\n", err)
			continue
		}
		log.Printf("New client from %s.\n", conn.RemoteAddr().String())

		stream := ms.New(conn)
		stream.SetKeys(ms.GenerateRSAKeyPair())
		if err = stream.Connect(); err != nil {
			log.Printf("failed to negatiate message stream with client: %v\n", err)
			continue
		}

		clients = append(clients, stream)

		go func() {
			for msg := range stream.Receiver() {
				messages <- msg
			}
		}()

		go func() {
			for err := range stream.Errors() {
				log.Printf("MsgStream Error: %s\n", err)
			}
		}()
	}
}

package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"

	"github.com/EddisonKing/message-stream"
)

var ChatMessage = messagestream.MessageType("chat")

func main() {
	log.Printf("Connecting to Server...\n")
	conn, err := net.Dial("tcp", "127.0.0.1:4547")
	if err != nil {
		log.Fatalf("Failed to connect to Server: %s\n", err)
	}

	stream, err := messagestream.New(conn)
	if err != nil {
		log.Fatalf("Failed to negotiate message stream with server: %s\n", err)
	}

	buffer := bufio.NewReader(os.Stdin)

	fmt.Printf("Username: ")
	username, err := buffer.ReadString('\n')
	if err != nil {
		log.Fatalf("Failed to read from stdin: %s", err)
	}
	username = strings.TrimSpace(username)
	fmt.Printf("\n")

	go func() {
		for err := range stream.Errors() {
			log.Printf("MsgStream Error: %s\n", err)
		}
	}()

	go func() {
		for msg := range stream.Receiver() {
			metadata := messagestream.ExtractMetadata(msg)
			chat, err := messagestream.ExtractPayload[string](msg)
			if err != nil {
				log.Printf("Failed to extract message payload: %s\n", err)
				continue
			}

			incomingUsername, exists := metadata["username"]
			if !exists {
				incomingUsername = "unknown"
			}

			if incomingUsername == username {
				continue
			}

			fmt.Printf("%s : %s\n", incomingUsername, chat)
		}
	}()

	metadata := map[string]any{
		"username": username,
	}

	fmt.Printf("You can now send messages to the server.")

	for {
		fmt.Printf("\n> ")
		line, err := buffer.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				continue
			}
			log.Fatalf("Failed to read from stdin: %s", err)
		}
		line = strings.TrimSpace(line)

		err = stream.SendMessage(ChatMessage, metadata, line)
		if err != nil {
			log.Printf("Failed to send message: %s\n", err)
			continue
		}
	}
}

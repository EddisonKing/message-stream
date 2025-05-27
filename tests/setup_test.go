package tests

import (
	"crypto/rsa"
	"log/slog"
	"net"
	"os"

	ms "github.com/EddisonKing/message-stream"
)

var (
	server          net.Listener
	encryptedServer net.Listener
)

var logger *slog.Logger

func init() {
	logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))
}

var (
	serverPubKey  *rsa.PublicKey
	serverPrivKey *rsa.PrivateKey
)

func getOptions() *ms.MessageStreamOptions {
	opts := ms.NewMessageStreamOptions()
	opts.UseAsymmetricEncryption = false
	opts.Logger = logger
	opts.DeepLogging = false
	return opts
}

func getOptionsForEncryption() *ms.MessageStreamOptions {
	opts := ms.NewMessageStreamOptions()
	opts.AllowProxying = true
	opts.Logger = logger
	opts.DeepLogging = false
	return opts
}

// Echo Server for test setup
func getConn() (net.Conn, error) {
	if server == nil {
		listener, err := net.Listen("tcp", "127.0.0.2:9190")
		if err != nil {
			return nil, err
		}

		server = listener
		go func() {
			for {
				client, err := server.Accept()
				if err != nil {
					panic(err)
				}

				clientMsgStream, err := ms.New(client, getOptions())
				if err != nil {
					panic(err)
				}

				go func() {
					for msg := range clientMsgStream.Receiver() {
						// Simple echo server, forward message back
						if err := clientMsgStream.ForwardMessage(msg); err != nil {
							panic(err)
						}
					}
				}()
			}
		}()
	}

	conn, err := net.Dial("tcp", "127.0.0.2:9190")
	if err != nil {
		return nil, err
	}

	return conn, err
}

func getEncryptedConn() (net.Conn, error) {
	if encryptedServer == nil {
		serverOpts := getOptionsForEncryption()
		listener, err := net.Listen("tcp", "127.0.0.2:9191")
		if err != nil {
			return nil, err
		}

		encryptedServer = listener
		go func() {
			for {
				client, err := encryptedServer.Accept()
				if err != nil {
					panic(err)
				}

				clientMsgStream, err := ms.New(client, serverOpts)
				if err != nil {
					panic(err)
				}

				go func() {
					for msg := range clientMsgStream.Receiver() {
						// Simple echo server, forward message back
						if err := clientMsgStream.ForwardMessage(msg); err != nil {
							panic(err)
						}
					}
				}()
			}
		}()
	}

	conn, err := net.Dial("tcp", "127.0.0.2:9191")
	if err != nil {
		return nil, err
	}

	return conn, err
}

const (
	TestMessage  = ms.MessageType("test")
	ReplyMessage = ms.MessageType("reply")
)

package tests

import (
	"crypto/rsa"
	"log"
	"log/slog"
	"net"
	"os"
	"testing"
	"time"

	ms "github.com/EddisonKing/message-stream"

	"github.com/stretchr/testify/assert"
)

var server net.Listener

func init() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))
	ms.SetLogger(logger)
}

var (
	serverPubKey  *rsa.PublicKey
	serverPrivKey *rsa.PrivateKey
)

// Echo Server for test setup
func getConn(useEncryption bool) (net.Conn, error) {
	if useEncryption {
		if serverPrivKey == nil || serverPubKey == nil {
			serverPrivKey, serverPubKey = ms.GenerateRSAKeyPair()
		}
	}

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

				clientMsgStream := ms.New(client)
				clientMsgStream.SetKeys(serverPrivKey, serverPubKey)

				if err := clientMsgStream.Connect(); err != nil {
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

const TestMessage = ms.MessageType("test")

func TestFullMessageTransfer(t *testing.T) {
	conn, err := getConn(false)
	assert.Nil(t, err)
	if err != nil {
		return
	}

	header := map[string]any{
		"creation_time": time.Now().UTC(),
	}
	payload := "Hello World!"

	msgStream := ms.New(conn)

	err = msgStream.SendMessage(TestMessage, header, payload)
	assert.Nil(t, err)
	if err != nil {
		return
	}

	anyErrors := false
	go func() {
		for err := range msgStream.Errors() {
			anyErrors = true
			log.Println(err)
		}
	}()

	sentMsg := <-msgStream.Receiver()

	assert.NotNil(t, sentMsg)
	if sentMsg == nil {
		return
	}
	assert.Equal(t, TestMessage, sentMsg.Type)

	sentPayload, sentMetadata, err := ms.Unwrap[string](sentMsg)
	assert.Nil(t, err)
	if err != nil {
		return
	}

	assert.Equal(t, payload, sentPayload)

	assert.NotNil(t, sentMetadata)
	if sentMetadata != nil {
		_, exists := sentMetadata["creation_time"]
		assert.True(t, exists)
	}

	assert.False(t, anyErrors)
}

func TestMetadataOnlyMessageTransfer(t *testing.T) {
	conn, err := getConn(false)
	assert.Nil(t, err)
	if err != nil {
		return
	}

	header := map[string]any{
		"creation_time": time.Now().UTC(),
	}

	msgStream := ms.New(conn)

	err = msgStream.SendMessage(TestMessage, header, nil)
	assert.Nil(t, err)
	if err != nil {
		return
	}

	anyErrors := false
	go func() {
		for err := range msgStream.Errors() {
			anyErrors = true
			log.Println(err)
		}
	}()

	sentMsg := <-msgStream.Receiver()

	assert.NotNil(t, sentMsg)
	if sentMsg == nil {
		return
	}
	assert.Equal(t, TestMessage, sentMsg.Type)

	_, sentMetadata, err := ms.Unwrap[any](sentMsg)
	assert.Nil(t, err)

	assert.NotNil(t, sentMetadata)
	if sentMetadata != nil {
		_, exists := sentMetadata["creation_time"]
		assert.True(t, exists)
	}

	assert.False(t, anyErrors)
}

func TestPayloadOnlyMessageTransfer(t *testing.T) {
	conn, err := getConn(false)
	assert.Nil(t, err)
	if err != nil {
		return
	}

	payload := "Hello World!"

	msgStream := ms.New(conn)

	err = msgStream.SendMessage(TestMessage, nil, payload)
	assert.Nil(t, err)
	if err != nil {
		return
	}

	anyErrors := false
	go func() {
		for err := range msgStream.Errors() {
			anyErrors = true
			log.Println(err)
		}
	}()

	sentMsg := <-msgStream.Receiver()

	assert.NotNil(t, sentMsg)
	if sentMsg == nil {
		return
	}
	assert.Equal(t, TestMessage, sentMsg.Type)

	sentPayload, _, err := ms.Unwrap[string](sentMsg)
	assert.Nil(t, err)
	if err != nil {
		return
	}

	assert.Equal(t, payload, sentPayload)
	assert.False(t, anyErrors)
}

func TestEncryptedFullMessageTransfer(t *testing.T) {
	conn, err := getConn(true)
	assert.Nil(t, err)
	if err != nil {
		return
	}

	header := map[string]any{
		"creation_time": time.Now().UTC(),
	}
	payload := "Hello World!"

	msgStream := ms.New(conn)

	msgStream.SetKeys(ms.GenerateRSAKeyPair())

	err = msgStream.SendMessage(TestMessage, header, payload)
	assert.Nil(t, err)
	if err != nil {
		return
	}

	anyErrors := false
	go func() {
		for err := range msgStream.Errors() {
			anyErrors = true
			log.Println(err)
		}
	}()

	sentMsg := <-msgStream.Receiver()

	assert.NotNil(t, sentMsg)
	if sentMsg == nil {
		return
	}
	assert.Equal(t, TestMessage, sentMsg.Type)

	sentPayload, sentMetadata, err := ms.Unwrap[string](sentMsg)
	assert.Nil(t, err)
	if err != nil {
		return
	}

	assert.Equal(t, payload, sentPayload)

	assert.NotNil(t, sentMetadata)
	if sentMetadata != nil {
		_, exists := sentMetadata["creation_time"]
		assert.True(t, exists)
	}

	assert.False(t, anyErrors)
}

func TestEncryptedMetadataOnlyMessageTransfer(t *testing.T) {
	conn, err := getConn(true)
	assert.Nil(t, err)
	if err != nil {
		return
	}

	header := map[string]any{
		"creation_time": time.Now().UTC(),
	}

	msgStream := ms.New(conn)

	msgStream.SetKeys(ms.GenerateRSAKeyPair())

	err = msgStream.SendMessage(TestMessage, header, nil)
	assert.Nil(t, err)
	if err != nil {
		return
	}

	anyErrors := false
	go func() {
		for err := range msgStream.Errors() {
			anyErrors = true
			log.Println(err)
		}
	}()

	sentMsg := <-msgStream.Receiver()

	assert.NotNil(t, sentMsg)
	if sentMsg == nil {
		return
	}
	assert.Equal(t, TestMessage, sentMsg.Type)

	_, sentMetadata, err := ms.Unwrap[any](sentMsg)
	assert.Nil(t, err)

	assert.NotNil(t, sentMetadata)
	if sentMetadata != nil {
		_, exists := sentMetadata["creation_time"]
		assert.True(t, exists)
	}

	assert.False(t, anyErrors)
}

func TestEncryptedPayloadOnlyMessageTransfer(t *testing.T) {
	conn, err := getConn(true)
	assert.Nil(t, err)
	if err != nil {
		return
	}

	payload := "Hello World!"

	msgStream := ms.New(conn)

	msgStream.SetKeys(ms.GenerateRSAKeyPair())

	err = msgStream.SendMessage(TestMessage, nil, payload)
	assert.Nil(t, err)
	if err != nil {
		return
	}

	anyErrors := false
	go func() {
		for err := range msgStream.Errors() {
			anyErrors = true
			log.Println(err)
		}
	}()

	sentMsg := <-msgStream.Receiver()

	assert.NotNil(t, sentMsg)
	if sentMsg == nil {
		return
	}
	assert.Equal(t, TestMessage, sentMsg.Type)

	sentPayload, _, err := ms.Unwrap[string](sentMsg)
	assert.Nil(t, err)
	if err != nil {
		return
	}

	assert.Equal(t, payload, sentPayload)
	assert.False(t, anyErrors)
}

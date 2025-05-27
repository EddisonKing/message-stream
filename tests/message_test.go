package tests

import (
	"log"
	"testing"
	"time"

	ms "github.com/EddisonKing/message-stream"

	"github.com/stretchr/testify/assert"
)

func TestFullMessageTransfer(t *testing.T) {
	conn, err := getConn()
	assert.Nil(t, err)
	if err != nil {
		return
	}

	header := map[string]any{
		"creation_time": time.Now().UTC(),
	}
	payload := "Hello World!"

	msgStream, err := ms.New(conn, getOptions())
	assert.Nil(t, err)
	if err != nil {
		return
	}

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
	conn, err := getConn()
	assert.Nil(t, err)
	if err != nil {
		return
	}

	header := map[string]any{
		"creation_time": time.Now().UTC(),
	}

	msgStream, err := ms.New(conn, getOptions())
	assert.Nil(t, err)
	if err != nil {
		return
	}

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
	conn, err := getConn()
	assert.Nil(t, err)
	if err != nil {
		return
	}

	payload := "Hello World!"

	msgStream, err := ms.New(conn, getOptions())
	assert.Nil(t, err)
	if err != nil {
		return
	}

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
	conn, err := getEncryptedConn()
	assert.Nil(t, err)
	if err != nil {
		return
	}

	header := map[string]any{
		"creation_time": time.Now().UTC(),
	}
	payload := "Hello World!"

	msgStream, err := ms.New(conn, getOptionsForEncryption())
	assert.Nil(t, err)
	if err != nil {
		return
	}

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
	conn, err := getEncryptedConn()
	assert.Nil(t, err)
	if err != nil {
		return
	}

	header := map[string]any{
		"creation_time": time.Now().UTC(),
	}

	msgStream, err := ms.New(conn, getOptionsForEncryption())
	assert.Nil(t, err)
	if err != nil {
		return
	}

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
	conn, err := getEncryptedConn()
	assert.Nil(t, err)
	if err != nil {
		return
	}

	payload := "Hello World!"

	msgStream, err := ms.New(conn, getOptionsForEncryption())
	assert.Nil(t, err)
	if err != nil {
		return
	}

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

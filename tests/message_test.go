package tests

import (
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
}

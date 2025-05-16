package tests

import (
	"bytes"
	"log"
	"testing"
	"time"

	"github.com/EddisonKing/message-stream"

	"github.com/stretchr/testify/assert"
)

const TestMessage = messagestream.MessageType("test")

func TestFullMessageTransferLocally(t *testing.T) {
	header := map[string]any{
		"creation_time": time.Now().UTC(),
	}
	payload := "Hello World!"
	msg, err := messagestream.NewMessage(TestMessage, header, payload)
	assert.Nil(t, err)
	if err != nil {
		return
	}

	buffer := bytes.NewBuffer(nil)

	msgStream, err := messagestream.New(buffer)
	assert.Nil(t, err)
	if err != nil {
		return
	}

	msgStream.SendMessage(msg)

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

	sentPayload, err := messagestream.ExtractPayload[string](sentMsg)
	assert.Nil(t, err)
	if err != nil {
		return
	}

	assert.Equal(t, payload, sentPayload)

	sentMetadata := messagestream.ExtractMetadata(sentMsg)
	assert.NotNil(t, sentMetadata)
	if sentMetadata != nil {
		_, exists := sentMetadata["creation_time"]
		assert.True(t, exists)
	}

	assert.False(t, anyErrors)
}

func TestMetadataOnlyMessageTransferLocally(t *testing.T) {
	header := map[string]any{
		"creation_time": time.Now().UTC(),
	}
	msg, err := messagestream.NewMessage(TestMessage, header, nil)
	assert.Nil(t, err)
	if err != nil {
		return
	}

	buffer := bytes.NewBuffer(nil)

	msgStream, err := messagestream.New(buffer)
	assert.Nil(t, err)
	if err != nil {
		return
	}

	msgStream.SendMessage(msg)

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

	sentMetadata := messagestream.ExtractMetadata(sentMsg)
	assert.NotNil(t, sentMetadata)
	if sentMetadata != nil {
		_, exists := sentMetadata["creation_time"]
		assert.True(t, exists)
	}

	assert.False(t, anyErrors)
}

func TestPayloadOnlyMessageTransferLocally(t *testing.T) {
	payload := "Hello World!"
	msg, err := messagestream.NewMessage(TestMessage, nil, payload)
	assert.Nil(t, err)
	if err != nil {
		return
	}

	buffer := bytes.NewBuffer(nil)

	msgStream, err := messagestream.New(buffer)
	assert.Nil(t, err)
	if err != nil {
		return
	}

	msgStream.SendMessage(msg)

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

	sentPayload, err := messagestream.ExtractPayload[string](sentMsg)
	assert.Nil(t, err)
	if err != nil {
		return
	}

	assert.Equal(t, payload, sentPayload)
	assert.False(t, anyErrors)
}

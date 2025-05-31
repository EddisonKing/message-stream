package tests

import (
	"sync"
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

func TestMessageOnetimeCallbackFiredOnce(t *testing.T) {
	conn, err := getConn()
	assert.Nil(t, err)
	if err != nil {
		return
	}

	msgStream, err := ms.New(conn, getOptions())
	assert.Nil(t, err)
	if err != nil {
		return
	}

	wg := &sync.WaitGroup{}
	mu := &sync.Mutex{}

	count := 0
	wg.Add(1)

	msgStream.OnOne(TestMessage, func(msg *ms.Message) {
		mu.Lock()
		defer mu.Unlock()

		count++
		wg.Done()
	})

	msgStream.SendMessage(TestMessage, nil, nil)
	msgStream.SendMessage(TestMessage, nil, nil)

	wg.Wait()
	assert.Equal(t, 1, count)
}

func TestMessageCallbackFiredMultiple(t *testing.T) {
	conn, err := getConn()
	assert.Nil(t, err)
	if err != nil {
		return
	}

	msgStream, err := ms.New(conn, getOptions())
	assert.Nil(t, err)
	if err != nil {
		return
	}

	wg := &sync.WaitGroup{}
	mu := &sync.Mutex{}

	count := 0
	wg.Add(2)

	msgStream.On(TestMessage, func(msg *ms.Message) {
		mu.Lock()
		defer mu.Unlock()

		count++
		wg.Done()
	})

	msgStream.SendMessage(TestMessage, nil, nil)
	msgStream.SendMessage(TestMessage, nil, nil)

	wg.Wait()
	assert.Equal(t, 2, count)
}

func TestMessageOnetimeReadFiredOnce(t *testing.T) {
	conn, err := getConn()
	assert.Nil(t, err)
	if err != nil {
		return
	}

	msgStream, err := ms.New(conn, getOptions())
	assert.Nil(t, err)
	if err != nil {
		return
	}

	msgStream.SendMessage(TestMessage, nil, nil)
	msg := msgStream.ReadOne(TestMessage)
	assert.NotNil(t, msg)
	assert.Equal(t, TestMessage, msg.Type)

	timeout := time.NewTimer(time.Millisecond * 150).C

	second := make(chan *ms.Message, 1)
	go func() {
		second <- msgStream.ReadOne(TestMessage)
	}()

	select {
	case <-timeout:
		return
	case <-second:
		assert.FailNow(t, "Should not have been able to read second message")
	}
}

func TestMessageReadFiredMultiple(t *testing.T) {
	conn, err := getConn()
	assert.Nil(t, err)
	if err != nil {
		return
	}

	msgStream, err := ms.New(conn, getOptions())
	assert.Nil(t, err)
	if err != nil {
		return
	}

	count := 0
	wg := &sync.WaitGroup{}

	wg.Add(1)
	go func() {
		msgs := msgStream.Read(TestMessage)
		for range msgs {
			count++
			if count >= 2 {
				wg.Done()
				break
			}
		}
	}()

	msgStream.SendMessage(TestMessage, nil, nil)
	msgStream.SendMessage(TestMessage, nil, nil)

	wg.Wait()
	assert.Equal(t, 2, count)
}

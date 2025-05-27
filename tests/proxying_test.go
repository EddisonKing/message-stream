package tests

import (
	"sync"
	"testing"

	ms "github.com/EddisonKing/message-stream"
	"github.com/stretchr/testify/assert"
)

func TestProxying(t *testing.T) {
	serverHop, err := ms.Listen("127.0.0.1:9193", getOptionsForEncryption())
	assert.Nil(t, err)
	if err != nil {
		return
	}
	defer serverHop.Close()

	assert.NotNil(t, serverHop)

	go func() {
		conn, err := serverHop.Accept()
		assert.Nil(t, err)
		if err != nil {
			panic(err)
		}

		for range conn.Receiver() {
			// Empty since it won't receive messages
		}
	}()

	serverDestination, err := ms.Listen("127.0.0.1:9194", getOptionsForEncryption())
	assert.Nil(t, err)
	if err != nil {
		return
	}
	defer serverDestination.Close()

	assert.NotNil(t, serverDestination)

	wg := &sync.WaitGroup{}

	wg.Add(1)
	go func() {
		conn, err := serverDestination.Accept()
		assert.Nil(t, err)
		if err != nil {
			panic(err)
		}

		for range conn.Receiver() {
			// We just need to arbitrarily handle any messages
			if err := conn.SendMessage(ReplyMessage, nil, nil); err != nil {
				panic(err)
			}
			wg.Done()
		}
	}()

	client, err := ms.Dial("127.0.0.1:9193", getOptionsForEncryption())
	assert.Nil(t, err)
	if err != nil {
		return
	}
	defer client.Close()

	assert.NotNil(t, client)

	client.SendMessage(TestMessage, nil, nil, "127.0.0.1:9194")
	wg.Wait()

	reply := <-client.Receiver()

	assert.NotNil(t, reply)
	assert.Equal(t, ReplyMessage, reply.Type)
}

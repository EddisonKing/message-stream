package tests

import (
	"testing"

	ms "github.com/EddisonKing/message-stream"

	"github.com/stretchr/testify/assert"
)

func TestServerStart(t *testing.T) {
	server, err := ms.Listen("127.0.0.1:9192", nil)
	assert.Nil(t, err)
	if err != nil {
		return
	}
	defer server.Close()

	assert.NotNil(t, server)
}

func TestClientServerConnect(t *testing.T) {
	server, err := ms.Listen("127.0.0.1:9192", nil)
	assert.Nil(t, err)
	if err != nil {
		return
	}
	defer server.Close()

	assert.NotNil(t, server)

	go func() {
		server.Accept()
	}()

	client, err := ms.Dial("127.0.0.1:9192", nil)
	assert.Nil(t, err)
	if err != nil {
		return
	}
	defer client.Close()

	assert.NotNil(t, client)
}

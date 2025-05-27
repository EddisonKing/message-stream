package messagestream

import (
	"errors"
	"net"
	"sync"
)

var ErrListenerClosed = errors.New("listener already closed")

// Represents a server capable of accepting Message Stream connections
type MessageStreamListener struct {
	listener net.Listener
	opts     *MessageStreamOptions
	mu       *sync.Mutex
	closed   bool
}

// Creates a new Message Stream Server. The server will accept Message Stream connections and use `opts` when treating new incoming connections
func Listen(addr string, opts *MessageStreamOptions) (*MessageStreamListener, error) {
	if opts == nil {
		opts = NewMessageStreamOptions()
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	return &MessageStreamListener{
		listener: listener,
		opts:     opts,
		mu:       &sync.Mutex{},
		closed:   false,
	}, nil
}

// Accepts a new Message Stream connection. This function blocks until a connection is accepted
func (listener *MessageStreamListener) Accept() (*MessageStream, error) {
	listener.mu.Lock()
	defer listener.mu.Unlock()

	if listener.closed {
		return nil, ErrListenerClosed
	}

	conn, err := listener.listener.Accept()
	if err != nil {
		return nil, err
	}

	stream, err := New(conn, listener.opts)
	if err != nil {
		return nil, err
	}

	return stream, err
}

// Closes the underlying `net.Listener` and prevents accepting anymore Message Streams on this server
func (listener *MessageStreamListener) Close() error {
	listener.mu.Lock()
	defer listener.mu.Unlock()

	if listener.closed {
		return nil
	}

	listener.closed = true

	if err := listener.listener.Close(); err != nil {
		return err
	}

	return nil
}

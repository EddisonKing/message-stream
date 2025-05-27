package messagestream

import (
	"net"
)

// Dials a Message Stream endpoint and creates a Message Stream from the connection using the options provided
func Dial(addr string, opts *MessageStreamOptions) (*MessageStream, error) {
	if opts == nil {
		opts = NewMessageStreamOptions()
	}

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}

	stream, err := New(conn, opts)
	if err != nil {
		return nil, err
	}

	return stream, nil
}

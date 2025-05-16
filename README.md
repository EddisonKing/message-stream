# MessageStream

## Purpose
A simple structured communication layer supporting security through RSA encryption and signing.

Both sides create a Message Stream which builds a secured connection under the hood by exchanging RSA public keys. All messages carry a nonce to protect against replay attacks.
Messages are encrypted with the recipient's public key so only they can read it. Sent Messages are also signed and verified by the receiver. 
Messages are Typed and can easily be used in `switch` statements on the receiving end of the Message Stream. 
Messages also contain additional optional metadata for more flexibility. E.g. I want to add a creation timestamp to my message, but I don't want to add that as a property to the struct being sent as the Message payload.

**NOTE:** This library works, but I am aware of several issues and improvements I'd like to make to it before considering it production worthy.

## Usage

```go

var TestMessage = messagestream.MessageType("test")

conn, _ := Net.Dial("tcp", "someaddress:1234")

// Doesn't need to be a net.Conn, anything implementing io.ReadWriter 
// could be used. But a net.Conn is a logical and intended use case.
// Errors will only occur if there is a problem generating an RSA 
// private/public key, or more likely, there is a problem negotiating the
// initial connection with the target. i.e. the other side is not using 
// a MessageStream.
//
// It is inadvisable to send or receive on the conn after creating a 
// Message Stream from it.
stream, err := messagestream.New(conn)

// Loop through received messages
go func() {
  for msg := range stream.Receiver() {
    switch msg.Type {
      case TestMessage:
        // Do something with the msg
        // ...
    }
  }
}()

// Loop through any error events
// Errors are only sent here if they don't break the flow, such as reading
// bad data that can't be parsed as a message. This returns an error here,
// but if other successful messages can be parsed, the Message Stream will
// continue.
// Effectively, this is intended for alerting potentially transient errors.
go func() {
  for err := range stream.Errors() {
    log.Printf("Error from Message Stream: %s", err)
  }
}()

// Example metadata to send with the message. This is optional. Pass nil if
// not required.
metadata := map[string]any {
  "creation_time": time.Now().UTC(),
}

// This will error if unable to serialize either the metadata or payload. 
// Payload is `any` type, but a string for this example. More likely a 
// struct would be sent.
testMsg, err := messagestream.NewMessage(TestMessage, metadata, "hello")

// Send a message. This will encrypt and sign the message before sending.
// This will only error if the underlying io.Writer (or io.ReadWriter) is 
// closed.
err := stream.SendMessage(testMsg)
```

### Examples
Have a look at the chat client/server implementation using Message Streams in the `./examples` folder. Keep in mind that a real implementation would need more work to handle network resiliency such as lost messages, etc, as well as handling disconnect messages.

## Improvements & Known Issues
1. The tests use a `bytes.Buffer` for ease of test setup. This isn't a real use case and should be a network connection. The tests will sometimes stop suddenly after about ~20 tries due to a race condition between reading and writing to the `bytes.Buffer`.
2. Performance at scale was not an initial requirement of this project and no testing has been done on how scalable and fast this protocol actually is. 
3. It uses `encoding/json` to serialise and deserialise the metadata and payload. This is likely having a performance impact, both on the process of serialisation, as well as network bloat with the resultant size in bytes.
4. In the read loop, the Message Stream waits for a sequence of "magic bytes" to determine Message boundaries. It disregards everything it reads until it sees this sequence. Losing data is not ideal, but it would be impossible to reconstruct a full message otherwise.

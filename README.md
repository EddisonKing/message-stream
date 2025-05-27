# MessageStream

## Purpose
A simple structured communication layer supporting security through RSA encryption and signing. The protocol also comes with a read made server and client and supports proxying Messages through multiple servers across networks.

Both sides create a Message Stream which builds a secured connection under the hood by exchanging RSA public keys. All messages carry a nonce to protect against replay attacks.
Messages are encrypted with the recipient's public key so only they can read it. Sent Messages are also signed by the sender and verified by the receiver.
Messages are Typed and can easily be used in `switch` statements on the receiving end of the Message Stream. 
Messages also contain additional optional metadata for more flexibility. E.g. I want to add a creation timestamp to my message, but I don't want to add that as a property to the struct being sent as the Message payload.

## Usage

### Getting Started
#### Installation
```bash
go get github.com/EddisonKing/message-stream
```

#### Import & Alias
```go
import (
  ms "github.com/EddisonKing/message-stream"
)
```

Because of the length of the package name, it is easier to alias it as something simple, like `ms`. The following examples assume this alias has been used, otherwise, reference it as `messagestream`.

### Definition of New Message Type
```go
var TestMessage = ms.MessageType("test")
```

Message Types are strings that make it easier to differentiate Messages as they are received at the Message Stream's Receiver by the consumer of the Messages. A Message Type allows the consumer to know what Type to ExtractPayload as. 

### Creating and running a Message Stream Server
```go
listener, err := ms.Listen("0.0.0.0:1337", nil)

defer listener.Close()

client, err := listener.Accept()
```

To run a Message Stream Server, call `Listen` passing in an address to listen on and an optional `MessageStreamOptions`. It behaves very similar to a `net.Listener` since under the hood, it is creating a TCP listener and wrapping incoming connections in a Message Stream.

Passing `nil` to the `opts` when calling `Listen` or `Dial` (or even constructing the Message Stream directly with `New` or `NewFrom`) will result in the Message Stream using the default configuration. In the default configuration, a private and public RSA key of 2048 bits will be generated and used.

### Connecting to a Message Stream Server as a Client
```go
client, err := ms.Dial("127.0.0.1:1337", nil)

defer client.Close()
```

This takes an address string for an existing Message Stream Server and tries to connect to it. If successful, it will return back a pointer to a Message Stream that can be used to interact with the server. Both client and server must use encryption, or neither should, similar to TLS.

### Creating a new Message Stream from a ReadWriter, Reader or Writer
```go
opts := ms.NewMessageStreamOptions()

stream, err := ms.New(rw, opts)
// OR
stream, err := ms.NewFrom(r, w, opts)
```

Another way to create a MessageStream is to call `New` and pass in anything that implements `io.ReadWriter`, however it is also possible to call `NewFrom` which takes an `io.Reader` and `io.Writer`.

Message Streams support the `Close` operation which does multiple things, including calling close on the underlying channels for receiving Messages and internally generated errors, as well as closing the `io.Reader` and `io.Writer`. 

The `io.ReaderWriter`, `io.Reader` and `io.Writer` parts given to a Message Stream should not be used by the caller after the MessageStream struct has been created. Writes and reads from the underlying parts could result in undefined behaviour.

**Note:** For added security when intending to use a `net.Conn` to create a Message Stream, it is best to use `crypto/tls` (from https://pkg.go.dev/crypto/tls), and wrap the connection in TLS, ideally with a valid Certificate, although self-signed in a controlled environment is good too. The TLS layer will prevent Man-in-the-middle attacks, as well as hiding the byte boundaries between already encrypted Message Stream Messages. Message Stream already uses nonces to prevent replay attacks, but encrypting the entirety of the traffic, not just payloads and metadata, is much more secure.

### Receiving Messages
```go
for msg := range stream.Receiver() {
  switch msg.Type {
    case TestMessage:
      payload, metadata, err := ms.Unwrap[string](msg)
      // Do something with the msg
      // ...
  }
}
```

`Receiver()` will return a read-only channel of `*Message`. Message Streams are primarily intended for reading in multiple Messages, hence why the receiving of messages is handled by a channel, not a once-off "Read Message"-like function. 

### Sending Messages
```go
err := stream.SendMessage(TestMessage, map[string]any {
  "creation_time": time.Now().UTC(),
}, "hello")
```

Messages contain both a payload and metadata. The payload is `any` type that can be serialised using `encoding/json`. Similary, metadata is `map[string]any` where the `any` type must also be serialisable.
Whereas payload is the actual message to be sent, metadata is a way to apply additional information that might not be directly related to the message. For example, a timestamp of when the Message was created, while not directly relevent to the payload, might be useful to the recipient.

This will error if unable to serialize either the metadata or payload. A `nil` can be passed to both the `metadata` and `payload` field, effectively being an empty Message that can be treated by the receiver like an event.

### Logging
```go
// Create slog logger
logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
	Level: slog.LevelDebug,
}))

// Create instance of default Message Stream options and set the logger
opts := ms.NewMessageStreamOptions()
opts.Logger = logger

// Then pass opts to Message Stream
```

The Message Stream library surfaces a `MessageStreamOptions` struct allowing for a `slog.Logger` to be passed in for logging. No payloads are ever logged for security reasons. Setting `DeepLogging` to `true` will forward the logger onto the underlying `on-the-wire` library which will produce a significant amount of debugging logging raw bytes being transferred and transformed before and after reading/writing. No payloads are logged for security reasons. 

### Handling Errors
```go
// 
for err := range stream.Errors() {
  log.Printf("Error from Message Stream: %s", err)
}
```

Errors are only sent here if they don't break the normal flow, such as reading bad data that can't be parsed as a message but the underlying `io.Reader` and `io.Writer` are operational. This returns an error here, but if other successful messages can be parsed, the Message Stream will continue processing.

Effectively, this is intended for alerting potentially transient errors.


### Proxying
```go
err := stream.SendMessage(TestMessage, nil, nil, "172.16.0.1:8080", "192.168.10.1:4444") 
```

Message Streams support a type of proxying whereby appending strings to the variadic `SendMessage` function that represent network addresses will tell the receiving Message Stream endpoint to forward on the Message to that endpoint. The last address in the list is the recipient of the Message. Message replies will flow through back to the sender of the original Message. This proxying creates a subsequent transient Message Stream upon a TCP connection to the proxy, which means the proxy host must be running a Message Stream Server or have wrapped their own Message Stream around a network connection.

By default, Message Streams do not allow this proxying and an error will be thrown if the Message Stream wasn't created with it's `MessageStreamOptions.AllowProxying` set to `true`. This is because the host of a proxy in a list could intercept and modify the Messages. Unless you run or trust the proxies, or implement additional security surround Message payloads, you should avoid allowing proxying.

### Examples
Have a look at the chat client/server implementation using Message Streams in the `./examples` folder. Keep in mind that a real implementation would need more work to handle network resiliency such as notifying of a disconnect, etc.

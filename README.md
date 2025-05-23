# MessageStream

## Purpose
A simple structured communication layer supporting security through RSA encryption and signing.

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

### Explicit Definition of New Message Type
```go
var TestMessage = ms.MessageType("test")
```

Message Types are strings that make it easier to differentiate Messages as they are received at the Message Stream's Receiver by the consumer of the Messages. A Message Type allows the consumer to know what Type to ExtractPayload as. 

### Creating a new Message Stream
```go
stream := ms.New(rw)
// OR
stream := ms.NewFrom(r, w)
```

The most common way to create a MessageStream is to call `New` and pass in anything that implements `io.ReadWriter`, however it is also possible to call `NewFrom` which takes an `io.Reader` and `io.Writer`.

Message Streams support the `Close` operation which does multiple things, including calling close on the underlying channels for receiving Messages and internally generated errors, as well as closing the `io.Reader` and `io.Writer`. 

The `io.ReaderWriter`, `io.Reader` and `io.Writer` parts given to a Message Stream should not be used by the caller after the MessageStream struct has been created. Writes and reads from the underlying parts could result in undefined behaviour.

**Note:** For added security when intending to use a `net.Conn` to create a Message Stream, it is best to use `crypto/tls` (from https://pkg.go.dev/crypto/tls), and wrap the connection in TLS, ideally with a valid Certificate, although self-signed in a controlled environment is good too. The TLS layer will prevent Man-in-the-middle attacks, as well as hiding the byte boundaries between already encrypted Message Stream Messages. Message Stream already uses nonces to prevent replay attacks, but encrypting the entirety of the traffic, not just payloads and metadata, is much more secure.

### Enabling Encryption
Encryption is not enabled by default since RSA private and public keys need to be provided.
```go
stream.SetKeys(privateKey, publicKey)
err := stream.Connect()
```

You should call `Connect()` after `SetKeys()` so that the Message Stream performs a public key exchange. `Connect()` will be called automatically if keys are set and you attempt to `SendMessage()`.

Errors will occur if there is a problem generating an RSA private/public key, or more likely, there is a problem negotiating the initial connection with the target. ex. the other side is not using a MessageStream.

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

`Receiver()` will return a read-only channel of `*Message`. Message Streams are primarily intended for reading in multiple Messages, hence why the receiving of messages is handled by a channel, not a once-off "Read Message" function. 

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
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	messagestream.SetLogger(logger)
```

The Message Stream library surfaces a `SetLogger` function at the package level allowing for a `slog.Logger` to be passed in for logging. No payloads are ever logged for security reasons. Message signatures and hashes are logged for debugging. At `Debug` log level, the logs are verbose and numerous and will slow `stdout` considerably. 

### Handling Errors
```go
// 
for err := range stream.Errors() {
  log.Printf("Error from Message Stream: %s", err)
}
```

Errors are only sent here if they don't break the normal flow, such as reading bad data that can't be parsed as a message but the underlying `io.Reader` and `io.Writer` are operational. This returns an error here, but if other successful messages can be parsed, the Message Stream will continue processing.

Effectively, this is intended for alerting potentially transient errors.

### Examples
Have a look at the chat client/server implementation using Message Streams in the `./examples` folder. Keep in mind that a real implementation would need more work to handle network resiliency such as notifying of a disconnect, etc.

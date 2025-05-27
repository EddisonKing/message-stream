package messagestream

import (
	"crypto/rand"
	"encoding/binary"
	"log/slog"
	"time"
)

func randomInt() int {
	bytes := make([]byte, 4)
	if _, err := rand.Read(bytes); err != nil {
		panic(err)
	}

	return int(binary.BigEndian.Uint32(bytes))
}

const defaultNonceTTL time.Duration = time.Minute * 30

type nonceManager struct {
	history map[int]time.Time
	ttl     time.Duration
	logger  *slog.Logger
	name    string
}

func newNonceManager(ttl time.Duration, name string, logger *slog.Logger) *nonceManager {
	return &nonceManager{
		history: make(map[int]time.Time),
		ttl:     ttl,
		logger:  logger,
		name:    name,
	}
}

func (nm *nonceManager) Generate() int {
	nm.logger.Debug("Generating nonce", "Name", nm.name)
	for {
		n := randomInt()
		nm.logger.Debug("Generated Nonce", "Nonce", n, "Name", nm.name)
		if !nm.Contains(n) {
			nm.Store(n)
			return n
		}
	}
}

func (nm *nonceManager) Store(nonce int) {
	nm.logger.Debug("Storing", "Nonce", nonce, "Name", nm.name)
	nm.history[nonce] = time.Now().UTC()
	nm.logger.Debug("Stored", "Nonce", nonce, "Name", nm.name)
}

func (nm *nonceManager) Contains(nonce int) bool {
	nm.logger.Debug("Checking nonce", "Nonce", nonce, "Name", nm.name)
	if createdTime, exists := nm.history[nonce]; exists {
		nm.logger.Debug("Nonce has already been seen", "Nonce", nonce, "Created", createdTime, "Name", nm.name)
		return time.Since(createdTime) > nm.ttl
	}

	nm.logger.Debug("Nonce is good to go!", "Nonce", nonce, "Name", nm.name)
	return false
}

func (nm *nonceManager) NotContains(nonce int) bool {
	return !nm.Contains(nonce)
}

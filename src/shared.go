package src

import (
	"fmt"

	"golang.org/x/crypto/sha3"
)

// Hash hashes the input and returns the result hash
// The input gets hashed with sha3-256
func Hash(in string) string {
	h := sha3.New256()
	h.Write([]byte(in))
	return fmt.Sprintf("%x", h.Sum(nil))
}

// SendMeta is the data that gets send over the websocket
type SendMeta struct {
	Title         string `json:"title"`
	ID            string `json:"ID"`
	MessageID     string `json:"messageID"`
	ExpectsAnswer bool   `json:"expectsAnswer"`
}

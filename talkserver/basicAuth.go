package talkserver

import (
	"bytes"
)

// AuthWithKey is an auth function that
// If the key is an empty string it will panic
// This returns a function that can be used as Options.Auth
func AuthWithKey(key string) func(msg []byte) ([]byte, bool) {
	if key == "" {
		panic("AuthWithKey key is empty")
	}

	hashedKey := []byte(calcSha3(key))

	return func(msg []byte) ([]byte, bool) {
		if !bytes.HasPrefix(msg, hashedKey) {
			return nil, false
		}

		return bytes.TrimPrefix(msg, hashedKey), true
	}
}

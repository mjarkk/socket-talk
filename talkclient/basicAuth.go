package talkclient

import (
	"github.com/mjarkk/socket-talk/src"
)

// AuthWithKey returns a handeler for Options.Auth
// If key is empty this function will panic
func AuthWithKey(key string) func([]byte) []byte {
	if key == "" {
		panic("AuthWithKey key is empty")
	}

	hashedKey := []byte(src.Hash(key))

	return func(in []byte) []byte {
		return append(hashedKey, in...)
	}
}

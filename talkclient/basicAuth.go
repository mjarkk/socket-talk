package talkclient

// AuthWithKey returns a handeler for Options.Auth
// If key is empty this function will panic
func AuthWithKey(key string) func([]byte) []byte {
	if key == "" {
		panic("AuthWithKey key is empty")
	}

	hashedKey := []byte(calcSha3(key))

	return func(in []byte) []byte {
		return append(hashedKey, in...)
	}
}

package store

import (
	"crypto/rand"
	"encoding/hex"
)

func newID() string {
	bytes := make([]byte, 8)

	_, err := rand.Read(bytes)
	if err != nil {
		panic(err)
	}

	return hex.EncodeToString(bytes)
}

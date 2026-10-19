package store

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
)

func newID() string {
	return randomHex(8)
}

func newToken() string {
	return randomHex(32)
}

func newParticipantPIN() string {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%06d", n.Int64())
}

func randomHex(size int) string {
	bytes := make([]byte, size)

	if _, err := rand.Read(bytes); err != nil {
		panic(err)
	}

	return hex.EncodeToString(bytes)
}

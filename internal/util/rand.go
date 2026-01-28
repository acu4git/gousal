package util

import (
	"crypto/rand"
	"encoding/hex"
)

var base62 = []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

func RandStringBase62(n int) (string, error) {
	b := make([]byte, n)
	r := make([]byte, n)

	if _, err := rand.Read(r); err != nil {
		return "", err
	}

	for i := range n {
		b[i] = base62[int(r[i])%len(base62)]
	}

	return string(b), nil
}

func HexSuffix() string {
	b := make([]byte, 4) // 32bit
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

package random

import "math/rand/v2"

const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ" +
	"abcdefghijklmnopqrstuvwxyz" +
	"0123456789"

func NewRandomString(size int) string {
	if size <= 0 {
		return ""
	}

	b := make([]byte, size)
	for i := range b {
		b[i] = chars[rand.IntN(len(chars))]
	}

	return string(b)
}

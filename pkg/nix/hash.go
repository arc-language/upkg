// hash.go
package nix

import (
	"fmt"
)

// Nix uses a special base32 alphabet (without E, O, U, T)
// See: https://github.com/kolloch/nix-base32
const nixBase32Alphabet = "0123456789abcdfghijklmnpqrsvwxyz"

// toNixBase32 converts a byte slice to a Nix-compatible base32 encoded string
func toNixBase32(bytes []byte) string {
	length := (len(bytes)*8-1)/5 + 1
	result := make([]byte, length)

	for n := 0; n < length; n++ {
		b := n * 5
		i := b / 8
		j := b % 8

		// bits from the lower byte
		v1 := byte(0)
		if j < 8 {
			v1 = bytes[i] >> uint(j)
		}

		// bits from the upper byte
		v2 := byte(0)
		if i < len(bytes)-1 {
			if 8-j < 8 {
				v2 = bytes[i+1] << uint(8-j)
			}
		}

		v := (v1 | v2) & 0x1F // Keep only 5 bits
		result[length-n-1] = nixBase32Alphabet[v]
	}

	return string(result)
}

// fromNixBase32 converts a Nix-compatible base32 encoded string to a byte slice
func fromNixBase32(s string) ([]byte, error) {
	hashSize := len(s) * 5 / 8
	hash := make([]byte, hashSize)

	for n := 0; n < len(s); n++ {
		c := s[len(s)-n-1]

		// Find digit value in alphabet
		digit := byte(0)
		found := false
		for idx, ch := range nixBase32Alphabet {
			if byte(ch) == c {
				digit = byte(idx)
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("invalid character in base32 string: %c", c)
		}

		b := n * 5
		i := b / 8
		j := b % 8

		// Set bits in lower byte
		if j < 8 {
			hash[i] |= digit << uint(j)
		}

		// Set bits in upper byte
		v2 := byte(0)
		if 8-j < 8 {
			v2 = digit >> uint(8-j)
		}

		if i < hashSize-1 {
			hash[i+1] |= v2
		} else if v2 != 0 {
			return nil, fmt.Errorf("invalid base32 encoding")
		}
	}

	return hash, nil
}
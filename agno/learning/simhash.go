package learning

import (
	"hash/fnv"
	"strings"
	"unicode"
)

func simHash64(text string) uint64 {
	tokens := tokenize(text)
	if len(tokens) == 0 {
		return 0
	}

	var weights [64]int
	for _, token := range tokens {
		h := fnvHash64(token)
		for bit := 0; bit < 64; bit++ {
			if (h>>uint(bit))&1 == 1 {
				weights[bit]++
			} else {
				weights[bit]--
			}
		}
	}

	var out uint64
	for bit := 0; bit < 64; bit++ {
		if weights[bit] > 0 {
			out |= 1 << uint(bit)
		}
	}
	return out
}

func fnvHash64(s string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return h.Sum64()
}

func hammingDistance64(a, b uint64) int {
	x := a ^ b
	count := 0
	for x != 0 {
		x &= x - 1
		count++
	}
	return count
}

func tokenize(s string) []string {
	var tokens []string
	var b strings.Builder
	b.Grow(32)

	flush := func() {
		if b.Len() == 0 {
			return
		}
		t := b.String()
		b.Reset()
		if len(t) >= 2 {
			tokens = append(tokens, t)
		}
	}

	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' {
			b.WriteRune(r)
			continue
		}
		flush()
	}
	flush()
	return tokens
}


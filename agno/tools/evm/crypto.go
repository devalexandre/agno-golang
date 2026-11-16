package evm

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math/big"
	"strings"

	"golang.org/x/crypto/sha3"
)

// Minimal secp256k1 / Keccak / RLP primitives needed to sign EIP-1559
// transactions without pulling in go-ethereum. The curve arithmetic uses
// math/big and is not constant-time; it signs a handful of transactions per
// run and keys never leave the process.

var (
	curveP, _  = new(big.Int).SetString("fffffffffffffffffffffffffffffffffffffffffffffffffffffffefffffc2f", 16)
	curveN, _  = new(big.Int).SetString("fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364141", 16)
	curveGx, _ = new(big.Int).SetString("79be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798", 16)
	curveGy, _ = new(big.Int).SetString("483ada7726a3c4655da4fbfc0e1108a8fd17b448a68554199c47d08ffb10d4b8", 16)
	halfN      = new(big.Int).Rsh(curveN, 1)
)

type point struct{ x, y *big.Int } // nil x = point at infinity

func (p point) inf() bool { return p.x == nil }

func modP(v *big.Int) *big.Int { return v.Mod(v, curveP) }

func pointAdd(a, b point) point {
	if a.inf() {
		return b
	}
	if b.inf() {
		return a
	}
	var lambda *big.Int
	if a.x.Cmp(b.x) == 0 {
		if new(big.Int).Add(a.y, b.y).Mod(new(big.Int).Add(a.y, b.y), curveP).Sign() == 0 {
			return point{}
		}
		// lambda = 3x^2 / 2y
		num := new(big.Int).Mul(a.x, a.x)
		num.Mul(num, big.NewInt(3))
		den := new(big.Int).Lsh(a.y, 1)
		den.ModInverse(modP(den), curveP)
		lambda = modP(num.Mul(num, den))
	} else {
		num := new(big.Int).Sub(b.y, a.y)
		den := new(big.Int).Sub(b.x, a.x)
		den.ModInverse(modP(den), curveP)
		lambda = modP(num.Mul(num, den))
	}
	x := new(big.Int).Mul(lambda, lambda)
	x.Sub(x, a.x).Sub(x, b.x)
	modP(x)
	y := new(big.Int).Sub(a.x, x)
	y.Mul(y, lambda).Sub(y, a.y)
	modP(y)
	return point{x, y}
}

func scalarMult(k *big.Int, p point) point {
	r := point{}
	for i := k.BitLen() - 1; i >= 0; i-- {
		r = pointAdd(r, r)
		if k.Bit(i) == 1 {
			r = pointAdd(r, p)
		}
	}
	return r
}

func baseMult(k *big.Int) point { return scalarMult(k, point{curveGx, curveGy}) }

func keccak256(data ...[]byte) []byte {
	h := sha3.NewLegacyKeccak256()
	for _, d := range data {
		h.Write(d)
	}
	return h.Sum(nil)
}

func pad32(b []byte) []byte {
	out := make([]byte, 32)
	copy(out[32-len(b):], b)
	return out
}

// parsePrivateKey parses a hex private key (with or without 0x).
func parsePrivateKey(s string) (*big.Int, error) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "0x")
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != 32 {
		return nil, errors.New("private key must be 32 bytes of hex")
	}
	d := new(big.Int).SetBytes(b)
	if d.Sign() == 0 || d.Cmp(curveN) >= 0 {
		return nil, errors.New("private key out of range")
	}
	return d, nil
}

// pubToAddress returns the EIP-55 checksummed address of a public key.
func pubToAddress(p point) string {
	h := keccak256(pad32(p.x.Bytes()), pad32(p.y.Bytes()))
	return checksumAddress(hex.EncodeToString(h[12:]))
}

func checksumAddress(lowerHex string) string {
	lowerHex = strings.ToLower(strings.TrimPrefix(lowerHex, "0x"))
	hash := hex.EncodeToString(keccak256([]byte(lowerHex)))
	out := []byte(lowerHex)
	for i := range out {
		if out[i] >= 'a' && hash[i] >= '8' {
			out[i] -= 32
		}
	}
	return "0x" + string(out)
}

// rfc6979K derives a deterministic nonce (RFC 6979, HMAC-SHA256).
func rfc6979K(d *big.Int, hash []byte) *big.Int {
	x := pad32(d.Bytes())
	h1 := pad32(new(big.Int).Mod(new(big.Int).SetBytes(hash), curveN).Bytes())
	v := make([]byte, 32)
	k := make([]byte, 32)
	for i := range v {
		v[i] = 1
	}
	mac := func(key []byte, parts ...[]byte) []byte {
		m := hmac.New(sha256.New, key)
		for _, p := range parts {
			m.Write(p)
		}
		return m.Sum(nil)
	}
	k = mac(k, v, []byte{0}, x, h1)
	v = mac(k, v)
	k = mac(k, v, []byte{1}, x, h1)
	v = mac(k, v)
	for {
		v = mac(k, v)
		t := new(big.Int).SetBytes(v)
		if t.Sign() > 0 && t.Cmp(curveN) < 0 {
			return t
		}
		k = mac(k, v, []byte{0})
		v = mac(k, v)
	}
}

// sign signs a 32-byte hash, returning r, s (low-s) and the recovery id.
func sign(d *big.Int, hash []byte) (r, s *big.Int, recID int) {
	z := new(big.Int).SetBytes(hash)
	k := rfc6979K(d, hash)
	R := baseMult(k)
	r = new(big.Int).Mod(R.x, curveN)
	recID = int(R.y.Bit(0))
	if R.x.Cmp(curveN) >= 0 {
		recID |= 2
	}
	kInv := new(big.Int).ModInverse(k, curveN)
	s = new(big.Int).Mul(r, d)
	s.Add(s, z).Mul(s, kInv).Mod(s, curveN)
	if s.Cmp(halfN) > 0 {
		s.Sub(curveN, s)
		recID ^= 1
	}
	return r, s, recID
}

// recoverPub recovers the public key from a signature (used by tests and
// as a post-signing self-check).
func recoverPub(hash []byte, r, s *big.Int, recID int) (point, error) {
	x := new(big.Int).Set(r)
	if recID&2 != 0 {
		x.Add(x, curveN)
	}
	// y^2 = x^3 + 7
	y2 := new(big.Int).Exp(x, big.NewInt(3), curveP)
	y2.Add(y2, big.NewInt(7)).Mod(y2, curveP)
	y := new(big.Int).ModSqrt(y2, curveP)
	if y == nil {
		return point{}, errors.New("invalid signature")
	}
	if int(y.Bit(0)) != recID&1 {
		y.Sub(curveP, y)
	}
	R := point{x, y}
	rInv := new(big.Int).ModInverse(r, curveN)
	z := new(big.Int).SetBytes(hash)
	negZ := new(big.Int).Neg(z)
	negZ.Mod(negZ, curveN)
	// Q = r^-1 (sR - zG)
	sR := scalarMult(s, R)
	zG := baseMult(negZ)
	Q := scalarMult(rInv, pointAdd(sR, zG))
	if Q.inf() {
		return point{}, errors.New("invalid signature")
	}
	return Q, nil
}

// --- RLP ---

func rlpBytes(b []byte) []byte {
	if len(b) == 1 && b[0] < 0x80 {
		return b
	}
	return append(rlpLen(len(b), 0x80), b...)
}

func rlpList(items ...[]byte) []byte {
	var body []byte
	for _, it := range items {
		body = append(body, it...)
	}
	return append(rlpLen(len(body), 0xc0), body...)
}

func rlpLen(n int, offset byte) []byte {
	if n < 56 {
		return []byte{offset + byte(n)}
	}
	lb := new(big.Int).SetInt64(int64(n)).Bytes()
	return append([]byte{offset + 55 + byte(len(lb))}, lb...)
}

func rlpInt(v *big.Int) []byte { return rlpBytes(v.Bytes()) } // big.Int.Bytes strips leading zeros

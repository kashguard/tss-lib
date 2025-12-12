// Copyright © 2019 Binance
//
// This file is part of Binance. The full Binance copyright notice, including
// terms governing use, modification, and redistribution, is contained in the
// file LICENSE at the root of the source code distribution tree.

package signing

import (
	"crypto/elliptic"
	"fmt"
	"io"
	"math/big"

	"github.com/agl/ed25519/edwards25519"

	"github.com/kashguard/tss-lib/common"
)

func encodedBytesToBigInt(s *[32]byte) *big.Int {
	// Use a copy so we don't screw up our original
	// memory.
	sCopy := new([32]byte)
	for i := 0; i < 32; i++ {
		sCopy[i] = s[i]
	}
	reverse(sCopy)

	bi := new(big.Int).SetBytes(sCopy[:])

	return bi
}

func bigIntToEncodedBytes(a *big.Int) *[32]byte {
	s := new([32]byte)
	if a == nil {
		return s
	}

	// Caveat: a can be longer than 32 bytes.
	s = copyBytes(a.Bytes())

	// Reverse the byte string --> little endian after
	// encoding.
	reverse(s)

	return s
}

func copyBytes(aB []byte) *[32]byte {
	if aB == nil {
		return nil
	}
	s := new([32]byte)

	// If we have a short byte string, expand
	// it so that it's long enough.
	aBLen := len(aB)
	if aBLen < 32 {
		diff := 32 - aBLen
		for i := 0; i < diff; i++ {
			aB = append([]byte{0x00}, aB...)
		}
	}

	for i := 0; i < 32; i++ {
		s[i] = aB[i]
	}

	return s
}

func ecPointToEncodedBytes(x *big.Int, y *big.Int) *[32]byte {
	s := bigIntToEncodedBytes(y)
	xB := bigIntToEncodedBytes(x)
	xFE := new(edwards25519.FieldElement)
	edwards25519.FeFromBytes(xFE, xB)
	isNegative := edwards25519.FeIsNegative(xFE) == 1

	if isNegative {
		s[31] |= (1 << 7)
	} else {
		s[31] &^= (1 << 7)
	}

	return s
}

// Note: RFC 8032 Ed25519 uses LITTLE-ENDIAN encoding, not big-endian.
// The tss-lib functions (bigIntToEncodedBytes, ecPointToEncodedBytes) already output
// in little-endian format, which is the correct format for standard Ed25519.

// PublicKeyToStandardEd25519 converts a tss-lib public key to standard Ed25519 format (RFC 8032)
// IMPORTANT: RFC 8032 Ed25519 uses LITTLE-ENDIAN encoding, not big-endian!
// The tss-lib output (ecPointToEncodedBytes) is already in little-endian format,
// so this function mainly ensures the format is correct and handles edge cases.
func PublicKeyToStandardEd25519(x, y *big.Int) [32]byte {
	// Use the existing ecPointToEncodedBytes which already outputs little-endian (RFC 8032 format)
	// This is the correct format for standard Ed25519
	return *ecPointToEncodedBytes(x, y)
}

// reverseBytes reverses a byte array (used for little-endian to big-endian conversion)
func reverseBytes(b []byte) {
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}
}

// littleEndianToBigEndian converts a little-endian 32-byte array to big-endian
// This is used when converting internal edwards25519 library output to standard Ed25519 format
func littleEndianToBigEndian(le *[32]byte) *[32]byte {
	be := new([32]byte)
	copy(be[:], le[:])
	reverseBytes(be[:])
	return be
}

// SignatureToStandardEd25519 converts a tss-lib signature to standard Ed25519 format (RFC 8032)
// IMPORTANT: RFC 8032 Ed25519 uses LITTLE-ENDIAN encoding!
// The tss-lib signature is already in little-endian format (R || S, each 32 bytes, little-endian),
// which is the correct format for standard Ed25519. This function mainly validates and ensures
// the format is correct.
//
// Parameters:
//   - signature: 64-byte signature in tss-lib format (R || S, little-endian, RFC 8032)
//
// Returns:
//   - 64-byte signature in standard Ed25519 format (same format, validated)
//   - error if signature length is not 64 bytes
func SignatureToStandardEd25519(signature []byte) ([]byte, error) {
	if len(signature) != 64 {
		return nil, fmt.Errorf("signature must be 64 bytes, got %d", len(signature))
	}

	// tss-lib signature is already in standard Ed25519 format (little-endian, RFC 8032)
	// Just return a copy to ensure we don't modify the original
	result := make([]byte, 64)
	copy(result, signature)
	return result, nil
}

func reverse(s *[32]byte) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

func addExtendedElements(p, q edwards25519.ExtendedGroupElement) edwards25519.ExtendedGroupElement {
	var r edwards25519.CompletedGroupElement
	var qCached edwards25519.CachedGroupElement
	q.ToCached(&qCached)
	edwards25519.GeAdd(&r, &p, &qCached)
	var result edwards25519.ExtendedGroupElement
	r.ToExtended(&result)
	return result
}

func ecPointToExtendedElement(ec elliptic.Curve, x *big.Int, y *big.Int, rand io.Reader) edwards25519.ExtendedGroupElement {
	encodedXBytes := bigIntToEncodedBytes(x)
	encodedYBytes := bigIntToEncodedBytes(y)

	z := common.GetRandomPositiveInt(rand, ec.Params().N)
	encodedZBytes := bigIntToEncodedBytes(z)

	var fx, fy, fxy edwards25519.FieldElement
	edwards25519.FeFromBytes(&fx, encodedXBytes)
	edwards25519.FeFromBytes(&fy, encodedYBytes)

	var X, Y, Z, T edwards25519.FieldElement
	edwards25519.FeFromBytes(&Z, encodedZBytes)

	edwards25519.FeMul(&X, &fx, &Z)
	edwards25519.FeMul(&Y, &fy, &Z)
	edwards25519.FeMul(&fxy, &fx, &fy)
	edwards25519.FeMul(&T, &fxy, &Z)

	return edwards25519.ExtendedGroupElement{
		X: X,
		Y: Y,
		Z: Z,
		T: T,
	}
}

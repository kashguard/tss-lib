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

// bigIntToEncodedBytesBigEndian converts a big.Int to a 32-byte array in BIG-ENDIAN format
// This is used for standard Ed25519 output (RFC 8032), which uses big-endian byte order
// Internal calculations still use little-endian (for edwards25519 library compatibility)
func bigIntToEncodedBytesBigEndian(a *big.Int) *[32]byte {
	s := new([32]byte)
	if a == nil {
		return s
	}

	bytes := a.Bytes()

	// Pad to 32 bytes (big-endian: most significant byte first)
	if len(bytes) >= 32 {
		// Take the last 32 bytes (most significant)
		copy(s[:], bytes[len(bytes)-32:])
	} else {
		// Pad with leading zeros (big-endian)
		copy(s[32-len(bytes):], bytes)
	}

	// ✅ No byte reversal - keep big-endian format
	return s
}

// ecPointToEncodedBytesBigEndian generates a standard Ed25519 public key in BIG-ENDIAN format (RFC 8032)
// This is the format expected by blockchain nodes and standard Ed25519 verifiers
func ecPointToEncodedBytesBigEndian(x *big.Int, y *big.Int) *[32]byte {
	pubKey := new([32]byte)
	yBytes := y.Bytes()

	// Pad Y coordinate to 32 bytes (big-endian)
	if len(yBytes) >= 32 {
		copy(pubKey[:], yBytes[len(yBytes)-32:])
	} else {
		copy(pubKey[32-len(yBytes):], yBytes)
	}

	// Set the most significant bit to indicate the sign of X (Ed25519 compressed format)
	// For big-endian, we need to check the sign differently
	// We use the internal little-endian format to determine the sign
	xLittleEndian := bigIntToEncodedBytes(x)
	xFE := new(edwards25519.FieldElement)
	edwards25519.FeFromBytes(xFE, xLittleEndian)
	isNegative := edwards25519.FeIsNegative(xFE) == 1

	if isNegative {
		pubKey[31] |= 0x80 // Set MSB in big-endian format
	}

	return pubKey
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

// SignatureToStandardEd25519 converts a tss-lib signature (little-endian) to standard Ed25519 format (big-endian, RFC 8032)
// This is the format expected by blockchain nodes and standard Ed25519 verifiers
//
// Parameters:
//   - signature: 64-byte signature in tss-lib format (R || S, little-endian)
//
// Returns:
//   - 64-byte signature in standard Ed25519 format (R || S, big-endian)
//   - error if signature length is not 64 bytes
func SignatureToStandardEd25519(signature []byte) ([]byte, error) {
	if len(signature) != 64 {
		return nil, fmt.Errorf("signature must be 64 bytes, got %d", len(signature))
	}

	// Extract R and S (each 32 bytes, little-endian)
	var rLE, sLE [32]byte
	copy(rLE[:], signature[:32])
	copy(sLE[:], signature[32:])

	// Convert to big-endian
	rBE := littleEndianToBigEndian(&rLE)
	sBE := littleEndianToBigEndian(&sLE)

	// Concatenate R || S (big-endian)
	result := make([]byte, 64)
	copy(result[:32], rBE[:])
	copy(result[32:], sBE[:])

	return result, nil
}

// PublicKeyToStandardEd25519 converts a tss-lib public key to standard Ed25519 format (big-endian, RFC 8032)
// This is the format expected by blockchain nodes and standard Ed25519 verifiers
//
// Parameters:
//   - x, y: Public key coordinates (big.Int)
//
// Returns:
//   - 32-byte public key in standard Ed25519 format (big-endian)
func PublicKeyToStandardEd25519(x, y *big.Int) [32]byte {
	return *ecPointToEncodedBytesBigEndian(x, y)
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

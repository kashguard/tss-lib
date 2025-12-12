package signing

import (
	"crypto/ed25519"
	"fmt"
	"math/big"

	"github.com/kashguard/tss-lib/common"
)

// DiagnoseSignatureFormat 诊断签名格式问题
func DiagnoseSignatureFormat(
	sigData *common.SignatureData,
	pubKeyX, pubKeyY *big.Int,
	message []byte,
) {
	fmt.Println("\n=== Signature Format Diagnosis ===")

	// 1. 获取 tss-lib 公钥（little-endian）
	tssPubKey := ecPointToEncodedBytes(pubKeyX, pubKeyY)
	fmt.Printf("tss-lib public key (32 bytes): %x\n", tssPubKey[:])
	fmt.Printf("tss-lib signature (64 bytes): %x\n", sigData.Signature)
	fmt.Printf("tss-lib signature R (first 32): %x\n", sigData.Signature[:32])
	fmt.Printf("tss-lib signature S (last 32): %x\n", sigData.Signature[32:])

	// 2. 尝试直接验证
	fmt.Println("\n--- Method 1: Direct verification (no conversion) ---")
	valid1 := ed25519.Verify(ed25519.PublicKey(tssPubKey[:]), message, sigData.Signature)
	fmt.Printf("Result: %v\n", valid1)

	if valid1 {
		fmt.Println("✅ SUCCESS: tss-lib output is already in standard Ed25519 format!")
		return
	}

	// 3. 尝试各种转换方法
	fmt.Println("\n--- Method 2: Reverse entire signature ---")
	reversedSig := make([]byte, 64)
	for i := 0; i < 64; i++ {
		reversedSig[i] = sigData.Signature[63-i]
	}
	valid2 := ed25519.Verify(ed25519.PublicKey(tssPubKey[:]), message, reversedSig)
	fmt.Printf("Result: %v\n", valid2)

	fmt.Println("\n--- Method 3: Reverse R and S separately ---")
	reversedRS := make([]byte, 64)
	for i := 0; i < 32; i++ {
		reversedRS[i] = sigData.Signature[31-i]
		reversedRS[32+i] = sigData.Signature[63-i]
	}
	valid3 := ed25519.Verify(ed25519.PublicKey(tssPubKey[:]), message, reversedRS)
	fmt.Printf("Result: %v\n", valid3)

	fmt.Println("\n--- Method 4: Reverse public key ---")
	reversedPubKey := make([]byte, 32)
	for i := 0; i < 32; i++ {
		reversedPubKey[i] = tssPubKey[31-i]
	}
	valid4 := ed25519.Verify(ed25519.PublicKey(reversedPubKey), message, sigData.Signature)
	fmt.Printf("Result: %v\n", valid4)

	fmt.Println("\n--- Method 5: Reverse both ---")
	valid5 := ed25519.Verify(ed25519.PublicKey(reversedPubKey), message, reversedRS)
	fmt.Printf("Result: %v\n", valid5)

	if !valid1 && !valid2 && !valid3 && !valid4 && !valid5 {
		fmt.Println("\n❌ All methods failed. Possible reasons:")
		fmt.Println("   1. Algorithm-level incompatibility between tss-lib EdDSA and standard Ed25519")
		fmt.Println("   2. Signature generation process differs from standard Ed25519")
		fmt.Println("   3. Public key encoding format differs")
	}
}


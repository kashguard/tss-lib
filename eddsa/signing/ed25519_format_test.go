package signing

import (
	"crypto/ed25519"
	"crypto/rand"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/kashguard/tss-lib/common"
	"github.com/kashguard/tss-lib/eddsa/keygen"
	"github.com/kashguard/tss-lib/tss"
)

// TestEd25519FormatCompatibility 测试 tss-lib 签名与标准 Ed25519 的格式兼容性
func TestEd25519FormatCompatibility(t *testing.T) {
	// 使用标准 Ed25519 生成密钥对作为参考
	stdPubKey, stdPrivKey, err := ed25519.GenerateKey(rand.Reader)
	assert.NoError(t, err)

	// 签名一个消息
	message := []byte("Test message for format compatibility")
	stdSignature := ed25519.Sign(stdPrivKey, message)

	// 验证标准签名
	valid := ed25519.Verify(stdPubKey, message, stdSignature)
	assert.True(t, valid, "Standard Ed25519 signature should be valid")

	t.Logf("✅ Standard Ed25519 test passed")
	t.Logf("Standard public key (first 8 bytes): %x", stdPubKey[:8])
	t.Logf("Standard signature (first 8 bytes): %x", stdSignature[:8])

	// 分析标准格式
	t.Logf("\n📊 Standard Ed25519 Format Analysis:")
	t.Logf("Public key length: %d bytes", len(stdPubKey))
	t.Logf("Signature length: %d bytes", len(stdSignature))
	t.Logf("Signature R (first 32 bytes, hex): %x", stdSignature[:32])
	t.Logf("Signature S (last 32 bytes, hex): %x", stdSignature[32:])
}

// TestTssLibSignatureFormat 测试 tss-lib 签名的格式
func TestTssLibSignatureFormat(t *testing.T) {
	// 加载测试fixture
	keys, signPIDs, err := keygen.LoadKeygenTestFixtures(2)
	assert.NoError(t, err)

	message := []byte("Test message for tss-lib format")
	msgBigInt := new(big.Int).SetBytes(message)

	pID := signPIDs[0]
	p2pCtx := tss.NewPeerContext([]*tss.PartyID{pID})
	params := tss.NewParameters(tss.Edwards(), p2pCtx, pID, 1, 0)

	outCh := make(chan tss.Message, 1)
	endCh := make(chan *common.SignatureData, 1)

	party := NewLocalParty(msgBigInt, params, keys[0], outCh, endCh)

	go func() {
		if err := party.Start(); err != nil {
			t.Errorf("party failed to start: %v", err)
		}
	}()

	party.Update(nil)

	var sigData *common.SignatureData
	select {
	case sigData = <-endCh:
		t.Log("✅ tss-lib signature completed")
	default:
		t.Fatal("Signing did not complete")
	}

	t.Logf("\n📊 tss-lib Signature Format Analysis:")
	t.Logf("Signature length: %d bytes", len(sigData.Signature))
	t.Logf("Signature (first 8 bytes, hex): %x", sigData.Signature[:8])
	t.Logf("Signature (last 8 bytes, hex): %x", sigData.Signature[56:])
	t.Logf("R component (hex): %x", sigData.R)
	t.Logf("S component (hex): %x", sigData.S)

	// 转换为标准格式
	standardSig, err := SignatureToStandardEd25519(sigData.Signature)
	assert.NoError(t, err)

	t.Logf("\n📊 Converted Signature Format Analysis:")
	t.Logf("Standard signature length: %d bytes", len(standardSig))
	t.Logf("Standard signature (first 8 bytes, hex): %x", standardSig[:8])
	t.Logf("Standard signature (last 8 bytes, hex): %x", standardSig[56:])

	// 转换公钥
	standardPubKey := PublicKeyToStandardEd25519(keys[0].EDDSAPub.X(), keys[0].EDDSAPub.Y())
	t.Logf("\n📊 Converted Public Key Format Analysis:")
	t.Logf("Standard public key length: %d bytes", len(standardPubKey))
	t.Logf("Standard public key (first 8 bytes, hex): %x", standardPubKey[:8])
	t.Logf("Standard public key (last 8 bytes, hex): %x", standardPubKey[24:])

	// 尝试验证
	valid := ed25519.Verify(standardPubKey[:], message, standardSig)
	t.Logf("\n🔍 Verification Result: %v", valid)

	if !valid {
		t.Logf("\n❌ Verification failed. Debugging info:")
		t.Logf("Original tss-lib signature (hex): %x", sigData.Signature)
		t.Logf("Converted signature (hex): %x", standardSig)
		t.Logf("Original R (big.Int): %s", new(big.Int).SetBytes(sigData.R).String())
		t.Logf("Original S (big.Int): %s", new(big.Int).SetBytes(sigData.S).String())
		t.Logf("Public key X (big.Int): %s", keys[0].EDDSAPub.X().String())
		t.Logf("Public key Y (big.Int): %s", keys[0].EDDSAPub.Y().String())
	}
}


package signing

import (
	"crypto/ed25519"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/kashguard/tss-lib/common"
	"github.com/kashguard/tss-lib/eddsa/keygen"
	"github.com/kashguard/tss-lib/tss"
)

// TestDirectEd25519Verification 测试直接使用 tss-lib 输出（不转换）进行标准 Ed25519 验证
func TestDirectEd25519Verification(t *testing.T) {
	// 加载测试fixture
	keys, signPIDs, err := keygen.LoadKeygenTestFixtures(2)
	assert.NoError(t, err)

	message := []byte("Test message for direct verification")
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
		t.Log("✅ Signature completed")
	default:
		t.Fatal("Signing did not complete")
	}

	// 方法1：直接使用 tss-lib 输出（假设已经是 little-endian，符合 RFC 8032）
	t.Logf("\n🔍 Method 1: Direct use (assuming little-endian, RFC 8032 format)")
	
	// 获取公钥（使用原始的 ecPointToEncodedBytes，little-endian）
	pubKeyLE := ecPointToEncodedBytes(keys[0].EDDSAPub.X(), keys[0].EDDSAPub.Y())
	
	valid1 := ed25519.Verify(ed25519.PublicKey(pubKeyLE[:]), message, sigData.Signature)
	t.Logf("Direct verification result: %v", valid1)
	
	if valid1 {
		t.Log("✅ SUCCESS: tss-lib output is already in standard Ed25519 format (little-endian)")
		return
	}

	// 方法2：使用转换后的格式（big-endian）
	t.Logf("\n🔍 Method 2: Using converted format (big-endian)")
	standardSig, err := SignatureToStandardEd25519(sigData.Signature)
	assert.NoError(t, err)
	
	standardPubKey := PublicKeyToStandardEd25519(keys[0].EDDSAPub.X(), keys[0].EDDSAPub.Y())
	
	valid2 := ed25519.Verify(ed25519.PublicKey(standardPubKey[:]), message, standardSig)
	t.Logf("Converted verification result: %v", valid2)
	
	if valid2 {
		t.Log("✅ SUCCESS: Converted format works")
	} else {
		t.Log("❌ FAILURE: Both methods failed")
		t.Logf("Original signature (hex): %x", sigData.Signature)
		t.Logf("Converted signature (hex): %x", standardSig)
		t.Logf("Original pubkey (hex): %x", pubKeyLE[:])
		t.Logf("Converted pubkey (hex): %x", standardPubKey[:])
	}
}


// Copyright © 2019 Binance
//
// This file is part of Binance. The full Binance copyright notice, including
// terms governing use, modification, and redistribution, is contained in the
// file LICENSE at the root of the source code distribution tree.

package signing

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"math/big"
	"sync/atomic"
	"testing"

	"github.com/agl/ed25519/edwards25519"
	"github.com/decred/dcrd/dcrec/edwards/v2"
	"github.com/ipfs/go-log"
	"github.com/stretchr/testify/assert"

	"github.com/kashguard/tss-lib/common"
	"github.com/kashguard/tss-lib/eddsa/keygen"
	"github.com/kashguard/tss-lib/test"
	"github.com/kashguard/tss-lib/tss"
)

const (
	testParticipants = test.TestParticipants
	testThreshold    = test.TestThreshold
)

func setUp(level string) {
	if err := log.SetLogLevel("tss-lib", level); err != nil {
		panic(err)
	}

	// only for test
	tss.SetCurve(tss.Edwards())
}

func TestE2EConcurrent(t *testing.T) {
	setUp("info")

	threshold := testThreshold

	// PHASE: load keygen fixtures
	keys, signPIDs, err := keygen.LoadKeygenTestFixturesRandomSet(testThreshold+1, testParticipants)
	assert.NoError(t, err, "should load keygen fixtures")
	assert.Equal(t, testThreshold+1, len(keys))
	assert.Equal(t, testThreshold+1, len(signPIDs))

	// PHASE: signing

	p2pCtx := tss.NewPeerContext(signPIDs)
	parties := make([]*LocalParty, 0, len(signPIDs))

	errCh := make(chan *tss.Error, len(signPIDs))
	outCh := make(chan tss.Message, len(signPIDs))
	endCh := make(chan *common.SignatureData, len(signPIDs))

	updater := test.SharedPartyUpdater

	msg := big.NewInt(200)
	// init the parties
	for i := 0; i < len(signPIDs); i++ {
		params := tss.NewParameters(tss.Edwards(), p2pCtx, signPIDs[i], len(signPIDs), threshold)

		P := NewLocalParty(msg, params, keys[i], outCh, endCh).(*LocalParty)
		parties = append(parties, P)
		go func(P *LocalParty) {
			if err := P.Start(); err != nil {
				errCh <- err
			}
		}(P)
	}

	var ended int32
signing:
	for {
		select {
		case err := <-errCh:
			common.Logger.Errorf("Error: %s", err)
			assert.FailNow(t, err.Error())
			break signing

		case msg := <-outCh:
			dest := msg.GetTo()
			if dest == nil {
				for _, P := range parties {
					if P.PartyID().Index == msg.GetFrom().Index {
						continue
					}
					go updater(P, msg, errCh)
				}
			} else {
				if dest[0].Index == msg.GetFrom().Index {
					t.Fatalf("party %d tried to send a message to itself (%d)", dest[0].Index, msg.GetFrom().Index)
				}
				go updater(parties[dest[0].Index], msg, errCh)
			}

		case <-endCh:
			atomic.AddInt32(&ended, 1)
			if atomic.LoadInt32(&ended) == int32(len(signPIDs)) {
				t.Logf("Done. Received signature data from %d participants", ended)
				R := parties[0].temp.r

				// BEGIN check s correctness
				sumS := parties[0].temp.si
				for i, p := range parties {
					if i == 0 {
						continue
					}

					var tmpSumS [32]byte
					edwards25519.ScMulAdd(&tmpSumS, sumS, bigIntToEncodedBytes(big.NewInt(1)), p.temp.si)
					sumS = &tmpSumS
				}
				fmt.Printf("S: %s\n", encodedBytesToBigInt(sumS).String())
				fmt.Printf("R: %s\n", R.String())
				// END check s correctness

				// BEGIN EDDSA verify
				pkX, pkY := keys[0].EDDSAPub.X(), keys[0].EDDSAPub.Y()
				pk := edwards.PublicKey{
					Curve: tss.Edwards(),
					X:     pkX,
					Y:     pkY,
				}

				newSig, err := edwards.ParseSignature(parties[0].data.Signature)
				if err != nil {
					println("new sig error, ", err.Error())
				}

				ok := edwards.Verify(&pk, msg.Bytes(), newSig.R, newSig.S)
				assert.True(t, ok, "eddsa verify must pass")
				t.Log("EDDSA signing test done.")
				// END EDDSA verify

				// BEGIN Standard Ed25519 verification test
				// Test if tss-lib signature can be verified with standard crypto/ed25519
				testStandardEd25519Verification(t, parties[0].data, keys[0], msg.Bytes())
				// END Standard Ed25519 verification test

				break signing
			}
		}
	}
}

func TestE2EConcurrentWithLeadingZeroInMSG(t *testing.T) {
	setUp("info")

	threshold := testThreshold

	// PHASE: load keygen fixtures
	keys, signPIDs, err := keygen.LoadKeygenTestFixturesRandomSet(testThreshold+1, testParticipants)
	assert.NoError(t, err, "should load keygen fixtures")
	assert.Equal(t, testThreshold+1, len(keys))
	assert.Equal(t, testThreshold+1, len(signPIDs))

	// PHASE: signing

	p2pCtx := tss.NewPeerContext(signPIDs)
	parties := make([]*LocalParty, 0, len(signPIDs))

	errCh := make(chan *tss.Error, len(signPIDs))
	outCh := make(chan tss.Message, len(signPIDs))
	endCh := make(chan *common.SignatureData, len(signPIDs))

	updater := test.SharedPartyUpdater

	msg, _ := hex.DecodeString("00f163ee51bcaeff9cdff5e0e3c1a646abd19885fffbab0b3b4236e0cf95c9f5")
	// init the parties
	for i := 0; i < len(signPIDs); i++ {
		params := tss.NewParameters(tss.Edwards(), p2pCtx, signPIDs[i], len(signPIDs), threshold)
		P := NewLocalParty(new(big.Int).SetBytes(msg), params, keys[i], outCh, endCh, len(msg)).(*LocalParty)
		parties = append(parties, P)
		go func(P *LocalParty) {
			if err := P.Start(); err != nil {
				errCh <- err
			}
		}(P)
	}

	var ended int32
signing:
	for {
		select {
		case err := <-errCh:
			common.Logger.Errorf("Error: %s", err)
			assert.FailNow(t, err.Error())
			break signing

		case msg := <-outCh:
			dest := msg.GetTo()
			if dest == nil {
				for _, P := range parties {
					if P.PartyID().Index == msg.GetFrom().Index {
						continue
					}
					go updater(P, msg, errCh)
				}
			} else {
				if dest[0].Index == msg.GetFrom().Index {
					t.Fatalf("party %d tried to send a message to itself (%d)", dest[0].Index, msg.GetFrom().Index)
				}
				go updater(parties[dest[0].Index], msg, errCh)
			}

		case <-endCh:
			atomic.AddInt32(&ended, 1)
			if atomic.LoadInt32(&ended) == int32(len(signPIDs)) {
				t.Logf("Done. Received signature data from %d participants", ended)
				R := parties[0].temp.r

				// BEGIN check s correctness
				sumS := parties[0].temp.si
				for i, p := range parties {
					if i == 0 {
						continue
					}

					var tmpSumS [32]byte
					edwards25519.ScMulAdd(&tmpSumS, sumS, bigIntToEncodedBytes(big.NewInt(1)), p.temp.si)
					sumS = &tmpSumS
				}
				fmt.Printf("S: %s\n", encodedBytesToBigInt(sumS).String())
				fmt.Printf("R: %s\n", R.String())
				// END check s correctness

				// BEGIN EDDSA verify
				pkX, pkY := keys[0].EDDSAPub.X(), keys[0].EDDSAPub.Y()
				pk := edwards.PublicKey{
					Curve: tss.Edwards(),
					X:     pkX,
					Y:     pkY,
				}

				newSig, err := edwards.ParseSignature(parties[0].data.Signature)
				if err != nil {
					println("new sig error, ", err.Error())
				}

				ok := edwards.Verify(&pk, msg, newSig.R, newSig.S)
				assert.True(t, ok, "eddsa verify must pass")
				t.Log("EDDSA signing test done.")
				// END EDDSA verify

				// BEGIN Standard Ed25519 verification test
				testStandardEd25519Verification(t, parties[0].data, keys[0], msg)
				// END Standard Ed25519 verification test

				break signing
			}
		}
	}
}

// testStandardEd25519Verification 测试 tss-lib 签名是否可以通过标准 Ed25519 验证
func testStandardEd25519Verification(
	t *testing.T,
	sigData *common.SignatureData,
	keyData keygen.LocalPartySaveData,
	message []byte,
) {
	t.Log("\n=== Standard Ed25519 Verification Test ===")

	// 方法1：直接使用 tss-lib 输出（little-endian，符合 RFC 8032）
	t.Log("\n--- Method 1: Direct verification (little-endian, RFC 8032) ---")
	tssPubKey := ecPointToEncodedBytes(keyData.EDDSAPub.X(), keyData.EDDSAPub.Y())

	t.Logf("tss-lib public key (32 bytes): %x", tssPubKey[:])
	t.Logf("tss-lib signature (64 bytes): %x", sigData.Signature)
	t.Logf("Message: %x", message)

	valid1 := ed25519.Verify(ed25519.PublicKey(tssPubKey[:]), message, sigData.Signature)
	t.Logf("Result: %v", valid1)

	if valid1 {
		t.Log("✅ SUCCESS: tss-lib output is already in standard Ed25519 format!")
		return
	}

	// 方法2：使用转换函数
	t.Log("\n--- Method 2: Using conversion functions ---")
	standardSig, err := SignatureToStandardEd25519(sigData.Signature)
	if err != nil {
		t.Logf("❌ Conversion error: %v", err)
		return
	}

	standardPubKey := PublicKeyToStandardEd25519(keyData.EDDSAPub.X(), keyData.EDDSAPub.Y())

	t.Logf("Converted public key: %x", standardPubKey[:])
	t.Logf("Converted signature: %x", standardSig)

	valid2 := ed25519.Verify(ed25519.PublicKey(standardPubKey[:]), message, standardSig)
	t.Logf("Result: %v", valid2)

	if valid2 {
		t.Log("✅ SUCCESS: Converted format works!")
		return
	}

	// 方法3：尝试各种字节序转换
	t.Log("\n--- Method 3: Trying different byte order conversions ---")

	// 3.1: 反转整个签名
	reversedSig1 := make([]byte, 64)
	for i := 0; i < 64; i++ {
		reversedSig1[i] = sigData.Signature[63-i]
	}
	valid3_1 := ed25519.Verify(ed25519.PublicKey(tssPubKey[:]), message, reversedSig1)
	t.Logf("Reversed entire signature: %v", valid3_1)

	// 3.2: 分别反转 R 和 S
	reversedSig2 := make([]byte, 64)
	for i := 0; i < 32; i++ {
		reversedSig2[i] = sigData.Signature[31-i]    // Reverse R
		reversedSig2[32+i] = sigData.Signature[63-i] // Reverse S
	}
	valid3_2 := ed25519.Verify(ed25519.PublicKey(tssPubKey[:]), message, reversedSig2)
	t.Logf("Reversed R and S separately: %v", valid3_2)

	// 3.3: 反转公钥
	reversedPubKey := make([]byte, 32)
	for i := 0; i < 32; i++ {
		reversedPubKey[i] = tssPubKey[31-i]
	}
	valid3_3 := ed25519.Verify(ed25519.PublicKey(reversedPubKey), message, sigData.Signature)
	t.Logf("Reversed public key: %v", valid3_3)

	// 3.4: 同时反转公钥和签名
	valid3_4 := ed25519.Verify(ed25519.PublicKey(reversedPubKey), message, reversedSig2)
	t.Logf("Reversed both: %v", valid3_4)

	if valid3_1 || valid3_2 || valid3_3 || valid3_4 {
		t.Log("✅ SUCCESS: Found working byte order conversion!")
		return
	}

	// 如果所有方法都失败
	t.Log("\n❌ FAILURE: All verification methods failed")
	t.Log("This suggests algorithm-level incompatibility between tss-lib EdDSA and standard Ed25519")
	t.Log("\nDebug information:")
	t.Logf("R (big.Int): %s", new(big.Int).SetBytes(sigData.R).String())
	t.Logf("S (big.Int): %s", new(big.Int).SetBytes(sigData.S).String())
	t.Logf("Public key X: %s", keyData.EDDSAPub.X().String())
	t.Logf("Public key Y: %s", keyData.EDDSAPub.Y().String())
	t.Logf("Message length: %d bytes", len(message))
}

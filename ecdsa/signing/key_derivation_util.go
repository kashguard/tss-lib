// Copyright © 2021 Swingby

// ⚠️ SECURITY WARNING: This file uses btcd/chaincfg for Bitcoin network parameters.
// The btcd dependency is isolated via go.mod replace directives to prevent conflicts
// with newer versions, but this may expose the system to known vulnerabilities in older
// btcd versions.
//
// Risk mitigation:
// - btcd usage here is limited to network parameters only (MainNetParams)
// - No network or transaction functionality is used
// - Dependency isolation prevents conflicts with newer btcd versions
//
// Future considerations:
// - Monitor for updated tss-lib forks with newer btcd dependencies
// - Consider replacing btcd/chaincfg usage with standalone alternatives

package signing

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"math/big"

	"github.com/kashguard/tss-lib/common"
	"github.com/kashguard/tss-lib/crypto"
	"github.com/kashguard/tss-lib/crypto/ckd"
	"github.com/kashguard/tss-lib/ecdsa/keygen"

	"github.com/btcsuite/btcd/chaincfg"
)

func UpdatePublicKeyAndAdjustBigXj(keyDerivationDelta *big.Int, keys []keygen.LocalPartySaveData, extendedChildPk *ecdsa.PublicKey, ec elliptic.Curve) error {
	var err error
	gDelta := crypto.ScalarBaseMult(ec, keyDerivationDelta)
	for k := range keys {
		keys[k].ECDSAPub, err = crypto.NewECPoint(ec, extendedChildPk.X, extendedChildPk.Y)
		if err != nil {
			common.Logger.Errorf("error creating new extended child public key")
			return err
		}
		// Suppose X_j has shamir shares X_j0,     X_j1,     ..., X_jn
		// So X_j + D has shamir shares  X_j0 + D, X_j1 + D, ..., X_jn + D
		for j := range keys[k].BigXj {
			keys[k].BigXj[j], err = keys[k].BigXj[j].Add(gDelta)
			if err != nil {
				common.Logger.Errorf("error in delta operation")
				return err
			}
		}
	}
	return nil
}

func derivingPubkeyFromPath(masterPub *crypto.ECPoint, chainCode []byte, path []uint32, ec elliptic.Curve) (*big.Int, *ckd.ExtendedKey, error) {
	// build ecdsa key pair
	pk := ecdsa.PublicKey{
		Curve: ec,
		X:     masterPub.X(),
		Y:     masterPub.Y(),
	}

	net := &chaincfg.MainNetParams
	extendedParentPk := &ckd.ExtendedKey{
		PublicKey:  pk,
		Depth:      0,
		ChildIndex: 0,
		ChainCode:  chainCode[:],
		ParentFP:   []byte{0x00, 0x00, 0x00, 0x00},
		Version:    net.HDPrivateKeyID[:],
	}

	return ckd.DeriveChildKeyFromHierarchy(path, extendedParentPk, ec.Params().N, ec)
}

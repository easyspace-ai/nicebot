package clob

import (
	"crypto/ecdsa"
	"encoding/hex"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

type Signer struct {
	privateKey *ecdsa.PrivateKey
	chainID    int64
	address    common.Address
}

func NewSigner(hexKey string, chainID int64) (*Signer, error) {
	key := strings.TrimSpace(hexKey)
	key = strings.TrimPrefix(key, "0x")
	pk, err := crypto.HexToECDSA(key)
	if err != nil {
		return nil, err
	}
	addr := crypto.PubkeyToAddress(pk.PublicKey)
	return &Signer{privateKey: pk, chainID: chainID, address: addr}, nil
}

func (s *Signer) Address() common.Address { return s.address }
func (s *Signer) ChainID() int64          { return s.chainID }

// SignHash signs a 32-byte hash using raw secp256k1 (no prefix), returning 0x-prefixed hex.
// Note: eth_account.Account._sign_hash returns a signature whose V is 27/28; we match that.
func (s *Signer) SignHash(hash [32]byte) (string, error) {
	sig, err := crypto.Sign(hash[:], s.privateKey)
	if err != nil {
		return "", err
	}
	// convert v 0/1 -> 27/28
	sig[64] = sig[64] + 27
	return "0x" + hex.EncodeToString(sig), nil
}

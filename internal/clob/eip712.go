package clob

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
)

const (
	clobDomainName = "ClobAuthDomain"
	clobVersion    = "1"
	clobAuthMsg    = "This message attests that I control the given wallet"
)

func SignClobAuthMessage(signer *Signer, timestamp int64, nonce int64) (string, error) {
	td := apitypes.TypedData{
		Types: apitypes.Types{
			"EIP712Domain": []apitypes.Type{
				{Name: "name", Type: "string"},
				{Name: "version", Type: "string"},
				{Name: "chainId", Type: "uint256"},
			},
			"ClobAuth": []apitypes.Type{
				{Name: "address", Type: "address"},
				{Name: "timestamp", Type: "string"},
				{Name: "nonce", Type: "uint256"},
				{Name: "message", Type: "string"},
			},
		},
		PrimaryType: "ClobAuth",
		Domain: apitypes.TypedDataDomain{
			Name:    clobDomainName,
			Version: clobVersion,
			ChainId: math.NewHexOrDecimal256(signer.ChainID()),
		},
		Message: apitypes.TypedDataMessage{
			"address":   signer.Address().Hex(),
			"timestamp": fmt.Sprintf("%d", timestamp),
			"nonce":     math.NewHexOrDecimal256(nonce),
			"message":   clobAuthMsg,
		},
	}

	digest, err := typedDataDigest(td)
	if err != nil {
		return "", err
	}
	var h32 [32]byte
	copy(h32[:], digest.Bytes())
	return signer.SignHash(h32)
}

func typedDataDigest(td apitypes.TypedData) (common.Hash, error) {
	// This signature helper intentionally returns the exact 32-byte digest used by eth_account._sign_hash:
	// keccak256("\x19\x01" || domainSeparator || structHash(message))
	domainSeparator, err := td.HashStruct("EIP712Domain", td.Domain.Map())
	if err != nil {
		return common.Hash{}, err
	}
	msgHash, err := td.HashStruct(td.PrimaryType, td.Message)
	if err != nil {
		return common.Hash{}, err
	}

	prefix := []byte{0x19, 0x01}
	return crypto.Keccak256Hash(prefix, domainSeparator[:], msgHash[:]), nil
}

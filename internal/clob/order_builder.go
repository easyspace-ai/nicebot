package clob

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/ethereum/go-ethereum/common"
	ethmath "github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
)

const (
	OrderSideBuy  = "BUY"
	OrderSideSell = "SELL"
)

type OrderType string

const (
	OrderTypeGTC OrderType = "GTC"
	OrderTypeFOK OrderType = "FOK"
	OrderTypeGTD OrderType = "GTD"
	OrderTypeFAK OrderType = "FAK"
)

type ApiCreds struct {
	APIKey        string
	APISecret     string
	APIPassphrase string
}

type OrderArgs struct {
	TokenID    string
	Price      float64
	Size       float64
	Side       string
	FeeRateBps int
	Nonce      int64
	Expiration int64
	Taker      string
}

type SignedOrderJSON struct {
	Salt          uint64 `json:"salt"`
	Maker         string `json:"maker"`
	Signer        string `json:"signer"`
	Taker         string `json:"taker"`
	TokenID       string `json:"tokenId"`
	MakerAmount   string `json:"makerAmount"`
	TakerAmount   string `json:"takerAmount"`
	Expiration    string `json:"expiration"`
	Nonce         string `json:"nonce"`
	FeeRateBps    string `json:"feeRateBps"`
	Side          string `json:"side"`
	SignatureType int    `json:"signatureType"`
	Signature     string `json:"signature"`
}

type postOrderBody struct {
	Order     SignedOrderJSON `json:"order"`
	Owner     string          `json:"owner"`
	OrderType OrderType       `json:"orderType"`
}

type TickSize string

type roundConfig struct {
	price  int
	size   int
	amount int
}

var roundingConfig = map[TickSize]roundConfig{
	"0.1":    {price: 1, size: 2, amount: 3},
	"0.01":   {price: 2, size: 2, amount: 4},
	"0.001":  {price: 3, size: 2, amount: 5},
	"0.0001": {price: 4, size: 2, amount: 6},
}

// toTokenDecimals replicates py_order_utils.order_builder.helpers.to_token_decimals (1e6 scale).
func toTokenDecimals(x float64) uint64 {
	f := 1e6 * x
	if decimalPlaces(f) > 0 {
		f = roundNormal(f, 0)
	}
	if f < 0 {
		return 0
	}
	return uint64(f)
}

func decimalPlaces(x float64) int {
	// Best-effort; mirrors Decimal exponent usage. We only use this for small rounding checks.
	s := fmt.Sprintf("%.12f", x)
	// trim trailing zeros
	i := len(s) - 1
	for i >= 0 && s[i] == '0' {
		i--
	}
	if i >= 0 && s[i] == '.' {
		return 0
	}
	// count decimals after dot
	for j := 0; j < len(s); j++ {
		if s[j] == '.' {
			return i - j
		}
	}
	return 0
}

func roundDown(x float64, sigDigits int) float64 {
	p := math.Pow10(sigDigits)
	return math.Floor(x*p) / p
}

func roundUp(x float64, sigDigits int) float64 {
	p := math.Pow10(sigDigits)
	return math.Ceil(x*p) / p
}

func roundNormal(x float64, sigDigits int) float64 {
	p := math.Pow10(sigDigits)
	return math.Round(x*p) / p
}

func generateSalt32() uint64 {
	var b [8]byte
	if _, err := rand.Read(b[:]); err == nil {
		return binary.LittleEndian.Uint64(b[:]) & 0xffffffff
	}
	return uint64(time.Now().UnixNano()) & 0xffffffff
}

func priceValid(price float64, tick TickSize) bool {
	t, err := parseTick(tick)
	if err != nil {
		return false
	}
	return price >= t && price <= 1.0-t
}

func parseTick(t TickSize) (float64, error) {
	switch t {
	case "0.1":
		return 0.1, nil
	case "0.01":
		return 0.01, nil
	case "0.001":
		return 0.001, nil
	case "0.0001":
		return 0.0001, nil
	default:
		return 0, fmt.Errorf("unknown tick size: %s", t)
	}
}

func buildOrderAmounts(side string, size float64, price float64, rc roundConfig) (sideInt int, makerAmt uint64, takerAmt uint64, err error) {
	rawPrice := roundNormal(price, rc.price)

	switch side {
	case OrderSideBuy:
		rawTaker := roundDown(size, rc.size)
		rawMaker := rawTaker * rawPrice
		if decimalPlaces(rawMaker) > rc.amount {
			rawMaker = roundUp(rawMaker, rc.amount+4)
			if decimalPlaces(rawMaker) > rc.amount {
				rawMaker = roundDown(rawMaker, rc.amount)
			}
		}
		return 0, toTokenDecimals(rawMaker), toTokenDecimals(rawTaker), nil
	case OrderSideSell:
		rawMaker := roundDown(size, rc.size)
		rawTaker := rawMaker * rawPrice
		if decimalPlaces(rawTaker) > rc.amount {
			rawTaker = roundUp(rawTaker, rc.amount+4)
			if decimalPlaces(rawTaker) > rc.amount {
				rawTaker = roundDown(rawTaker, rc.amount)
			}
		}
		return 1, toTokenDecimals(rawMaker), toTokenDecimals(rawTaker), nil
	default:
		return 0, 0, 0, fmt.Errorf("order_args.side must be 'BUY' or 'SELL'")
	}
}

func SignExchangeOrder(
	signer *Signer,
	exchangeAddr common.Address,
	chainID int64,
	order OrderForSigning,
) (string, error) {
	td := apitypes.TypedData{
		Types: apitypes.Types{
			"EIP712Domain": []apitypes.Type{
				{Name: "name", Type: "string"},
				{Name: "version", Type: "string"},
				{Name: "chainId", Type: "uint256"},
				{Name: "verifyingContract", Type: "address"},
			},
			"Order": []apitypes.Type{
				{Name: "salt", Type: "uint256"},
				{Name: "maker", Type: "address"},
				{Name: "signer", Type: "address"},
				{Name: "taker", Type: "address"},
				{Name: "tokenId", Type: "uint256"},
				{Name: "makerAmount", Type: "uint256"},
				{Name: "takerAmount", Type: "uint256"},
				{Name: "expiration", Type: "uint256"},
				{Name: "nonce", Type: "uint256"},
				{Name: "feeRateBps", Type: "uint256"},
				{Name: "side", Type: "uint8"},
				{Name: "signatureType", Type: "uint8"},
			},
		},
		PrimaryType: "Order",
		Domain: apitypes.TypedDataDomain{
			Name:              "Polymarket CTF Exchange",
			Version:           "1",
			ChainId:           ethmath.NewHexOrDecimal256(chainID),
			VerifyingContract: exchangeAddr.Hex(),
		},
		Message: apitypes.TypedDataMessage{
			"salt":          ethmath.NewHexOrDecimal256(int64(order.Salt)),
			"maker":         order.Maker.Hex(),
			"signer":        order.Signer.Hex(),
			"taker":         order.Taker.Hex(),
			"tokenId":       order.TokenID, // big ints can be provided as strings
			"makerAmount":   order.MakerAmount,
			"takerAmount":   order.TakerAmount,
			"expiration":    order.Expiration,
			"nonce":         order.Nonce,
			"feeRateBps":    order.FeeRateBps,
			"side":          order.Side,
			"signatureType": order.SignatureType,
		},
	}

	domainSeparator, err := td.HashStruct("EIP712Domain", td.Domain.Map())
	if err != nil {
		return "", err
	}
	msgHash, err := td.HashStruct(td.PrimaryType, td.Message)
	if err != nil {
		return "", err
	}
	digest := crypto.Keccak256Hash([]byte{0x19, 0x01}, domainSeparator[:], msgHash[:])
	var h32 [32]byte
	copy(h32[:], digest.Bytes())
	return signer.SignHash(h32)
}

type OrderForSigning struct {
	Salt          uint64
	Maker         common.Address
	Signer        common.Address
	Taker         common.Address
	TokenID       string
	MakerAmount   string
	TakerAmount   string
	Expiration    string
	Nonce         string
	FeeRateBps    string
	Side          int
	SignatureType int
}

func BuildPostOrderBodyJSON(order SignedOrderJSON, owner string, orderType OrderType) ([]byte, error) {
	body := postOrderBody{Order: order, Owner: owner, OrderType: orderType}
	// Must be compact and deterministic.
	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return b, nil
}

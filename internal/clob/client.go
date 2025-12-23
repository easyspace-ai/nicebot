package clob

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

type Client struct {
	host   string
	chain  int64
	signer *Signer
	creds  *ApiCreds
	http   httpClient

	// local caches
	tickSizes map[string]TickSize
	negRisk   map[string]bool
	feeRates  map[string]int

	// signature config
	sigType int
	funder  common.Address
}

func NewClient(host string, chainID int64, privateKey string, signatureType string, funder string) (*Client, error) {
	h := strings.TrimSuffix(host, "/")
	var s *Signer
	var err error
	if privateKey != "" {
		s, err = NewSigner(privateKey, chainID)
		if err != nil {
			return nil, err
		}
	}

	c := &Client{
		host:      h,
		chain:     chainID,
		signer:    s,
		http:      defaultHTTPClient(),
		tickSizes: map[string]TickSize{},
		negRisk:   map[string]bool{},
		feeRates:  map[string]int{},
	}

	c.sigType = 0
	switch strings.ToUpper(strings.TrimSpace(signatureType)) {
	case "POLY_PROXY":
		c.sigType = 1
	case "POLY_GNOSIS_SAFE":
		c.sigType = 2
	default:
		c.sigType = 0
	}
	if funder != "" {
		c.funder = common.HexToAddress(funder)
	} else if c.signer != nil {
		c.funder = c.signer.Address()
	}
	return c, nil
}

func (c *Client) Address() string {
	if c.signer == nil {
		return ""
	}
	return c.signer.Address().Hex()
}

func (c *Client) SetCreds(creds ApiCreds) {
	c.creds = &creds
}

func (c *Client) CreateOrDeriveAPICreds(ctx context.Context, nonce int64) (ApiCreds, error) {
	// Try create, fallback derive (matching python create_or_derive_api_creds)
	creds, err := c.CreateAPIKey(ctx, nonce)
	if err == nil {
		return creds, nil
	}
	return c.DeriveAPIKey(ctx, nonce)
}

func (c *Client) CreateAPIKey(ctx context.Context, nonce int64) (ApiCreds, error) {
	if c.signer == nil {
		return ApiCreds{}, ErrAuthUnavailableL1
	}
	headers, err := c.level1Headers(nonce)
	if err != nil {
		return ApiCreds{}, err
	}
	resp, err := doJSON(ctx, c.http, http.MethodPost, c.host+EndpointCreateAPIKey, headers, nil)
	if err != nil {
		return ApiCreds{}, err
	}
	m, ok := resp.(map[string]any)
	if !ok {
		return ApiCreds{}, fmt.Errorf("unexpected create_api_key response: %T", resp)
	}
	return ApiCreds{
		APIKey:        asString(m["apiKey"]),
		APISecret:     asString(m["secret"]),
		APIPassphrase: asString(m["passphrase"]),
	}, nil
}

func (c *Client) DeriveAPIKey(ctx context.Context, nonce int64) (ApiCreds, error) {
	if c.signer == nil {
		return ApiCreds{}, ErrAuthUnavailableL1
	}
	headers, err := c.level1Headers(nonce)
	if err != nil {
		return ApiCreds{}, err
	}
	resp, err := doJSON(ctx, c.http, http.MethodGet, c.host+EndpointDeriveAPIKey, headers, nil)
	if err != nil {
		return ApiCreds{}, err
	}
	m, ok := resp.(map[string]any)
	if !ok {
		return ApiCreds{}, fmt.Errorf("unexpected derive_api_key response: %T", resp)
	}
	return ApiCreds{
		APIKey:        asString(m["apiKey"]),
		APISecret:     asString(m["secret"]),
		APIPassphrase: asString(m["passphrase"]),
	}, nil
}

func (c *Client) GetOrderBook(ctx context.Context, tokenID string) (map[string]any, error) {
	u := c.host + EndpointGetOrderBook + "?token_id=" + url.QueryEscape(tokenID)
	resp, err := doJSON(ctx, c.http, http.MethodGet, u, nil, nil)
	if err != nil {
		return nil, err
	}
	m, ok := resp.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected orderbook response: %T", resp)
	}
	return m, nil
}

func (c *Client) GetTickSize(ctx context.Context, tokenID string) (TickSize, error) {
	if t, ok := c.tickSizes[tokenID]; ok {
		return t, nil
	}
	u := c.host + EndpointGetTickSize + "?token_id=" + url.QueryEscape(tokenID)
	resp, err := doJSON(ctx, c.http, http.MethodGet, u, nil, nil)
	if err != nil {
		return "", err
	}
	m := resp.(map[string]any)
	ts := TickSize(fmt.Sprintf("%v", m["minimum_tick_size"]))
	c.tickSizes[tokenID] = ts
	return ts, nil
}

func (c *Client) GetNegRisk(ctx context.Context, tokenID string) (bool, error) {
	if v, ok := c.negRisk[tokenID]; ok {
		return v, nil
	}
	u := c.host + EndpointGetNegRisk + "?token_id=" + url.QueryEscape(tokenID)
	resp, err := doJSON(ctx, c.http, http.MethodGet, u, nil, nil)
	if err != nil {
		return false, err
	}
	m := resp.(map[string]any)
	v := asBool(m["neg_risk"])
	c.negRisk[tokenID] = v
	return v, nil
}

func (c *Client) GetFeeRateBps(ctx context.Context, tokenID string) (int, error) {
	if v, ok := c.feeRates[tokenID]; ok {
		return v, nil
	}
	u := c.host + EndpointGetFeeRate + "?token_id=" + url.QueryEscape(tokenID)
	resp, err := doJSON(ctx, c.http, http.MethodGet, u, nil, nil)
	if err != nil {
		return 0, err
	}
	m := resp.(map[string]any)
	fee := asInt(m["base_fee"])
	c.feeRates[tokenID] = fee
	return fee, nil
}

func (c *Client) CreateOrder(ctx context.Context, args OrderArgs, tickSize *TickSize, negRiskOverride *bool) (SignedOrderJSON, bool, error) {
	if c.signer == nil {
		return SignedOrderJSON{}, false, ErrAuthUnavailableL1
	}
	// tick size
	ts := TickSize("0.01")
	var err error
	if tickSize != nil {
		ts = *tickSize
	} else {
		ts, err = c.GetTickSize(ctx, args.TokenID)
		if err != nil {
			return SignedOrderJSON{}, false, err
		}
	}
	if !priceValid(args.Price, ts) {
		return SignedOrderJSON{}, false, fmt.Errorf("price (%v), min: %s - max: %v", args.Price, ts, 1-floatFromTick(ts))
	}
	negRisk := false
	if negRiskOverride != nil {
		negRisk = *negRiskOverride
	} else {
		negRisk, err = c.GetNegRisk(ctx, args.TokenID)
		if err != nil {
			return SignedOrderJSON{}, false, err
		}
	}

	feeRate := 0
	feeRate, err = c.GetFeeRateBps(ctx, args.TokenID)
	if err != nil {
		return SignedOrderJSON{}, false, err
	}
	if args.FeeRateBps > 0 && feeRate > 0 && args.FeeRateBps != feeRate {
		return SignedOrderJSON{}, false, fmt.Errorf("invalid user provided fee rate: (%d), fee rate for the market must be %d", args.FeeRateBps, feeRate)
	}
	args.FeeRateBps = feeRate

	rc, ok := roundingConfig[ts]
	if !ok {
		return SignedOrderJSON{}, false, fmt.Errorf("unsupported tick size: %s", ts)
	}

	sideInt, makerAmt, takerAmt, err := buildOrderAmounts(args.Side, args.Size, args.Price, rc)
	if err != nil {
		return SignedOrderJSON{}, false, err
	}

	// maker/taker semantics match py_order_utils: BUY -> makerAmount=USDC, takerAmount=shares
	// SELL -> makerAmount=shares, takerAmount=USDC
	maker := c.funder
	orderSigner := c.signer.Address()
	taker := common.HexToAddress("0x0000000000000000000000000000000000000000")
	if args.Taker != "" {
		taker = common.HexToAddress(args.Taker)
	}

	salt := generateSalt32()
	expiration := args.Expiration
	if expiration < 0 {
		expiration = 0
	}
	nonce := args.Nonce
	if nonce < 0 {
		nonce = 0
	}

	ofs := OrderForSigning{
		Salt:          salt,
		Maker:         maker,
		Signer:        orderSigner,
		Taker:         taker,
		TokenID:       args.TokenID,
		MakerAmount:   fmt.Sprintf("%d", makerAmt),
		TakerAmount:   fmt.Sprintf("%d", takerAmt),
		Expiration:    fmt.Sprintf("%d", expiration),
		Nonce:         fmt.Sprintf("%d", nonce),
		FeeRateBps:    fmt.Sprintf("%d", args.FeeRateBps),
		Side:          sideInt,
		SignatureType: c.sigType,
	}

	contractCfg, err := GetContractConfig(c.chain, negRisk)
	if err != nil {
		return SignedOrderJSON{}, negRisk, err
	}

	sig, err := SignExchangeOrder(c.signer, common.HexToAddress(contractCfg.Exchange), c.chain, ofs)
	if err != nil {
		return SignedOrderJSON{}, negRisk, err
	}

	// json payload structure must mirror py_order_utils SignedOrder.dict
	sideStr := OrderSideBuy
	if sideInt == 1 {
		sideStr = OrderSideSell
	}

	return SignedOrderJSON{
		Salt:          salt,
		Maker:         maker.Hex(),
		Signer:        orderSigner.Hex(),
		Taker:         taker.Hex(),
		TokenID:       args.TokenID,
		MakerAmount:   fmt.Sprintf("%d", makerAmt),
		TakerAmount:   fmt.Sprintf("%d", takerAmt),
		Expiration:    fmt.Sprintf("%d", expiration),
		Nonce:         fmt.Sprintf("%d", nonce),
		FeeRateBps:    fmt.Sprintf("%d", args.FeeRateBps),
		Side:          sideStr,
		SignatureType: c.sigType,
		Signature:     sig,
	}, negRisk, nil
}

func (c *Client) PostOrder(ctx context.Context, order SignedOrderJSON, orderType OrderType) (map[string]any, error) {
	if c.signer == nil {
		return nil, ErrAuthUnavailableL1
	}
	if c.creds == nil {
		return nil, ErrAuthUnavailableL2
	}
	bodyBytes, err := BuildPostOrderBodyJSON(order, c.creds.APIKey, orderType)
	if err != nil {
		return nil, err
	}

	headers, err := c.level2Headers(http.MethodPost, EndpointPostOrder, bodyBytes)
	if err != nil {
		return nil, err
	}
	resp, err := doJSON(ctx, c.http, http.MethodPost, c.host+EndpointPostOrder, headers, bodyBytes)
	if err != nil {
		return nil, err
	}
	m, ok := resp.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected post_order response: %T", resp)
	}
	return m, nil
}

func (c *Client) GetOrder(ctx context.Context, orderID string) (map[string]any, error) {
	if c.signer == nil {
		return nil, ErrAuthUnavailableL1
	}
	if c.creds == nil {
		return nil, ErrAuthUnavailableL2
	}
	path := EndpointGetOrderPrefix + orderID
	headers, err := c.level2Headers(http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	resp, err := doJSON(ctx, c.http, http.MethodGet, c.host+path, headers, nil)
	if err != nil {
		return nil, err
	}
	m, ok := resp.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected get_order response: %T", resp)
	}
	return m, nil
}

func (c *Client) Cancel(ctx context.Context, orderID string) (any, error) {
	if c.signer == nil {
		return nil, ErrAuthUnavailableL1
	}
	if c.creds == nil {
		return nil, ErrAuthUnavailableL2
	}
	// body: {"orderID": "..."} exactly, compact JSON
	body := struct {
		OrderID string `json:"orderID"`
	}{OrderID: orderID}
	b, _ := json.Marshal(body)
	headers, err := c.level2Headers(http.MethodDelete, EndpointCancel, b)
	if err != nil {
		return nil, err
	}
	return doJSON(ctx, c.http, http.MethodDelete, c.host+EndpointCancel, headers, b)
}

type BalanceAllowanceParams struct {
	AssetType      string
	TokenID        string
	SignatureType  int
}

func (c *Client) GetBalanceAllowance(ctx context.Context, params *BalanceAllowanceParams) (map[string]any, error) {
	if c.signer == nil {
		return nil, ErrAuthUnavailableL1
	}
	if c.creds == nil {
		return nil, ErrAuthUnavailableL2
	}
	headers, err := c.level2Headers(http.MethodGet, EndpointBalanceAllowance, nil)
	if err != nil {
		return nil, err
	}
	u := c.host + EndpointBalanceAllowance
	u = addBalanceAllowanceQuery(u, params, c.sigType)
	resp, err := doJSON(ctx, c.http, http.MethodGet, u, headers, nil)
	if err != nil {
		return nil, err
	}
	m, ok := resp.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected balance-allowance response: %T", resp)
	}
	return m, nil
}

func (c *Client) UpdateBalanceAllowance(ctx context.Context, params *BalanceAllowanceParams) (map[string]any, error) {
	if c.signer == nil {
		return nil, ErrAuthUnavailableL1
	}
	if c.creds == nil {
		return nil, ErrAuthUnavailableL2
	}
	headers, err := c.level2Headers(http.MethodGet, EndpointBalanceAllowanceUpdt, nil)
	if err != nil {
		return nil, err
	}
	u := c.host + EndpointBalanceAllowanceUpdt
	u = addBalanceAllowanceQuery(u, params, c.sigType)
	resp, err := doJSON(ctx, c.http, http.MethodGet, u, headers, nil)
	if err != nil {
		return nil, err
	}
	m, ok := resp.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected balance-allowance/update response: %T", resp)
	}
	return m, nil
}

type OpenOrderParams struct {
	Market  string
	AssetID string
	ID      string
}

const endCursor = "LTE="
const defaultCursor = "MA=="

func (c *Client) GetOrders(ctx context.Context, params *OpenOrderParams) ([]map[string]any, error) {
	if c.signer == nil {
		return nil, ErrAuthUnavailableL1
	}
	if c.creds == nil {
		return nil, ErrAuthUnavailableL2
	}
	headers, err := c.level2Headers(http.MethodGet, EndpointOrders, nil)
	if err != nil {
		return nil, err
	}

	next := defaultCursor
	var out []map[string]any
	for next != endCursor {
		u := c.host + EndpointOrders
		u = addOpenOrdersQuery(u, params, next)
		resp, err := doJSON(ctx, c.http, http.MethodGet, u, headers, nil)
		if err != nil {
			return nil, err
		}
		m, ok := resp.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("unexpected orders response: %T", resp)
		}
		next = asString(m["next_cursor"])
		if next == "" {
			next = endCursor
		}
		data, _ := m["data"].([]any)
		for _, v := range data {
			om, _ := v.(map[string]any)
			if om != nil {
				out = append(out, om)
			}
		}
	}
	return out, nil
}

func addOpenOrdersQuery(base string, params *OpenOrderParams, nextCursor string) string {
	u := base
	q := url.Values{}
	if params != nil {
		if params.Market != "" {
			q.Set("market", params.Market)
		}
		if params.AssetID != "" {
			q.Set("asset_id", params.AssetID)
		}
		if params.ID != "" {
			q.Set("id", params.ID)
		}
	}
	if nextCursor != "" {
		q.Set("next_cursor", nextCursor)
	}
	if len(q) == 0 {
		return u
	}
	return u + "?" + q.Encode()
}

func addBalanceAllowanceQuery(base string, params *BalanceAllowanceParams, defaultSigType int) string {
	u := base
	q := url.Values{}
	if params != nil {
		if params.AssetType != "" {
			q.Set("asset_type", params.AssetType)
		}
		if params.TokenID != "" {
			q.Set("token_id", params.TokenID)
		}
		if params.SignatureType != 0 {
			q.Set("signature_type", fmt.Sprintf("%d", params.SignatureType))
		}
	}
	if q.Get("signature_type") == "" {
		q.Set("signature_type", fmt.Sprintf("%d", defaultSigType))
	}
	if len(q) == 0 {
		return u
	}
	return u + "?" + q.Encode()
}

func (c *Client) level1Headers(nonce int64) (map[string]string, error) {
	ts := time.Now().Unix()
	sig, err := SignClobAuthMessage(c.signer, ts, nonce)
	if err != nil {
		return nil, err
	}
	return map[string]string{
		HeaderPolyAddress:   c.signer.Address().Hex(),
		HeaderPolySignature: sig,
		HeaderPolyTimestamp: fmt.Sprintf("%d", ts),
		HeaderPolyNonce:     fmt.Sprintf("%d", nonce),
	}, nil
}

func (c *Client) level2Headers(method, path string, bodyBytes []byte) (map[string]string, error) {
	ts := time.Now().Unix()
	bodyStr := ""
	if bodyBytes != nil {
		bodyStr = string(bodyBytes)
	}
	hmacSig, err := BuildHMACSignature(c.creds.APISecret, ts, method, path, bodyStr)
	if err != nil {
		return nil, err
	}
	return map[string]string{
		HeaderPolyAddress:    c.signer.Address().Hex(),
		HeaderPolySignature:  hmacSig,
		HeaderPolyTimestamp:  fmt.Sprintf("%d", ts),
		HeaderPolyAPIKey:     c.creds.APIKey,
		HeaderPolyPassphrase: c.creds.APIPassphrase,
	}, nil
}

func asString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	default:
		return fmt.Sprintf("%v", v)
	}
}

func asBool(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return strings.ToLower(t) == "true"
	default:
		return false
	}
}

func asInt(v any) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case json.Number:
		i, _ := t.Int64()
		return int(i)
	default:
		return 0
	}
}

func floatFromTick(t TickSize) float64 {
	switch t {
	case "0.1":
		return 0.1
	case "0.01":
		return 0.01
	case "0.001":
		return 0.001
	case "0.0001":
		return 0.0001
	default:
		return 0.01
	}
}

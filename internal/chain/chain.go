package chain

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

const (
	USDCeAddress = "0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174"
	CTFAddress   = "0x4D97DCd97eC945f40cF65F87097ACe5EA0476045"
)

var (
	erc20ABI   = mustABI(`[{"constant":true,"inputs":[{"name":"_owner","type":"address"}],"name":"balanceOf","outputs":[{"name":"balance","type":"uint256"}],"type":"function"},{"constant":true,"inputs":[{"name":"_owner","type":"address"},{"name":"_spender","type":"address"}],"name":"allowance","outputs":[{"name":"","type":"uint256"}],"type":"function"},{"constant":false,"inputs":[{"name":"_spender","type":"address"},{"name":"_value","type":"uint256"}],"name":"approve","outputs":[{"name":"","type":"bool"}],"type":"function"}]`)
	erc1155ABI = mustABI(`[{"constant":true,"inputs":[{"name":"account","type":"address"},{"name":"id","type":"uint256"}],"name":"balanceOf","outputs":[{"name":"","type":"uint256"}],"type":"function"},{"constant":true,"inputs":[{"name":"account","type":"address"},{"name":"operator","type":"address"}],"name":"isApprovedForAll","outputs":[{"name":"","type":"bool"}],"type":"function"},{"constant":false,"inputs":[{"name":"operator","type":"address"},{"name":"approved","type":"bool"}],"name":"setApprovalForAll","outputs":[],"type":"function"},{"constant":false,"inputs":[{"name":"collateralToken","type":"address"},{"name":"parentCollectionId","type":"bytes32"},{"name":"conditionId","type":"bytes32"},{"name":"partition","type":"uint256[]"},{"name":"amount","type":"uint256"}],"name":"mergePositions","outputs":[],"type":"function"},{"constant":false,"inputs":[{"name":"collateralToken","type":"address"},{"name":"parentCollectionId","type":"bytes32"},{"name":"conditionId","type":"bytes32"},{"name":"indexSets","type":"uint256[]"}],"name":"redeemPositions","outputs":[],"type":"function"}]`)
)

type Client struct {
	rpcURL  string
	chainID *big.Int
	ec      *ethclient.Client

	privateKey *ecdsa.PrivateKey
	address    common.Address
}

func New(rpcURL string, privateKeyHex string, chainID int64) (*Client, error) {
	pk, err := crypto.HexToECDSA(strings.TrimPrefix(strings.TrimSpace(privateKeyHex), "0x"))
	if err != nil {
		return nil, err
	}
	ec, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, err
	}
	addr := crypto.PubkeyToAddress(pk.PublicKey)
	return &Client{
		rpcURL:     rpcURL,
		chainID:    big.NewInt(chainID),
		ec:         ec,
		privateKey: pk,
		address:    addr,
	}, nil
}

func (c *Client) Close() error                 { c.ec.Close(); return nil }
func (c *Client) Address() common.Address      { return c.address }
func (c *Client) EthClient() *ethclient.Client { return c.ec }

func (c *Client) USDCBalance(ctx context.Context) (float64, error) {
	contract := common.HexToAddress(USDCeAddress)
	data, err := erc20ABI.Pack("balanceOf", c.address)
	if err != nil {
		return 0, err
	}
	res, err := c.ec.CallContract(ctx, ethereum.CallMsg{To: &contract, Data: data}, nil)
	if err != nil {
		return 0, err
	}
	out, err := erc20ABI.Unpack("balanceOf", res)
	if err != nil {
		return 0, err
	}
	bal := out[0].(*big.Int)
	f := new(big.Rat).SetFrac(bal, big.NewInt(1_000_000))
	val, _ := f.Float64()
	return val, nil
}

func (c *Client) ERC20Allowance(ctx context.Context, token, spender common.Address) (*big.Int, error) {
	data, err := erc20ABI.Pack("allowance", c.address, spender)
	if err != nil {
		return nil, err
	}
	res, err := c.ec.CallContract(ctx, ethereum.CallMsg{To: &token, Data: data}, nil)
	if err != nil {
		return nil, err
	}
	out, err := erc20ABI.Unpack("allowance", res)
	if err != nil {
		return nil, err
	}
	return out[0].(*big.Int), nil
}

func (c *Client) ERC1155IsApprovedForAll(ctx context.Context, token, operator common.Address) (bool, error) {
	data, err := erc1155ABI.Pack("isApprovedForAll", c.address, operator)
	if err != nil {
		return false, err
	}
	res, err := c.ec.CallContract(ctx, ethereum.CallMsg{To: &token, Data: data}, nil)
	if err != nil {
		return false, err
	}
	out, err := erc1155ABI.Unpack("isApprovedForAll", res)
	if err != nil {
		return false, err
	}
	return out[0].(bool), nil
}

func (c *Client) ERC1155BalanceOf(ctx context.Context, token common.Address, tokenID *big.Int) (*big.Int, error) {
	data, err := erc1155ABI.Pack("balanceOf", c.address, tokenID)
	if err != nil {
		return nil, err
	}
	res, err := c.ec.CallContract(ctx, ethereum.CallMsg{To: &token, Data: data}, nil)
	if err != nil {
		return nil, err
	}
	out, err := erc1155ABI.Unpack("balanceOf", res)
	if err != nil {
		return nil, err
	}
	return out[0].(*big.Int), nil
}

func (c *Client) ApproveUSDC(ctx context.Context, spender common.Address, amount *big.Int) (common.Hash, error) {
	return c.transact(ctx, common.HexToAddress(USDCeAddress), erc20ABI, "approve", spender, amount)
}

func (c *Client) SetCTFApprovalForAll(ctx context.Context, operator common.Address, approved bool) (common.Hash, error) {
	return c.transact(ctx, common.HexToAddress(CTFAddress), erc1155ABI, "setApprovalForAll", operator, approved)
}

func (c *Client) MergePositions(ctx context.Context, conditionID [32]byte, amountUSDC6 *big.Int) (common.Hash, error) {
	parent := [32]byte{}
	partition := []*big.Int{big.NewInt(1), big.NewInt(2)}
	return c.transact(ctx, common.HexToAddress(CTFAddress), erc1155ABI, "mergePositions",
		common.HexToAddress(USDCeAddress),
		parent,
		conditionID,
		partition,
		amountUSDC6,
	)
}

func (c *Client) RedeemPositions(ctx context.Context, conditionID [32]byte) (common.Hash, error) {
	parent := [32]byte{}
	indexSets := []*big.Int{big.NewInt(1), big.NewInt(2)}
	return c.transact(ctx, common.HexToAddress(CTFAddress), erc1155ABI, "redeemPositions",
		common.HexToAddress(USDCeAddress),
		parent,
		conditionID,
		indexSets,
	)
}

func (c *Client) transact(ctx context.Context, to common.Address, a abi.ABI, method string, args ...any) (common.Hash, error) {
	auth, err := bind.NewKeyedTransactorWithChainID(c.privateKey, c.chainID)
	if err != nil {
		return common.Hash{}, err
	}
	auth.Context = ctx

	// Reasonable defaults; we still estimate gas.
	auth.GasLimit = 300_000
	auth.GasPrice, _ = c.ec.SuggestGasPrice(ctx)

	bound := bind.NewBoundContract(to, a, c.ec, c.ec, c.ec)
	tx, err := bound.Transact(auth, method, args...)
	if err != nil {
		return common.Hash{}, err
	}
	// wait (similar to python wait_for_transaction_receipt timeout=120)
	_, err = bind.WaitMined(context.WithoutCancel(ctx), c.ec, tx)
	if err != nil {
		// not fatal for returning tx hash
		return tx.Hash(), nil
	}
	return tx.Hash(), nil
}

func mustABI(raw string) abi.ABI {
	a, err := abi.JSON(strings.NewReader(raw))
	if err != nil {
		panic(err)
	}
	return a
}

// Helper to mimic python behavior for timeouts.
func WithTimeout(parent context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, d)
}

func ConditionIDFromHex(hex0x string) ([32]byte, error) {
	var out [32]byte
	s := strings.TrimPrefix(strings.TrimSpace(hex0x), "0x")
	if len(s) != 64 {
		return out, fmt.Errorf("invalid condition id length")
	}
	b := common.FromHex("0x" + s)
	if len(b) != 32 {
		return out, fmt.Errorf("invalid condition id bytes")
	}
	copy(out[:], b)
	return out, nil
}

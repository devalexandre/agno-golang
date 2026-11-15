// Package evm ports Agno's EvmTools: sending ETH on an EVM chain through a
// JSON-RPC endpoint.
//
// Transactions are EIP-1559 (type 2), signed locally with the private key
// (EVM_PRIVATE_KEY) using a self-contained secp256k1/RLP implementation, then
// broadcast with eth_sendRawTransaction to EVM_RPC_URL. SendTransaction moves
// funds and therefore requires user confirmation.
//
// Read-only helpers (GetBalance, GetTransactionCount, GetTransactionReceipt)
// are Go additions, opt-in via WithEVMEnableReadMethods; Python's toolkit only
// exposes send_transaction.
package evm

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/devalexandre/agno-golang/agno/tools/internal/restapi"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// EvmTools interacts with an EVM blockchain.
type EvmTools struct {
	toolkit.Toolkit
	privateKey     string
	rpcURL         string
	httpClient     *http.Client
	enableSend     bool
	enableRead     bool
	receiptTimeout time.Duration
	pollInterval   time.Duration
	reqID          int64

	key     *big.Int
	address string
	keyErr  error
}

// Option configures EvmTools.
type Option func(*EvmTools)

// WithEVMPrivateKey sets the signing key (default EVM_PRIVATE_KEY).
func WithEVMPrivateKey(k string) Option { return func(e *EvmTools) { e.privateKey = k } }

// WithEVMRPCURL sets the JSON-RPC endpoint (default EVM_RPC_URL).
func WithEVMRPCURL(u string) Option { return func(e *EvmTools) { e.rpcURL = u } }

// WithEVMHTTPClient replaces the HTTP client.
func WithEVMHTTPClient(c *http.Client) Option { return func(e *EvmTools) { e.httpClient = c } }

// WithEVMEnableSendTransaction toggles SendTransaction (default true).
func WithEVMEnableSendTransaction(b bool) Option { return func(e *EvmTools) { e.enableSend = b } }

// WithEVMEnableReadMethods toggles the read-only helpers (default false).
func WithEVMEnableReadMethods(b bool) Option { return func(e *EvmTools) { e.enableRead = b } }

// WithEVMAll enables every method.
func WithEVMAll() Option { return func(e *EvmTools) { e.enableSend, e.enableRead = true, true } }

// WithEVMReceiptTimeout bounds how long SendTransaction waits for a receipt (default 120s).
func WithEVMReceiptTimeout(d time.Duration) Option {
	return func(e *EvmTools) { e.receiptTimeout = d }
}

// WithEVMPollInterval sets the receipt polling interval (default 1s).
func WithEVMPollInterval(d time.Duration) Option { return func(e *EvmTools) { e.pollInterval = d } }

// SendTransactionParams are the parameters of SendTransaction.
type SendTransactionParams struct {
	ToAddress   string `json:"to_address" description:"The address to which you want to send ETH." required:"true"`
	AmountInWei string `json:"amount_in_wei" description:"The amount of ETH to send in wei (integer, decimal string)." required:"true"`
}

// AddressParams identifies an account.
type AddressParams struct {
	Address string `json:"address,omitempty" description:"Account address. Defaults to the toolkit wallet address."`
}

// TxHashParams identifies a transaction.
type TxHashParams struct {
	TxHash string `json:"tx_hash" description:"Transaction hash (0x-prefixed)." required:"true"`
}

// NewEvmTools creates the EVM toolkit.
func NewEvmTools(opts ...Option) *EvmTools {
	e := &EvmTools{
		privateKey:     os.Getenv("EVM_PRIVATE_KEY"),
		rpcURL:         os.Getenv("EVM_RPC_URL"),
		enableSend:     true,
		receiptTimeout: 120 * time.Second,
		pollInterval:   time.Second,
	}
	e.Toolkit = toolkit.NewToolkit()
	e.Toolkit.Name = "evm_tools"
	e.Toolkit.Description = "Send ETH and query accounts on an EVM-compatible blockchain via JSON-RPC."
	for _, o := range opts {
		o(e)
	}
	if e.httpClient == nil {
		e.httpClient = e.Toolkit.HTTPClient()
	}
	if e.privateKey != "" {
		e.key, e.keyErr = parsePrivateKey(e.privateKey)
		if e.keyErr == nil {
			e.address = pubToAddress(baseMult(e.key))
		}
	}
	if e.enableSend {
		e.Toolkit.RegisterWithOptions("SendTransaction", "Sends a transaction (ETH transfer) to the address provided. Returns the transaction hash or an error message.", e, e.SendTransaction, SendTransactionParams{}, toolkit.WithConfirmation())
	}
	if e.enableRead {
		e.Toolkit.Register("GetBalance", "Get the balance (in wei) of an address.", e, e.GetBalance, AddressParams{})
		e.Toolkit.Register("GetTransactionCount", "Get the nonce (transaction count) of an address.", e, e.GetTransactionCount, AddressParams{})
		e.Toolkit.Register("GetTransactionReceipt", "Get the receipt of a transaction by hash.", e, e.GetTransactionReceipt, TxHashParams{})
	}
	return e
}

// Address returns the wallet address derived from the private key.
func (e *EvmTools) Address() string { return e.address }

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *EvmTools) call(ctx context.Context, method string, out interface{}, params ...interface{}) error {
	if e.rpcURL == "" {
		return errors.New("RPC Url is needed to interact with EVM blockchain")
	}
	if params == nil {
		params = []interface{}{}
	}
	req := map[string]interface{}{"jsonrpc": "2.0", "id": atomic.AddInt64(&e.reqID, 1), "method": method, "params": params}
	var resp struct {
		Result json.RawMessage `json:"result"`
		Error  *rpcError       `json:"error"`
	}
	if err := restapi.New(e.rpcURL, e.httpClient, nil).JSON(ctx, http.MethodPost, e.rpcURL, nil, req, &resp); err != nil {
		return err
	}
	if resp.Error != nil {
		return fmt.Errorf("rpc error %d: %s", resp.Error.Code, resp.Error.Message)
	}
	if out != nil {
		return json.Unmarshal(resp.Result, out)
	}
	return nil
}

func (e *EvmTools) callBig(ctx context.Context, method string, params ...interface{}) (*big.Int, error) {
	var s string
	if err := e.call(ctx, method, &s, params...); err != nil {
		return nil, err
	}
	return parseHexBig(s)
}

func parseHexBig(s string) (*big.Int, error) {
	v, ok := new(big.Int).SetString(strings.TrimPrefix(s, "0x"), 16)
	if !ok {
		if s == "0x" || s == "0x0" {
			return big.NewInt(0), nil
		}
		return nil, fmt.Errorf("invalid hex quantity %q", s)
	}
	return v, nil
}

func hexBig(v *big.Int) string { return "0x" + v.Text(16) }

// SendTransaction signs and broadcasts an EIP-1559 ETH transfer.
func (e *EvmTools) SendTransaction(p SendTransactionParams) (interface{}, error) {
	hash, err := e.sendTransaction(context.Background(), p)
	if err != nil {
		return fmt.Sprintf("error: %v", err), nil
	}
	return hash, nil
}

func (e *EvmTools) sendTransaction(ctx context.Context, p SendTransactionParams) (string, error) {
	if e.privateKey == "" {
		return "", errors.New("Private Key is required")
	}
	if e.keyErr != nil {
		return "", e.keyErr
	}
	to := strings.TrimPrefix(strings.TrimSpace(p.ToAddress), "0x")
	toBytes, err := hex.DecodeString(to)
	if err != nil || len(toBytes) != 20 {
		return "", fmt.Errorf("invalid to_address %q", p.ToAddress)
	}
	value, ok := new(big.Int).SetString(strings.TrimSpace(p.AmountInWei), 10)
	if !ok || value.Sign() < 0 {
		return "", fmt.Errorf("invalid amount_in_wei %q", p.AmountInWei)
	}

	maxPriority := big.NewInt(1_000_000_000) // 1 gwei
	var block struct {
		BaseFeePerGas *string `json:"baseFeePerGas"`
	}
	if err := e.call(ctx, "eth_getBlockByNumber", &block, "latest", false); err != nil {
		return "", err
	}
	if block.BaseFeePerGas == nil {
		return "", errors.New("Base fee per gas not found in the latest block.")
	}
	baseFee, err := parseHexBig(*block.BaseFeePerGas)
	if err != nil {
		return "", err
	}
	maxFee := new(big.Int).Add(new(big.Int).Mul(baseFee, big.NewInt(2)), maxPriority)
	nonce, err := e.callBig(ctx, "eth_getTransactionCount", e.address, "pending")
	if err != nil {
		return "", err
	}
	chainID, err := e.callBig(ctx, "eth_chainId")
	if err != nil {
		return "", err
	}

	raw := signEIP1559(e.key, chainID, nonce, maxPriority, maxFee, big.NewInt(21000), toBytes, value)
	var txHash string
	if err := e.call(ctx, "eth_sendRawTransaction", &txHash, "0x"+hex.EncodeToString(raw)); err != nil {
		return "", err
	}

	deadline := time.Now().Add(e.receiptTimeout)
	for {
		var receipt map[string]interface{}
		if err := e.call(ctx, "eth_getTransactionReceipt", &receipt, txHash); err != nil {
			return "", err
		}
		if receipt != nil {
			if status, _ := receipt["status"].(string); status == "0x1" {
				return txHash, nil
			}
			return "", errors.New("Transaction failed!")
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("timed out waiting for receipt of %s", txHash)
		}
		time.Sleep(e.pollInterval)
	}
}

// signEIP1559 builds and signs a type-2 transaction, returning its raw bytes.
func signEIP1559(key, chainID, nonce, maxPriority, maxFee, gas *big.Int, to []byte, value *big.Int) []byte {
	fields := [][]byte{
		rlpInt(chainID), rlpInt(nonce), rlpInt(maxPriority), rlpInt(maxFee), rlpInt(gas),
		rlpBytes(to), rlpInt(value), rlpBytes(nil), rlpList(),
	}
	sigHash := keccak256([]byte{0x02}, rlpList(fields...))
	r, s, recID := sign(key, sigHash)
	fields = append(fields, rlpInt(big.NewInt(int64(recID&1))), rlpInt(r), rlpInt(s))
	return append([]byte{0x02}, rlpList(fields...)...)
}

func (e *EvmTools) addressOr(a string) string {
	if strings.TrimSpace(a) != "" {
		return strings.TrimSpace(a)
	}
	return e.address
}

// GetBalance returns the balance of an address in wei.
func (e *EvmTools) GetBalance(p AddressParams) (interface{}, error) {
	addr := e.addressOr(p.Address)
	if addr == "" {
		return "error: an address is required", nil
	}
	v, err := e.callBig(context.Background(), "eth_getBalance", addr, "latest")
	if err != nil {
		return fmt.Sprintf("error: %v", err), nil
	}
	return v.String(), nil
}

// GetTransactionCount returns the nonce of an address.
func (e *EvmTools) GetTransactionCount(p AddressParams) (interface{}, error) {
	addr := e.addressOr(p.Address)
	if addr == "" {
		return "error: an address is required", nil
	}
	v, err := e.callBig(context.Background(), "eth_getTransactionCount", addr, "latest")
	if err != nil {
		return fmt.Sprintf("error: %v", err), nil
	}
	return v.String(), nil
}

// GetTransactionReceipt returns a transaction receipt as JSON.
func (e *EvmTools) GetTransactionReceipt(p TxHashParams) (interface{}, error) {
	var receipt map[string]interface{}
	if err := e.call(context.Background(), "eth_getTransactionReceipt", &receipt, p.TxHash); err != nil {
		return fmt.Sprintf("error: %v", err), nil
	}
	if receipt == nil {
		return "Receipt not found (transaction pending or unknown).", nil
	}
	return restapi.ToJSON(receipt), nil
}

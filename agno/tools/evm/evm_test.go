package evm

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const key1 = "0x0000000000000000000000000000000000000000000000000000000000000001"

func TestAddressFromKey(t *testing.T) {
	d, err := parsePrivateKey(key1)
	if err != nil {
		t.Fatal(err)
	}
	if got := pubToAddress(baseMult(d)); got != "0x7E5F4552091A69125d5DfCb7b8C2659029395Bdf" {
		t.Fatalf("address %s", got)
	}
}

func TestRFC6979Vector(t *testing.T) {
	// Trezor/python-ecdsa secp256k1 vector: key=1, msg="Satoshi Nakamoto".
	d := big.NewInt(1)
	h := sha256.Sum256([]byte("Satoshi Nakamoto"))
	k := rfc6979K(d, h[:])
	if hex.EncodeToString(pad32(k.Bytes())) != "8f8a276c19f4149656b280621e358cce24f5f52542772691ee69063b74f15d15" {
		t.Fatalf("k = %x", k)
	}
	r, s, _ := sign(d, h[:])
	if hex.EncodeToString(pad32(r.Bytes())) != "934b1ea10a4b3c1757e2b0c017d0b6143ce3c9a7e6a4a49860d7a6ab210ee3d8" ||
		hex.EncodeToString(pad32(s.Bytes())) != "2442ce9d2b916064108014783e923ec36b49743e2ffa1c4496f01a512aafd9e5" {
		t.Fatalf("sig r=%x s=%x", r, s)
	}
}

func TestRLP(t *testing.T) {
	if hex.EncodeToString(rlpBytes([]byte("dog"))) != "83646f67" ||
		hex.EncodeToString(rlpList()) != "c0" ||
		hex.EncodeToString(rlpInt(big.NewInt(0))) != "80" ||
		hex.EncodeToString(rlpInt(big.NewInt(1024))) != "820400" {
		t.Fatal("rlp mismatch")
	}
	long := rlpBytes([]byte(strings.Repeat("a", 56)))
	if hex.EncodeToString(long[:2]) != "b838" {
		t.Fatalf("long prefix %x", long[:2])
	}
}

func TestSignAndRecover(t *testing.T) {
	d, _ := parsePrivateKey("4c0883a69102937d6231471b5dbb6204fe5129617082792ae468d01a3f362318")
	for i := 0; i < 5; i++ {
		h := keccak256([]byte{byte(i)})
		r, s, rec := sign(d, h)
		if s.Cmp(halfN) > 0 {
			t.Fatal("high s")
		}
		q, err := recoverPub(h, r, s, rec)
		if err != nil || pubToAddress(q) != pubToAddress(baseMult(d)) {
			t.Fatalf("recover mismatch %v", err)
		}
	}
}

func TestSendTransaction(t *testing.T) {
	var raw string
	receiptCalls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     interface{}   `json:"id"`
			Method string        `json:"method"`
			Params []interface{} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		var result interface{}
		switch req.Method {
		case "eth_getBlockByNumber":
			result = map[string]string{"baseFeePerGas": "0x3b9aca00"}
		case "eth_getTransactionCount":
			result = "0x5"
		case "eth_chainId":
			result = "0xaa36a7"
		case "eth_sendRawTransaction":
			raw = req.Params[0].(string)
			result = "0xabc"
		case "eth_getTransactionReceipt":
			receiptCalls++
			if receiptCalls > 1 {
				result = map[string]string{"status": "0x1"}
			}
		case "eth_getBalance":
			result = "0xde0b6b3a7640000"
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"jsonrpc": "2.0", "id": req.ID, "result": result})
	}))
	defer srv.Close()

	e := NewEvmTools(WithEVMPrivateKey(key1), WithEVMRPCURL(srv.URL), WithEVMAll(), WithEVMPollInterval(time.Millisecond))
	if !e.GetMethods()["evm_tools_SendTransaction"].RequiresConfirmation {
		t.Fatal("send must require confirmation")
	}
	out, _ := e.SendTransaction(SendTransactionParams{ToAddress: "0x7E5F4552091A69125d5DfCb7b8C2659029395Bdf", AmountInWei: "1000"})
	if out != "0xabc" {
		t.Fatalf("send: %v", out)
	}
	if !strings.HasPrefix(raw, "0x02f8") && !strings.HasPrefix(raw, "0x02f7") {
		t.Fatalf("raw tx not type-2 list: %s", raw)
	}
	if out, _ = e.GetBalance(AddressParams{}); out != "1000000000000000000" {
		t.Fatalf("balance %v", out)
	}
	if out, _ = e.SendTransaction(SendTransactionParams{ToAddress: "0x12", AmountInWei: "1"}); !strings.HasPrefix(out.(string), "error: invalid to_address") {
		t.Fatalf("bad addr %v", out)
	}
}

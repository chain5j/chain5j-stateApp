// Package stateApp
//
// @author: xwc1125
package stateApp

import (
	"github.com/chain5j/chain5j-pkg/codec"
	"github.com/chain5j/chain5j-pkg/crypto/signature"
	"github.com/chain5j/chain5j-pkg/util/hexutil"
	"github.com/chain5j/chain5j-protocol/models"
	"github.com/davecgh/go-spew/spew"
	"log"
	"math/big"
	"testing"
)

type SginMsg struct {
	V *big.Int `json:"v"`
	R *big.Int `json:"r"`
	S *big.Int `json:"s"`
}

func TestTransactionsRlp(t *testing.T) {
	var txs models.Transactions

	tx := &Transaction{
		data: txData{
			//From:      types.HexToAddress("0x9254E62FBCA63769DFd4Cc8e23f630F0785610CE"),
			To:       "0x353C02434dE6c99F5587b62Ae9d6DA2BD776Daa7",
			Nonce:    260,
			GasLimit: 60000,
			GasPrice: 0,
			Value:    big.NewInt(1000000),
			Input:    []byte("0x"),
			//Timestamp: uint64(dateutil.CurrentTime()),
			Signature: nil,
		},
	}

	txs.Add(tx)

	enc, err := codec.Coder().Encode(&txs)
	if err != nil {
		log.Fatal(err)
	}

	t.Logf("rlpbytes: %s\n", hexutil.Encode(enc))

	var dec models.Transactions
	if err := codec.Coder().Decode(enc, &dec); err != nil {
		log.Fatal(err)
	}

	return
}

// curl -H "Content-Type:application/json" -X POST --data '{"timestamp":"123456","method":"pg_sendTransaction","params":[0,"0xf87194353c02434de6c99f5587b62ae9d6da2bd776daa782010482ea6080830f424082307886016e86edf9f4b845f843a0e1d205005444956245faca07bf9c277186e53a2d124fceb76eec41bfe6a10d53a068b541501007d46202388449f3941cac2af1bcee3cc080c7ddc72a8cb0facc5e1c"],"id":1}' http://127.0.0.1:9545
func TestTransactionRLP(t *testing.T) {
	v, _ := hexutil.DecodeBig("0xe1d205005444956245faca07bf9c277186e53a2d124fceb76eec41bfe6a10d53")
	r, _ := hexutil.DecodeBig("0x68b541501007d46202388449f3941cac2af1bcee3cc080c7ddc72a8cb0facc5e")
	s, _ := hexutil.DecodeBig("0x1c")
	signMsg := &SginMsg{
		V: v,
		R: r,
		S: s,
	}
	signBytes, _ := codec.Coder().Encode(signMsg)
	signResult := &signature.SignResult{
		Name:      signature.S256,
		PubKey:    nil,
		Signature: signBytes,
	}

	transaction := &Transaction{
		data: txData{
			//From:      types.HexToAddress("0x9254E62FBCA63769DFd4Cc8e23f630F0785610CE"),
			To:       "0x353C02434dE6c99F5587b62Ae9d6DA2BD776Daa7",
			Nonce:    260,
			GasLimit: 60000,
			GasPrice: 0,
			Value:    big.NewInt(1000000),
			Input:    []byte("0x"),
			//Timestamp: uint64(dateutil.CurrentTime()),
			Signature: signResult,
		},
	}
	toBytes, err := codec.Coder().Encode(&transaction)
	if err != nil {
		panic(err)
	}
	encode := hexutil.Encode(toBytes)
	spew.Dump(encode)
	spew.Dump(toBytes)
	decode, _ := hexutil.Decode(encode)
	tx2 := &Transaction{}
	err = codec.Coder().Decode(decode, tx2)
	if err != nil {
		panic(err)
	}
	spew.Dump(tx2)
}

func TestDecode(t *testing.T) {
	rawTx := "0xf87194353c02434de6c99f5587b62ae9d6da2bd776daa782010482ea6080830f424082307886016e86edf9f4b845f843a0e1d205005444956245faca07bf9c277186e53a2d124fceb76eec41bfe6a10d53a068b541501007d46202388449f3941cac2af1bcee3cc080c7ddc72a8cb0facc5e1c"

	decode, _ := hexutil.Decode(rawTx)
	tx2 := &Transaction{}
	err := codec.Coder().Decode(decode, tx2)
	if err != nil {
		panic(err)
	}

	spew.Dump(tx2)
}

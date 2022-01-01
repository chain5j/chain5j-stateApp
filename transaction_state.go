// Package stateApp
//
// @author: xwc1125
package stateApp

import (
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/chain5j/chain5j-pkg/codec"
	"github.com/chain5j/chain5j-pkg/codec/rlp"
	"github.com/chain5j/chain5j-pkg/crypto/hashalg"
	"github.com/chain5j/chain5j-pkg/crypto/signature"
	"github.com/chain5j/chain5j-pkg/math"
	"github.com/chain5j/chain5j-pkg/types"
	"github.com/chain5j/chain5j-protocol/models"
	"github.com/chain5j/chain5j-protocol/pkg/crypto"
	"github.com/chain5j/logger"
	"io"
	"math/big"
	"strings"
	"sync/atomic"
)

var (
	_ models.Transaction = new(Transaction)
)

func init() {
	models.RegisterTransaction(&Transaction{})
}

type Transaction struct {
	data txData

	// cache
	fromPub atomic.Value // 签名的公钥，不需要入库
	signer  atomic.Value
	txHash  atomic.Value // hash cache
	size    atomic.Value // size cache
}

type txData struct {
	From        string     `json:"from" ` // 使用域名
	To          string     `json:"to" `   // 使用域名
	Interpreter string     `json:"interpreter"`
	Nonce       uint64     `json:"nonce"`
	GasLimit    uint64     `json:"gasLimit"`
	GasPrice    uint64     `json:"gasPrice"`
	Value       *big.Int   `json:"value"`
	Input       []byte     `json:"input"`
	Deadline    uint64     `json:"deadline"`  // 截止时间，如果为0，表示不限制。在共识中需要使用block.time进行校验
	ExtraHash   types.Hash `json:"extraHash"` // 扩展内容的hash

	Signature *signature.SignResult `json:"signature"` // 签名数据
	// 对于接入的节点可见，其他节点不可见的状态
	Extra []byte `json:"extra" rlp:"-"` // 扩展内容，不需要进行sign校验

	// This is only used when marshaling to JSON.
	Hash types.Hash `json:"hash" rlp:"-"`
}

func NewTransaction(from string, to string, interpreter string, nonce uint64, gasPrice, gasLimit uint64, value *big.Int, input []byte, deadline uint64, extra []byte) *Transaction {
	if interpreter == "" {
		interpreter = BaseInterpreter
	}

	return &Transaction{
		data: NewTxData(from, to, interpreter, nonce, gasPrice, gasLimit, value, input, deadline, extra),
	}
}

func NewTxData(from string, to string, interpreter string, nonce uint64, gasPrice, gasLimit uint64, value *big.Int, input []byte, deadline uint64, extra []byte) txData {
	return txData{
		From:        from,
		To:          to,
		Interpreter: interpreter,
		Nonce:       nonce,
		GasLimit:    gasLimit,
		GasPrice:    gasPrice,
		Value:       value,
		Input:       input,
		Deadline:    deadline,
		ExtraHash:   types.BytesToHash(extra),
		Extra:       extra,
	}
}

func (t txData) Serialize() ([]byte, error) {
	return json.Marshal(t)
}

func (t txData) Deserialize(d []byte) error {
	return json.Unmarshal(d, t)
}

type TxJson struct {
	Data txData

	// cache
	Signer          types.Address
	TransactionHash types.Hash // hash cache
}

func (tx *Transaction) Serialize() ([]byte, error) {
	tx1 := &TxJson{
		Data: tx.data,
	}
	if address := tx.signer.Load(); address != nil {
		tx1.Signer = address.(types.Address)
	}
	if txHash := tx.txHash.Load(); txHash != nil {
		tx1.TransactionHash = txHash.(types.Hash)
	}

	bytes, err := codec.Coder().Encode(tx1)
	return bytes, err
}

func (tx *Transaction) Deserialize(d []byte) error {
	if d == nil {
		return errors.New("byte is empty")
	}
	var txJson TxJson
	err := codec.Coder().Decode(d, &txJson)
	if err != nil {
		return err
	}
	tx.data = txJson.Data

	rawHash, err := tx.getRawHash()
	if err != nil {
		return err
	}
	pubKey, err := crypto.RecoverPubKey(rawHash.Bytes(), tx.data.Signature)
	if err != nil {
		return err
	}
	tx.fromPub.Store(pubKey)
	tx.signer.Store(txJson.Signer)
	tx.txHash.Store(txJson.TransactionHash)
	return nil
}

func (tx *Transaction) MarshalJSON() ([]byte, error) {
	tx.data.Hash = tx.Hash()
	return json.Marshal(tx.data)
}

func (tx *Transaction) UnmarshalJSON(input []byte) error {
	return json.Unmarshal(input, &tx.data)
}

func (tx *Transaction) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, &tx.data)
}

func (tx *Transaction) DecodeRLP(s *rlp.Stream) error {
	_, size, _ := s.Kind()
	err := s.Decode(&tx.data)
	if err == nil {
		tx.size.Store(types.StorageSize(rlp.ListSize(size)))
	}
	return err
}

func (tx *Transaction) Less(tx2 models.Transaction) bool {
	transaction := tx2.(*Transaction)
	if tx.Nonce() < transaction.Nonce() {
		return true
	} else if tx.Nonce() == transaction.Nonce() {
		return tx.GasPrice() > transaction.GasPrice()
	} else {
		return false
	}
}

func (tx *Transaction) TxType() types.TxType {
	return "STATE"
}

func (tx *Transaction) ChainId() string {
	return "1"
}

func (tx *Transaction) From() string {
	return strings.ToLower(tx.data.From)
}

func (tx *Transaction) To() string {
	return strings.ToLower(tx.data.To)
}

func (tx *Transaction) Nonce() uint64 {
	return tx.data.Nonce
}

func (tx *Transaction) Value() *big.Int {
	return tx.data.Value
}

func (tx *Transaction) GasLimit() uint64 {
	return tx.data.GasLimit
}

func (tx *Transaction) GasPrice() uint64 {
	return tx.data.GasPrice
}

func (tx *Transaction) Interpreter() string {
	return tx.data.Interpreter
}

func (tx *Transaction) Input() []byte {
	return tx.data.Input
}

func (tx *Transaction) Deadline() uint64 {
	return tx.data.Deadline
}

func (tx *Transaction) Signer() (types.Address, error) {
	if address := tx.signer.Load(); address != nil {
		return address.(types.Address), nil
	}

	rlpHash, err := tx.getRawHash()
	if err != nil {
		return types.EmptyAddress, err
	}
	tx.data.Signature.PubKey = nil
	pubKey, err := crypto.RecoverPubKey(rlpHash[:], tx.data.Signature)
	if err != nil {
		return types.EmptyAddress, err
	}
	address, err := crypto.PubkeyToAddress(pubKey)
	if err != nil {
		return types.Address{}, err
	}
	logger.Debug("tx from addr", "addr", address)
	tx.signer.Store(address)
	tx.fromPub.Store(pubKey)
	return address, nil
}

func (tx *Transaction) Hash() types.Hash {
	if hash := tx.txHash.Load(); hash != nil {
		return hash.(types.Hash)
	}
	// TODO 计算hash应该与sign中的rlpHash一致？
	hash, _ := tx.getSignedHash()
	tx.txHash.Store(hash)
	return hash
}

// getRawHash 获取需要签名的交易
func (tx *Transaction) getRawHash() (types.Hash, error) {
	return hashalg.RlpHash([]interface{}{
		tx.data.From,
		tx.data.To,
		tx.data.Interpreter,
		tx.data.Nonce,
		tx.data.GasLimit,
		tx.data.GasPrice,
		tx.data.Value,
		tx.data.Input,
		tx.data.Deadline,
		tx.data.ExtraHash,
		nil,
	})
}
func (tx *Transaction) getSignedHash() (types.Hash, error) {
	return hashalg.RlpHash([]interface{}{
		tx.data.From,
		tx.data.To,
		tx.data.Interpreter,
		tx.data.Nonce,
		tx.data.GasLimit,
		tx.data.GasPrice,
		tx.data.Value,
		tx.data.Input,
		tx.data.Deadline,
		tx.data.ExtraHash,
		tx.data.Signature,
	})
}

func (tx *Transaction) Sign(privKey *ecdsa.PrivateKey) (*signature.SignResult, error) {
	if sign := tx.data.Signature; sign != nil {
		return sign, nil
	}
	rlpHash, err := tx.getRawHash()
	if err != nil {
		return nil, err
	}
	fmt.Println("rlpHash", rlpHash.Hex())

	tx.data.Signature, err = signature.SignWithECDSA(privKey, rlpHash.Bytes())
	if err != nil {
		return nil, err
	}

	return tx.data.Signature, nil
}

func (tx *Transaction) Cost() *big.Int {
	return new(big.Int).Add(tx.data.Value, new(big.Int).SetUint64(tx.data.GasLimit*tx.GasPrice()))
}

func (tx *Transaction) Size() types.StorageSize {
	if size := tx.size.Load(); size != nil {
		return size.(types.StorageSize)
	}
	c := types.WriteCounter(0)
	rlp.Encode(&c, &tx.data)
	tx.size.Store(types.StorageSize(c))
	return types.StorageSize(c)
}

func (tx *Transaction) PubKey() *ecdsa.PublicKey {
	if pub := tx.fromPub.Load(); pub != nil {
		fromPub := pub.(*ecdsa.PublicKey)
		return fromPub
	}
	return nil
}

func (tx *Transaction) AsEvmMessage() (models.VmMessage, error) {
	var (
		from types.Address
		to   types.Address
	)

	from = types.HexToAddress(tx.From())
	to = types.HexToAddress(tx.To())

	return models.NewEvmMessage(from, &to, tx.data.Nonce, tx.data.Value, tx.data.GasLimit, math.Big0, tx.data.Input, true), nil
}

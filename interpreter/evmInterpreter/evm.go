// Package evmInterpreter
//
// @author: xwc1125
package evmInterpreter

import (
	evm "github.com/chain5j/chain5j-evm"
	"github.com/chain5j/chain5j-pkg/types"
	"github.com/chain5j/chain5j-protocol/models"
	"github.com/chain5j/chain5j-protocol/protocol"
	"math/big"
)

// NewEVMContext creates a new context for use in the EVM.
func NewEVMContext(msg models.VmMessage, header *models.Header, chain protocol.ChainContext, author *types.Address) evm.Context {
	var beneficiary types.Address
	if author == nil {
		beneficiary = types.Address{}
	} else {
		beneficiary = *author
	}

	return evm.Context{
		CanTransfer: CanTransfer,
		Transfer:    Transfer,
		GetHash:     GetHashFn(header, chain),
		Origin:      types.DomainToAddress(msg.From()),
		Coinbase:    beneficiary,
		BlockNumber: big.NewInt(int64(header.Height)),
		Time:        big.NewInt(int64(header.Timestamp)),
		GasLimit:    header.GasLimit,
	}
}

// GetHashFn returns a GetHashFunc which retrieves header hashes by number
func GetHashFn(ref *models.Header, chain protocol.ChainContext) func(n uint64) types.Hash {
	var cache map[uint64]types.Hash

	return func(n uint64) types.Hash {
		// If there's no hash cache yet, make one
		if cache == nil {
			cache = map[uint64]types.Hash{
				ref.Height - 1: ref.ParentHash,
			}
		}
		// Try to fulfill the request from the cache
		if hash, ok := cache[n]; ok {
			return hash
		}
		// Not cached, iterate the blocks and cache the hashes
		for header := chain.GetHeader(ref.ParentHash, ref.Height-1); header != nil; header = chain.GetHeader(header.ParentHash, header.Height-1) {
			cache[header.Height-1] = header.ParentHash
			if n == header.Height-1 {
				return header.ParentHash
			}
		}
		return types.Hash{}
	}
}

// CanTransfer checks whether there are enough funds in the address' account to make a transfer.
// This does not take the necessary gas in to account to make the transfer valid.
func CanTransfer(db evm.StateDB, addr types.Address, amount *big.Int) bool {
	// TODO  需要判断目标地址是否存在
	return db.GetBalance(addr).Cmp(amount) >= 0
}

// Transfer subtracts amount from sender and adds amount to recipient using the given Db
func Transfer(db evm.StateDB, sender, recipient types.Address, amount *big.Int) {
	db.SubBalance(sender, amount)
	db.AddBalance(recipient, amount)
}

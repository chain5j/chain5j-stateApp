// Package txpool
//
// @author: xwc1125
package txpool

import (
	"github.com/chain5j/chain5j-protocol/models"
	"math/big"
	"sort"
	"sync"
)

// nonce ==> txList
type txList struct {
	mu    sync.RWMutex
	txs   map[uint64]models.StateTransaction
	cache models.Transactions
}

func newTxList() *txList {
	return &txList{
		txs: make(map[uint64]models.StateTransaction),
	}
}

// get tx by nonce
func (list *txList) get(nonce uint64) models.StateTransaction {
	list.mu.RLock()
	defer list.mu.RUnlock()
	return list.txs[nonce]
}

// Add adds a new transaction to the list, returning whether the
// transaction was accepted, and if yes, any previous transaction it replaced.
//
// PriceBump is the percent number
func (list *txList) add(tx models.StateTransaction, priceBump int) (bool, models.Transaction) {
	canInsert, old := list.CanInsert(tx, priceBump)
	if !canInsert {
		return false, nil
	}
	list.Put(tx)
	return true, old
}

// Put TODO
// nonce ==> tx
func (list *txList) Put(tx models.StateTransaction) {
	list.mu.Lock()
	defer list.mu.Unlock()
	list.txs[tx.Nonce()] = tx
	list.cache = nil
}

// CanInsert TODO
// whether can insert
// 1. nonce is exist
// 2. gasPrice is bigger
func (list *txList) CanInsert(tx models.StateTransaction, priceBump int) (bool, models.Transaction) {
	var old models.Transaction
	if old := list.get(tx.Nonce()); old != nil {
		// Replacement strategy. Temporary design
		if old.GasPrice() >= tx.GasPrice() {
			return false, nil
		}
		boundGas := old.GasLimit() * uint64(100+priceBump) / 100
		if boundGas > tx.GasLimit() {
			return false, nil
		}
	}
	return true, old
}

// Del deletes a transaction from the list
func (list *txList) Del(nonce uint64) {
	if old := list.get(nonce); old == nil {
		return
	}

	delete(list.txs, nonce)
	list.cache = nil
}

// Len TODO
func (list *txList) Len() uint64 {
	list.mu.RLock()
	defer list.mu.RUnlock()
	return uint64(len(list.txs))
}

// All creates a nonce-sorted slice of current transaction list,
// and the result will be cache in case any modifications are made
func (list *txList) All() models.Transactions {
	var results models.Transactions
	if list.cache == nil {
		for _, tx := range list.txs {
			//list.cache = append(list.cache, tx)
			list.cache.Add(tx)
		}

		sort.Sort(list.cache)
	}
	copy(results.Data(), list.cache.Data())

	return results
}

// Filter filters all transactions which make filter func true and false, and
// removes unmatching transactions from the list
func (list *txList) filter(filter func(tx models.StateTransaction) bool) (models.Transactions, models.Transactions) {
	var (
		matched   models.Transactions
		unmatched models.Transactions
	)
	list.mu.Lock()
	defer list.mu.Unlock()
	for _, tx := range list.txs {
		if filter(tx) {
			matched.Add(tx)
		} else {
			unmatched.Add(tx)
		}
	}

	for _, txList := range unmatched.Data() {
		for _, tx := range txList {
			if transaction, ok := tx.(models.StateTransaction); ok {
				delete(list.txs, transaction.Nonce())
			}
		}
	}

	list.cache = nil
	return matched, unmatched
}

// Ready retrieves a sequentially increasing list of transactions starting at the
// provided nonce that is ready for processing. The returned transactions will be
// removed from the list.
func (list *txList) Ready(start uint64) models.Transactions {
	var (
		results models.Transactions
		nonce   = start
	)
	list.mu.Lock()
	defer list.mu.Unlock()
	for {
		if tx, exist := list.txs[nonce]; exist {
			results.Add(tx)
			nonce = nonce + 1
		} else {
			break
		}
	}
	for _, txList := range results.Data() {
		for _, tx := range txList {
			if transaction, ok := tx.(models.StateTransaction); ok {
				delete(list.txs, transaction.Nonce())
			}
		}
	}
	list.cache = nil
	return results
}

// Forget drops all transactions whose nonce is lower than bound.
// Every removed transaction is returned.
func (list *txList) Forget(bound uint64) models.Transactions {
	_, drops := list.filter(func(tx models.StateTransaction) bool {
		return tx.Nonce() >= bound
	})
	return drops
}

// Release drops all transactions whose cost is over balance.
// Every removed transaction is returned.
func (list *txList) Release(balance *big.Int) models.Transactions {
	_, drops := list.filter(func(tx models.StateTransaction) bool {
		return tx.Cost().Cmp(balance) <= 0
	})
	return drops
}

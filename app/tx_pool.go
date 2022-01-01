// Package app
//
// @author: xwc1125
package app

// import (
//	"context"
//	prque "github.com/chain5j/chain5j-pkg/collection/queues/preque"
//	"github.com/chain5j/chain5j-pkg/types"
//	"github.com/chain5j/chain5j-protocol/models"
//	"github.com/chain5j/chain5j-protocol/protocol"
//	"github.com/chain5j/logger"
//	"sort"
//	"sync"
// )
//
// type txPool struct {
//	log    logger.Logger
//	ctx    context.Context
//	cancel context.CancelFunc
//
//	config       protocol.Config      // 配置
//	broadcaster  protocol.Broadcaster // 广播
//	apps         protocol.Apps        // apps服务
//	blockReader  protocol.BlockReader // 数据库读
//	currentState protocol.StateDB
//
//	all     sync.Map // 保存所有的交易
//	pending sync.Map // 当前可执行的交易。types.Address-->*txList
//	queue   sync.Map // 处于队列中，当前不可执行的交易。types.Address-->*txList
// }
//
// func newTxPool(rootCtx context.Context) (*txPool, error) {
//	ctx, cancel := context.WithCancel(rootCtx)
//	return &txPool{
//		log:    logger.New("txPool"),
//		ctx:    ctx,
//		cancel: cancel,
//	}, nil
// }
//
// func (t *txPool) Start() error {
//	return nil
// }
// func (t *txPool) Stop() error {
//	return nil
// }
//
// func (t *txPool) Add(peerId *models.P2PID, tx models.Transaction) error {
//	return nil
// }
//
// func (t *txPool) promoteExecutables(accounts []string) {
//	if accounts == nil {
//		accounts = make([]string, 0)
//		t.queue.Range(func(addr, value interface{}) bool {
//			accounts = append(accounts, addr.(string))
//			return true
//		})
//	}
//
//	// Track the promoted transactions to broadcast them at once
//	var promoted []models.Transaction
//	// Iterate over all accounts and promote any executable transactions
//	for _, addr := range accounts {
//		val, ok := t.queue.Load(addr)
//		if !ok {
//			continue // Just in case someone calls with a non existing account
//		}
//		list := val.(*txList)
//		// Drop all transactions that are deemed too old (low nonce)
//		for _, tx := range list.Forward(t.currentState.GetNonce(addr)) {
//			hash := tx.Hash()
//			t.log.Trace("Removed old queued transaction", "hash", hash)
//			t.all.Delete(hash)
//			//t.priced.Removed()
//		}
//		// Drop all transactions that are too costly (low balance or out of gas)
//		drops, _ := list.Filter(t.currentState.GetBalance(addr), t.currentMaxGas)
//		for _, tx := range drops {
//			hash := tx.Hash()
//			t.log.Trace("Removed unpayable queued transaction", "hash", hash)
//			t.all.Delete(hash)
//			//t.priced.Removed()
//			//queuedNofundsCounter.Inc(1)
//		}
//		// Gather all executable transactions and promote them
//		for _, tx := range list.Ready(t.pendingState.GetNonce(addr)) {
//			hash := tx.Hash()
//			if t.promoteTx(addr, hash, tx) {
//				t.log.Trace("Promoting queued transaction", "hash", hash)
//				promoted = append(promoted, tx)
//			}
//		}
//		// Drop all transactions over the allowed limit
//		if !t.locals.contains(addr) {
//			for _, tx := range list.Cap(int(t.config.AccountQueue)) {
//				hash := tx.Hash()
//				t.all.Delete(hash)
//				t.priced.Removed()
//				t.log.Trace("Removed cap-exceeding queued transaction", "hash", hash)
//			}
//		}
//		// Delete the entire queue entry if it became empty.
//		if list.Empty() {
//			delete(t.queue, addr)
//		}
//	}
//	// Notify subsystem for new promoted transactions.
//	if len(promoted) > 0 {
//		//go t.txFeed.Send(NewTxsEvent{promoted})
//	}
//	// If the pending limit is overflown, start equalizing allowances
//	pending := uint64(0)
//	t.pending.Range(func(key, value interface{}) bool {
//		list := value.(*txList)
//		pending += uint64(list.Len())
//		return true
//	})
//	if pending > t.config.GlobalSlots {
//		pendingBeforeCap := pending
//		// Assemble a spam order to penalize large transactors first
//		spammers := prque.New(nil)
//		t.pending.Range(func(key, value interface{}) bool {
//			list := value.(*txList)
//			// Only evict transactions from high rollers
//			if !t.locals.contains(addr) && uint64(list.Len()) > t.config.AccountSlots {
//				spammers.Push(addr, int64(list.Len()))
//			}
//			return true
//		})
//		// Gradually drop transactions from offenders
//		offenders := []types.Address{}
//		for pending > t.config.GlobalSlots && !spammers.Empty() {
//			// Retrieve the next offender if not local address
//			offender, _ := spammers.Pop()
//			offenders = append(offenders, offender.(types.Address))
//
//			// Equalize balances until all the same or below threshold
//			if len(offenders) > 1 {
//				// Calculate the equalization threshold for all current offenders
//				threshold := t.pending[offender.(types.Address)].Len()
//
//				// Iteratively reduce all offenders until below limit or threshold reached
//				for pending > t.config.GlobalSlots && t.pending[offenders[len(offenders)-2]].Len() > threshold {
//					for i := 0; i < len(offenders)-1; i++ {
//						list := t.pending[offenders[i]]
//						for _, tx := range list.Cap(list.Len() - 1) {
//							// Drop the transaction from the global pools too
//							hash := tx.Hash()
//							t.all.Remove(hash)
//							t.priced.Removed()
//
//							// Update the account nonce to the dropped transaction
//							if nonce := tx.Nonce(); t.pendingState.GetNonce(offenders[i]) > nonce {
//								t.pendingState.SetNonce(offenders[i], nonce)
//							}
//							log.Trace("Removed fairness-exceeding pending transaction", "hash", hash)
//						}
//						pending--
//					}
//				}
//			}
//		}
//		// If still above threshold, reduce to limit or min allowance
//		if pending > t.config.GlobalSlots && len(offenders) > 0 {
//			for pending > t.config.GlobalSlots && uint64(t.pending[offenders[len(offenders)-1]].Len()) > t.config.AccountSlots {
//				for _, addr := range offenders {
//					list := t.pending[addr]
//					for _, tx := range list.Cap(list.Len() - 1) {
//						// Drop the transaction from the global pools too
//						hash := tx.Hash()
//						t.all.Remove(hash)
//						t.priced.Removed()
//
//						// Update the account nonce to the dropped transaction
//						if nonce := tx.Nonce(); t.pendingState.GetNonce(addr) > nonce {
//							t.pendingState.SetNonce(addr, nonce)
//						}
//						log.Trace("Removed fairness-exceeding pending transaction", "hash", hash)
//					}
//					pending--
//				}
//			}
//		}
//		pendingRateLimitCounter.Inc(int64(pendingBeforeCap - pending))
//	}
//	// If we've queued more transactions than the hard limit, drop oldest ones
//	queued := uint64(0)
//	for _, list := range t.queue {
//		queued += uint64(list.Len())
//	}
//	if queued > t.config.GlobalQueue {
//		// Sort all accounts with queued transactions by heartbeat
//		addresses := make(addressesByHeartbeat, 0, len(t.queue))
//		for addr := range t.queue {
//			if !t.locals.contains(addr) { // don't drop locals
//				addresses = append(addresses, addressByHeartbeat{addr, t.beats[addr]})
//			}
//		}
//		sort.Sort(addresses)
//
//		// Drop transactions until the total is below the limit or only locals remain
//		for drop := queued - t.config.GlobalQueue; drop > 0 && len(addresses) > 0; {
//			addr := addresses[len(addresses)-1]
//			val := t.queue[addr.address]
//			list := val.(*txList)
//			addresses = addresses[:len(addresses)-1]
//
//			// Drop all transactions if they are less than the overflow
//			if size := uint64(list.Len()); size <= drop {
//				for _, tx := range list.Flatten() {
//					t.deleteTx(tx, true)
//				}
//				drop -= size
//				continue
//			}
//			// Otherwise drop only last few transactions
//			txs := list.Flatten()
//			for i := len(txs) - 1; i >= 0 && drop > 0; i-- {
//				t.deleteTx(txs[i], true)
//				drop--
//			}
//		}
//	}
// }
//
// // FetchTxs 需排序处理
// func (t *txPool) FetchTxs(txsLimit uint64) models.Transactions {
//	var (
//		data = make([]models.Transaction, 0, txsLimit)
//	)
//	t.pending.Range(func(addr, value interface{}) bool {
//		list := value.(*txList)
//		txs := list.Flatten()
//		for _, tx := range txs.Data() {
//			data = append(data, tx)
//			if len(data) >= int(txsLimit) {
//				return true
//			}
//		}
//		return true
//	})
//	return models.NewTransactions(data)
// }
//
// // Delete 删除交易
// func (t *txPool) Delete(txs []models.Transaction, noErr bool) {
//	for _, tx := range txs {
//		hash := tx.Hash()
//		if txI, ok := t.all.Load(hash); ok {
//			t.deleteTx(txI.(models.Transaction), noErr)
//		}
//	}
// }
//
// func (t txPool) deleteTx(tx models.Transaction, noErr bool) {
//	hash := tx.Hash()
//	if _, ok := t.all.Load(hash); !ok {
//		return
//	}
//	from := tx.From()
//	t.all.Delete(hash)
//	// 判断pending中是否存在
//	if val, ok := t.pending.Load(from); ok {
//		pending := val.(*txList)
//		if removed, invalids := pending.Remove(tx); removed {
//			if pending.Empty() {
//				t.pending.Delete(from)
//			}
//			for _, tx := range invalids.Data() {
//				t.addTx(tx.Hash(), tx)
//			}
//		}
//	}
//
//	if val, ok := t.queue.Load(from); ok {
//		queue := val.(*txList)
//		queue.Remove(tx)
//		if queue.Empty() {
//			t.queue.Delete(from)
//		}
//	}
// }
//
// func (t *txPool) addTx(hash types.Hash, tx models.Transaction) (bool, error) {
//	var queue *txList
//	if value, ok := t.queue.Load(tx.From()); ok {
//		queue = value.(*txList)
//	} else {
//		queue = newTxList(false)
//		t.queue.Store(tx.From(), queue)
//	}
//	inserted, old := queue.Add(tx, 10) // 替换现有交易的百分比，10%
//	if !inserted {
//		return false, ErrReplaceUnderpriced
//	}
//	if old != nil {
//		t.all.Delete(old.Hash())
//	}
//	if _, ok := t.all.Load(hash); !ok {
//		t.all.Store(hash, tx)
//	}
//	return old != nil, nil
// }
//
// func (t *txPool) Fallback(txs []models.Transaction) error {
//	for _, tx := range txs {
//		t.Add(nil, tx)
//	}
//	return nil
// }

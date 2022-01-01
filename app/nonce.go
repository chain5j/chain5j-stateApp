// Package app
//
// @author: xwc1125
package app

import (
	"container/heap"
	"errors"
	"github.com/chain5j/chain5j-pkg/collection/queues/queue"
	"github.com/chain5j/chain5j-pkg/math"
	"github.com/chain5j/chain5j-protocol/models"
	"github.com/chain5j/chain5j-stateApp"
	"github.com/chain5j/logger"
	"math/big"
	"sort"
	"sync"
)

var (
	errNonceTooLow    = errors.New("tx nonce too low")
	errNonceTooHeight = errors.New("nonce too height")
)

// nonceHeap nonce的堆
type nonceHeap []uint64

func (h nonceHeap) Len() int           { return len(h) }
func (h nonceHeap) Less(i, j int) bool { return h[i] < h[j] }
func (h nonceHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *nonceHeap) Push(x interface{}) {
	*h = append(*h, x.(uint64))
}
func (h *nonceHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

type txSortedMap struct {
	items map[uint64]*stateApp.Transaction // nonce-->tx
	index *nonceHeap                       // 存储nonce的堆
	cache models.TransactionSortedList     // 缓存已排序的交易
}

func newTxSortedMap() *txSortedMap {
	return &txSortedMap{
		items: make(map[uint64]*stateApp.Transaction),
		index: new(nonceHeap),
	}
}

// Get 根据nonce获取transaction
func (m *txSortedMap) Get(nonce uint64) *stateApp.Transaction {
	return m.items[nonce]
}

// Put 添加交易
func (m *txSortedMap) Put(tx *stateApp.Transaction) {
	nonce := tx.Nonce()
	if m.items[nonce] == nil {
		heap.Push(m.index, nonce)
	}
	m.items[nonce], m.cache = tx, nil
}

// Forward 获取小于阀值的交易，并将缓存交易删除
func (m *txSortedMap) Forward(threshold uint64) models.TransactionSortedList {
	var removed models.TransactionSortedList

	// 从堆中取出数据，直到阀值
	for m.index.Len() > 0 && (*m.index)[0] < threshold {
		nonce := heap.Pop(m.index).(uint64)
		removed = append(removed, m.items[nonce])
		delete(m.items, nonce)
	}
	if m.cache != nil {
		// 修改cache内容
		m.cache = m.cache[len(removed):]
	}
	return removed
}

// Filter 循环过滤需要被删除的数据。并返回被删除的数据
func (m *txSortedMap) Filter(filter func(models.Transaction) bool) models.TransactionSortedList {
	// 缓存中被删除的数据
	var removed models.TransactionSortedList

	// 回调判断，进行交易处理
	for nonce, tx := range m.items {
		if filter(tx) {
			removed = append(removed, tx)
			delete(m.items, nonce)
		}
	}
	// 有需要被清理的交易
	if removed.Len() > 0 {
		// 堆的数据需要被重新处理
		*m.index = make([]uint64, 0, len(m.items))
		for nonce := range m.items {
			*m.index = append(*m.index, nonce)
		}
		heap.Init(m.index)

		// 将排好序的数据清空
		m.cache = nil
	}
	return removed
}

// Cap 删除超过阀值的交易，并返回被删除的交易
func (m *txSortedMap) Cap(threshold int) models.TransactionSortedList {
	if len(m.items) <= threshold {
		return nil
	}
	var drops models.TransactionSortedList

	// 堆进行排序
	sort.Sort(*m.index)
	// 循环删除大于阀值的数据
	for size := len(m.items); size > threshold; size-- {
		drops = append(drops, m.items[(*m.index)[size-1]])
		delete(m.items, (*m.index)[size-1])
	}
	// 堆中取阀值内的数据
	*m.index = (*m.index)[:threshold]
	heap.Init(m.index)

	// 如果cache不为空，清理后面的数据
	if m.cache != nil {
		m.cache = m.cache[:len(m.cache)-len(drops)]
	}
	return drops
}

// Remove 删除nonce
func (m *txSortedMap) Remove(nonce uint64) bool {
	// 如果nonce不存在，则无需操作
	_, ok := m.items[nonce]
	if !ok {
		return false
	}
	// 否则从堆中删除
	for i := 0; i < m.index.Len(); i++ {
		if (*m.index)[i] == nonce {
			heap.Remove(m.index, i)
			break
		}
	}
	// 集合中删除
	delete(m.items, nonce)
	m.cache = nil

	return true
}

// Ready 从start开始获取递增的交易集合。并将交易从堆中删除
//
// 注意，所有nonce低于start的事务也将返回，以防止进入无效状态。
// 这不是应该发生的事情，但自我纠正比失败要好！
func (m *txSortedMap) Ready(start uint64) models.TransactionSortedList {
	if m.index.Len() == 0 || (*m.index)[0] > start {
		// 如果无交易，或者第一个交易的nonce已经大于start，那么返回空
		return nil
	}
	// 计算获得递增交易
	var ready models.TransactionSortedList
	for next := (*m.index)[0]; m.index.Len() > 0 && (*m.index)[0] == next; next++ {
		ready = append(ready, m.items[next])
		delete(m.items, next)
		heap.Pop(m.index)
	}
	m.cache = nil

	return ready
}

func (m *txSortedMap) Len() int {
	return len(m.items)
}

// Flatten 创建一个有序的交易集合
func (m *txSortedMap) Flatten() models.TransactionSortedList {
	if m.cache == nil {
		m.cache = make(models.TransactionSortedList, 0, len(m.items))
		for _, tx := range m.items {
			m.cache = append(m.cache, tx)
		}
		sort.Sort(m.cache)
	}
	// 复制缓存交易
	txs := make(models.TransactionSortedList, len(m.cache))
	copy(txs, m.cache)
	return txs
}

type txList struct {
	strict bool         // nonce是否严格连续
	txs    *txSortedMap // 交易堆排序

	costCap *big.Int // 最高的price，当超过balance重置
	gasCap  uint64   // 最高的gas，当超过block限制时重置
}

func newTxList(strict bool) *txList {
	return &txList{
		strict:  strict,
		txs:     newTxSortedMap(),
		costCap: new(big.Int),
	}
}

// Overlaps 判断交易是否存在
func (l *txList) Overlaps(tx models.Transaction) bool {
	transaction, ok := tx.(*stateApp.Transaction)
	if !ok {
		return false
	}
	return l.txs.Get(transaction.Nonce()) != nil
}

// Add 增加交易
func (l *txList) Add(tx0 models.Transaction, priceBump uint64) (bool, models.Transaction) {
	tx, ok := tx0.(*stateApp.Transaction)
	if !ok {
		return false, nil
	}
	// 如果缓存已经存在nonce
	old := l.txs.Get(tx.Nonce())
	if old != nil {
		// 计算gasPrice
		threshold := old.GasPrice() * (100 + priceBump) / 100
		// 确保新交易的gasPrice大于旧交易的gasPrice。或者百分比大于旧交易
		if old.GasPrice() >= tx.GasPrice() || threshold > tx.GasPrice() {
			return false, nil
		}
	}

	l.txs.Put(tx)
	// 如果缓存中的cost小于交易的cost，那么更新缓存
	if cost := tx.Cost(); l.costCap.Cmp(cost) < 0 {
		l.costCap = cost
	}
	// 如果缓存中的gas小于交易的gas，更新gas
	if gas := tx.GasLimit(); l.gasCap < gas {
		l.gasCap = gas
	}
	return true, old
}

// Forward 获取小于阀值nonce的交易，并将缓存交易删除
func (l *txList) Forward(threshold uint64) models.TransactionSortedList {
	return l.txs.Forward(threshold)
}

// Filter 从交易列表中删除cost或者gas高于阀值的所有交易。
// 已被删除的交易都会被返回。
// 如果是严格模式，还会删除无效的交易。
func (l *txList) Filter(costLimit *big.Int, gasLimit uint64) (removedTxs models.TransactionSortedList, invalidTxs models.TransactionSortedList) {
	// 如果costLimit大于缓存中的cos，并且gasLimit大于缓存中的gas，那么返回空，表示无数据需要删除
	if l.costCap.Cmp(costLimit) <= 0 && l.gasCap <= gasLimit {
		return nil, nil
	}
	// 表示costLimit较小，修改缓存中的costCap
	l.costCap = new(big.Int).Set(costLimit)
	// 修改缓存中的gasCap
	l.gasCap = gasLimit

	// 过滤删除交易cost大于costLimit的，或者交易gas大于gasLimit的交易
	removed := l.txs.Filter(func(tx models.Transaction) bool {
		transaction := tx.(*stateApp.Transaction)
		return transaction.Cost().Cmp(costLimit) > 0 || transaction.GasLimit() > gasLimit
	})
	var invalids models.TransactionSortedList
	// 如果是严格模式，那么将会删除任何大于被删除交易的最小nonce。
	if l.strict && len(removed) > 0 {
		// 通过循环获取被删除交易中最小的nonce
		lowest := uint64(math.MaxUint64)
		for _, tx := range removed {
			transaction, ok := tx.(*stateApp.Transaction)
			if !ok {
				continue
			}
			if nonce := transaction.Nonce(); lowest > nonce {
				lowest = nonce
			}
		}
		// 过滤删除小于最小nonce的交易
		invalids = l.txs.Filter(func(tx models.Transaction) bool {
			transaction, ok := tx.(*stateApp.Transaction)
			if !ok {
				return false
			}
			return transaction.Nonce() > lowest
		})
	}
	// 被删除的交易，无效的交易
	return removed, invalids
}

// Cap 保留指定数量的交易，删除多余的交易，并将删除的交易返回
func (l *txList) Cap(threshold int) models.TransactionSortedList {
	return l.txs.Cap(threshold)
}

// Remove 删除交易。如果是严格模式，还会返回被过滤的交易
func (l *txList) Remove(txI models.Transaction) (bool, models.TransactionSortedList) {
	tx, ok := txI.(*stateApp.Transaction)
	if !ok {
		return false, nil
	}
	nonce := tx.Nonce()
	if removed := l.txs.Remove(nonce); !removed {
		return false, nil
	}
	// 严格模式，过滤nonce大于被删除的nonce的交易
	if l.strict {
		filter := l.txs.Filter(func(txI models.Transaction) bool {
			tx, ok := txI.(*stateApp.Transaction)
			if !ok {
				return false
			}
			return tx.Nonce() > nonce
		})
		return true, filter
	}
	return true, nil
}

// Ready 从start开始获取递增的交易集合。并将交易从堆中删除
func (l *txList) Ready(start uint64) models.TransactionSortedList {
	return l.txs.Ready(start)
}

func (l *txList) Len() int {
	return l.txs.Len()
}
func (l *txList) Empty() bool {
	return l.Len() == 0
}

// Flatten 创建一个有序的交易集合
func (l *txList) Flatten() models.TransactionSortedList {
	return l.txs.Flatten()
}

type item struct {
	nonce uint64
	value interface{}
}

type Nonce struct {
	log logger.Logger

	nonceCache *sync.Map //address==> *prque.Prque
}

func newNonce() *Nonce {
	return &Nonce{
		log:        logger.New("nonce"),
		nonceCache: new(sync.Map),
	}
}

// DeleteErrTx 删除错误的交易
// 同时需要将错误交易的nonce后的所有缓存交易都得删除
func (a *Nonce) DeleteErrTx(txI models.Transaction) error {
	var (
		tx *stateApp.Transaction
		ok bool
	)
	if tx, ok = txI.(*stateApp.Transaction); !ok {
		a.log.Error("transaction state parse", "err", errTxParse.Error())
		return errTxParse
	}
	a.log.Debug("delete err tx", "hash", tx.Hash(), "nonce", tx.Nonce())
	a.popTopNonce(tx)
	return nil
}

// DeleteOkTx 删除成功的交易
// 需要将成功交易的nonce前的所有缓存交易删除
// 并且调用写入数据库的操作
func (a *Nonce) DeleteOkTx(txI models.Transaction) error {
	var (
		tx *stateApp.Transaction
		ok bool
	)
	if tx, ok = txI.(*stateApp.Transaction); !ok {
		a.log.Error("transaction state parse", "err", errTxParse.Error())
		return errTxParse
	}

	a.log.Debug("DeleteOkTx", "hash", tx.Hash(), "nonce", tx.Nonce())
	a.popBottomNonce(tx.From(), tx.Nonce())
	return nil
}

// popTopNonce 将nonce高的tx从缓存中剔除掉
func (a *Nonce) popTopNonce(tx *stateApp.Transaction) {
	a.log.Trace("popTopNonce", "address", tx.From(), "nonce", tx.Nonce())
	stack, ok := a.nonceCache.Load(tx.From())
	if !ok {
		return
	}
	a.popTopNonceLockFree(stack.(*queue.LinkedQueue), tx.Nonce())
}

func (a *Nonce) popTopNonceLockFree(stack *queue.LinkedQueue, nonce uint64) {
	if stack == nil {
		return
	}
	element := stack.PeekBack()
	if element != nil {
		item := element.(item)
		// priority下一个nonce值
		if item.value != nil && item.nonce > nonce {
			// 需要删除的nonce大于缓存中的，那么需要将缓存中小于等于它的都删除
			stack.PollBack()
			a.popTopNonceLockFree(stack, nonce)
		}
	}
}

// 将nonce低的tx从缓存中剔除掉
func (a *Nonce) popBottomNonce(account string, nonce uint64) {
	stack, ok := a.nonceCache.Load(account)
	if !ok {
		return
	}
	a.log.Trace("pop bottom nonce lock free", "account===>", account, "nonce", nonce)
	a.popBottomNonceLockFree(account, stack.(*queue.LinkedQueue), nonce)
}

func (a *Nonce) popBottomNonceLockFree(account string, stack *queue.LinkedQueue, nonce uint64) {
	if stack == nil || stack.Size() == 1 {
		return
	}

	element := stack.PeekFront()

	if element != nil {
		item := element.(item)
		// priority下一个nonce值
		if item.value != nil && item.nonce < nonce {
			a.log.Trace("pop bottom nonce lock free", "item.nonce", item.nonce, "nonce", nonce)
			stack.PollFront()
			a.popBottomNonceLockFree(account, stack, nonce)
		}
	}
}

// 获取nonce值缓存
func (a *Nonce) getOrNewNonceCache(account string) uint64 {
	cache, ok := a.getNonceCache(account)
	if !ok {
		a.log.Debug("new nonce cache")
	}

	return cache
}

// 获取nonce值缓存
func (a *Nonce) getNonceCache(account string) (uint64, bool) {
	cache, ok := a.nonceCache.Load(account)
	if !ok {
		cache1 := queue.NewLinkedQueue()
		a.nonceCache.Store(account, cache1)
		a.log.Debug("get nonce cache empty", "account", account)
		return 0, false
	}

	itemE := cache.(*queue.LinkedQueue).PeekBack() // 查看最顶部的值（最大）
	if itemE == nil {
		return 0, true
	}
	item := itemE.(item)
	return item.nonce, true
}

// push 将tx存入缓存中
func (a *Nonce) push(tx *stateApp.Transaction) {
	a.log.Debug("push tx to nonce cache", "address", tx.From(), "nonce", tx.Nonce())
	var que *queue.LinkedQueue
	if cache, ok := a.nonceCache.Load(tx.From()); ok {
		que = cache.(*queue.LinkedQueue)
		// 进行查看
		element := que.PeekFront()
		if element != nil {
			item := element.(item)
			if tx.Nonce() < item.nonce {
				return
			}
		}
	} else {
		que = queue.NewLinkedQueue()
	}
	que.PushBack(queue.Element(item{
		nonce: tx.Nonce(),
		value: tx,
	}))

	//bytes, err := models.TxEncode(tx)
	//if err == nil {
	//	// 保存的是交易个数，所以交易中的nonce值需要+1
	//	nextNonce := tx.Nonce()+1
	//
	//	//app.log.Debug("cache.Push", "address", tx.From(), "nonce", tx.Nonce(), "nextNonce", nextNonce)
	//	linkedQueue.PushBack(queue.Element(item{
	//		nonce: nextNonce,
	//		value: bytes,
	//	}))
	//} else {
	//	a.log.Error("push tx to nonce cache", "address", tx.From(), "nonce", tx.Nonce(), "err", err)
	//}
	a.nonceCache.Store(tx.From(), que)
	//app.log.Debug("cache.Push", "address", tx.From(), "result", "Success")
}

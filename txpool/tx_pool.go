// Package txpool
//
// @author: xwc1125
//
// txPool 只交易缓存，将交易的校验放进app中处理
// txPool 只有一个缓存，对交易做强制性限制，nonce必须是递增的，如果中间有断层，那么后续所有交易都会被抛弃。
// txPool 需要对账户进行管理，如nonce值管理等
package txpool

import (
	"context"
	"errors"
	"github.com/allegro/bigcache/v2"
	"github.com/chain5j/chain5j-pkg/collection/lookup"
	"github.com/chain5j/chain5j-pkg/pool/pool"
	"github.com/chain5j/chain5j-pkg/types"
	"github.com/chain5j/chain5j-pkg/util/hexutil"
	"github.com/chain5j/chain5j-protocol/models"
	"github.com/chain5j/chain5j-protocol/protocol"
	"github.com/chain5j/logger"
	"sort"
	"sync"
)

var (
	_ protocol.TxPool = new(txPool)
)

type txPool struct {
	log    logger.Logger
	ctx    context.Context
	cancel context.CancelFunc

	config      protocol.Config      // 配置
	apps        protocol.Apps        // apps服务
	blockReader protocol.BlockReader // 数据库读

	appContexts    protocol.AppContexts // context
	appContextLock sync.RWMutex         // context lock

	txQueue *lookup.Lookup // 缓存所有的hash，服务器规定时间内将会清理未处理的交易
	chPool  *pool.Pool     // 协程池
}

// NewTxPool new txPool
func NewTxPool(rootCtx context.Context, opts ...option) (protocol.TxPool, error) {
	ctx, cancel := context.WithCancel(rootCtx)
	chPool, err := pool.NewPool(10000)
	if err != nil {
		return nil, err
	}

	t := &txPool{
		log:    logger.New("txPool"),
		ctx:    ctx,
		cancel: cancel,

		chPool: chPool,
	}

	if err := apply(t, opts...); err != nil {
		t.log.Error("apply is error", "err", err)
		return nil, err
	}
	t.txQueue = t.initCache()
	return t, nil
}

func (t *txPool) initCache() *lookup.Lookup {
	return lookup.NewLookup(&lookup.PoolConfig{
		MaxTxSize:  t.config.TxPoolConfig().Capacity,
		TxLifeTime: 1,
		TxTaskTime: 10,
	}, t.OnRemoveWithReason)
}

// OnRemoveWithReason 当交易过期或被删除时被调用
func (t *txPool) OnRemoveWithReason(key string, entry []byte, reason bigcache.RemoveReason) {
	go func() {
		tx, err := models.TxDecode(entry)
		if err != nil {
			return
		}
		if reason == bigcache.Expired || reason == bigcache.NoSpace {
			if t.isMetrics(3) {
				t.log.Trace("OnRemoveWithReason", "key", key, "reason", reason)
			}
			if txQueue := t.getTxQueue(); txQueue != nil {
				txQueue.Del(key)
			}
		}
		if reason == bigcache.Expired {
			// 过期
			t.Delete(tx, false)
		} else {
			// 正常删除的交易
			t.Delete(tx, true)
		}
	}()
}

// Start 启动
func (t *txPool) Start() error {
	// 根据当前的block stateRoot获取ctx
	t.log.Debug("block stateRoot", "state_roots", hexutil.Encode(t.blockReader.CurrentBlock().StateRoots()))
	appContexts, err := t.apps.NewAppContexts("txPool", t.blockReader.CurrentBlock().StateRoots())
	if err != nil {
		t.log.Crit("new appContexts error", "error", err)
		return err
	}
	t.appContexts = appContexts
	return nil
}

// Stop 停止
func (t *txPool) Stop() error {
	t.cancel()
	return nil
}

// Add 添加交易
// 如果交易来自rpc，那么peerId为空
func (t *txPool) Add(peerId *models.P2PID, txI models.Transaction) error {
	tx, ok := txI.(models.StateTransaction)
	if !ok {
		return errors.New("tx is not state transaction")
	}
	flag := "Rpc Tx"
	if peerId != nil {
		flag = "P2P Tx"
	}
	// 检测交易是否重复
	txQueue := t.getTxQueue()
	if txQueue == nil {
		if t.isMetrics(2) {
			t.log.Debug(errTxType.Error(), "flag", flag, "hash", "txType", tx.TxType())
		}
		return errTxType
	}
	if txQueue.Exist(tx.Hash().Hex()) {
		if t.isMetrics(2) {
			t.log.Debug("tx is duplicate", "flag", flag, "hash", tx.Hash(), "address", tx.From(), "nonce", tx.Nonce())
		}
		return errTxDuplicate
	}

	// 检测交易池是否已满
	if txQueue.Len() >= t.config.TxPoolConfig().Capacity {
		return errPoolFull
	}

	// 检验交易
	if err := t.ValidateTx(tx); err != nil {
		t.log.Error("validate tx failed", "flag", flag, "hash", tx.Hash(), "address", tx.From(), "nonce", tx.Nonce(), "err", err)
		return err
	}

	// 添加到交易池
	if err := t.addQueue(txQueue, tx); err != nil {
		t.log.Error("add tx to queue err", "hash", tx.Hash(), "err", err)
		return err
	}

	return nil
}

// 将交易添加进队列
func (t *txPool) addQueue(txQueue *lookup.Lookup, tx models.Transaction) error {
	encode, err := models.TxEncode(tx)
	if err != nil {
		return err
	}
	txQueue.Add(tx.Hash().Hex(), encode)
	return nil
}

func (t *txPool) getTxQueue() *lookup.Lookup {
	return t.txQueue
}

// ===========app===========

// ValidateTx 校验交易
func (t *txPool) ValidateTx(tx models.Transaction) error {
	t.appContextLock.Lock()
	defer t.appContextLock.Unlock()

	// 通过交易类型获取app
	app, err := t.apps.App(tx.TxType())
	if err != nil {
		t.log.Error("get app by txType err", "txType", tx.TxType(), "err", err)
		return err
	}
	// 使用app建议交易
	return app.ValidateTx(t.appContexts.Ctx(tx.TxType()), tx)
}

func (t *txPool) setAppContexts() error {
	appContexts, err := t.apps.NewAppContexts("txPool", t.blockReader.CurrentBlock().StateRoots())
	if err != nil {
		t.log.Crit("new appContexts error", "error", err)
		return err
	}
	t.appContexts = appContexts
	return nil
}

// Len 交易池中交易个数
func (t *txPool) Len() uint64 {
	return t.txQueue.Len()
}

// ===========共识调用===========

// FetchTxs 需排序处理
func (t *txPool) FetchTxs(txsLimit uint64, headerTimestamp uint64) []models.Transaction {
	txQueue := t.txQueue
	txQueueLen := txQueue.Len()
	if txQueueLen == 0 {
		return nil
	}
	if txsLimit > txQueueLen {
		txsLimit = txQueueLen
	}

	var data = make([]models.Transaction, 0, txsLimit)
	keys := txQueue.GetAllKeys()
	for _, k := range keys {
		if txsLimit <= uint64(len(data)) {
			break
		}
		bytes, err := txQueue.Get(k)
		if err != nil {
			txQueue.Del(k)
			continue
		}
		tx, err := models.TxDecode(bytes)
		if err != nil {
			txQueue.Del(k)
			continue
		}
		// 对交易进行校验
		err = t.ValidateTx(tx)
		if err != nil {
			t.log.Error("validate tx err", "txType", tx.TxType(), "hash", tx.Hash(), "err", err)
			txQueue.Del(k)
			continue
		}
		data = append(data, tx)
	}
	txs := models.NewTransactionSortedList(data)
	sort.Sort(txs) // sort
	return txs
}

// Delete 删除交易
// 通过订阅blockChain的事件来进行删除交易
// 删除交易池中的交易
// blockChain已经处理了的交易，或者数据库中已经存在的交易
func (t *txPool) Delete(tx models.Transaction, noErr bool) error {
	t.appContextLock.Lock()
	defer t.appContextLock.Unlock()

	if tx == nil {
		return nil
	}
	t.txQueue.Del(tx.Hash().Hex())
	if app, err := t.apps.App(tx.TxType()); err == nil {
		if noErr {
			app.DeleteOkTx(tx)
		} else {
			app.DeleteErrTx(tx)
		}
	}
	return nil
}

func (t *txPool) Fallback(txs []models.Transaction) error {
	for _, tx := range txs {
		t.Add(nil, tx)
	}
	return nil
}

func (t *txPool) Exist(hash types.Hash) bool {
	if val, err := t.txQueue.Get(hash.Hex()); err == nil && len(val) > 0 {
		return true
	}
	return false
}

func (t *txPool) Get(hash types.Hash) (models.Transaction, models.TxStatus) {
	txEnc, err := t.txQueue.Get(hash.Hex())
	if err != nil {
		t.log.Error("get tx by hash err", "err", err)
		return nil, models.TxStatus_Unkown
	}
	tx, err := models.TxDecode(txEnc)
	if err != nil {
		t.log.Error("decode tx err", "err", err)
		return nil, models.TxStatus_Unkown
	}
	return tx, models.TxStatus_Waiting
}

func (t *txPool) GetTxs(txsLimit uint64) []models.Transaction {
	var data = make([]models.Transaction, 0, txsLimit)
	keys := t.txQueue.GetAllKeys()
	for _, k := range keys {
		if txsLimit <= uint64(len(data)) {
			break
		}
		bytes, err := t.txQueue.Get(k)
		if err != nil {
			t.txQueue.Del(k)
			continue
		}
		tx, err := models.TxDecode(bytes)
		if err != nil {
			t.txQueue.Del(k)
			continue
		}
		// 对交易进行校验
		err = t.ValidateTx(tx)
		if err != nil {
			t.log.Error("validate tx err", "txType", tx.TxType(), "hash", tx.Hash(), "err", err)
			t.txQueue.Del(k)
			continue
		}
		data = append(data, tx)
	}
	txs := models.NewTransactionSortedList(data)
	sort.Sort(txs) // sort
	return txs
}

func (t *txPool) isMetrics(metrics uint64) bool {
	return t.config.TxPoolConfig().IsMetrics(metrics)
}

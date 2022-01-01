// Package app
//
// @author: xwc1125
package app

import (
	"context"
	"errors"
	"fmt"
	"github.com/chain5j/chain5j-pkg/database/kvstore"
	"github.com/chain5j/chain5j-pkg/types"
	"github.com/chain5j/chain5j-pkg/util/dateutil"
	"github.com/chain5j/chain5j-protocol/models"
	"github.com/chain5j/chain5j-protocol/models/statetype"
	"github.com/chain5j/chain5j-protocol/models/vm"
	"github.com/chain5j/chain5j-protocol/pkg/database/ethStatedb"
	"github.com/chain5j/chain5j-protocol/pkg/database/statedb"
	"github.com/chain5j/chain5j-protocol/protocol"
	"github.com/chain5j/chain5j-stateApp"
	"github.com/chain5j/chain5j-stateApp/txpool"
	"github.com/chain5j/logger"
	"sync"
	"time"
)

var (
	_ protocol.Application = new(application)
)

type application struct {
	log     logger.Logger
	rootCtx context.Context

	config  protocol.Config
	db      protocol.Database
	blockRW protocol.BlockReadWriter
	kvDB    kvstore.Database // todo
	txPool  protocol.TxPool
	nodeKey protocol.NodeKey
	apis    protocol.APIs

	nonce       *Nonce
	useEthereum bool // 是否采用以太坊地址模型

	commitLock sync.RWMutex
}

func NewApplication(rootCtx context.Context, opts ...option) (protocol.Application, error) {
	a := &application{
		log:     logger.New("stateApp"),
		rootCtx: rootCtx,
	}
	if err := apply(a, opts...); err != nil {
		a.log.Error("apply is error", "err", err)
		return nil, err
	}
	initInterpreter(a.nodeKey)

	a.useEthereum = a.config.ChainConfig().StateApp.UseEthereum
	a.nonce = newNonce()
	a.apis.RegisterAPI([]protocol.API{
		{
			Namespace: "apps",
			Version:   "1.0",
			Service:   a.newAPI(),
			Public:    true,
		},
	})
	return a, nil
}

func (a *application) Start() error {
	return nil
}

func (a *application) Stop() error {
	if a.txPool != nil {
		return a.txPool.Stop()
	}
	return nil
}

func (a *application) NewAppContexts(caller string, args ...interface{}) (protocol.AppContext, error) {
	if len(args) == 0 {
		return nil, errors.New("invalid args")
	}
	var root types.Hash
	switch param := args[0].(type) {
	case []byte:
		root = types.BytesToHash(param)
	case types.Hash:
		root = param
	default:
		return nil, fmt.Errorf("invaild arg type")
	}
	a.log.Trace("New context", "caller", caller, "root", root)

	context := &stateContext{
		caller:      caller,
		app:         a,
		useEthereum: a.useEthereum,
	}

	if a.useEthereum {
		stateDB, err := ethStatedb.New(root, ethStatedb.NewDatabase(a.kvDB))
		if err != nil {
			return nil, err
		}
		context.ethState = stateDB
	} else {
		stateDB, err := statedb.New(root, a.kvDB)
		if err != nil {
			return nil, err
		}
		context.stateDB = stateDB
	}

	return context, nil
}

func (a *application) TxPool(config protocol.Config, apps protocol.Apps, blockReader protocol.BlockReader, broadcaster protocol.Broadcaster) (protocol.TxPool, error) {
	if a.txPool != nil {
		return a.txPool, nil
	}
	txPool, err := txpool.NewTxPool(a.rootCtx,
		txpool.WithConfig(config),
		txpool.WithApps(apps),
		txpool.WithBlockReader(blockReader),
	)
	if err != nil {
		return nil, err
	}
	a.txPool = txPool
	if err := a.txPool.Start(); err != nil {
		return nil, err
	}
	return a.txPool, nil
}

// ValidateTx
// 校验交易
// 维护nonce
// nonce 值在缓存中的维护，如果缓存中已经存在nonce大的，则丢弃
// 如果nonce值比较小的失败了，那么nonce值大的都得失败
func (a *application) ValidateTx(ctx protocol.AppContext, txI models.Transaction) error {
	t := time.Now()
	context := ctx.(*stateContext)
	tx, ok := txI.(*stateApp.Transaction)
	if !ok {
		return fmt.Errorf("invaild tx. hash=%s", txI.Hash())
	}

	if tx.Size() > a.config.TxSizeLimit() {
		return errTxLooLarge
	}

	if _, ok := interpreters[tx.Interpreter()]; !ok {
		return errInvalidInterpreter
	} else {
		if a.checkInterpreters(tx.Interpreter()) != nil {
			return errInvalidInterpreter
		}
	}

	// check nonce
	// stateNonce是数据库中的nonce值，如果数据库开始没有时，那么返回的是0，代表已经存在的个数
	// nonceCache是缓存中的nonce值，如果缓存开始没有时，那么返回的是0，代表已经存在的个数
	// 如果是严格递增，那么tx.Nonce只能等于stateNonce或nonceCache。tx.Nonce代表当前已拥有的交易个数
	// 因此，nonceCache在进行存储时，需要
	var nonce uint64      // transaction count
	var stateNonce uint64 // nonce in db,transaction count

	if stateNonce = context.getNonce(tx.From()); tx.Nonce() < stateNonce {
		a.log.Debug("validate tx nonce err", "nonceDB", nonce, "txNonce", tx.Nonce())
		return errNonceTooLow
	}
	nonce = stateNonce

	nonceCache := a.nonce.getOrNewNonceCache(tx.From()) // 查看最大的nonce值

	a.log.Debug("validate tx nonce end", "address", tx.From(), "stateNonce", stateNonce, "cacheNonce", nonceCache, "txNonce", tx.Nonce())
	if nonceCache > nonce {
		nonce = nonceCache
	}
	// todo 此处需要处理
	if tx.Nonce() != nonce {
		a.log.Debug("ValidateTxNonce err", "address", tx.From(), "stateNonce", stateNonce, "cacheNonce", nonceCache, "txNonce", tx.Nonce())
		return errNonceTooHeight
	}

	header := a.blockRW.CurrentBlock().Header()
	interpreterCtx, err := stateApp.NewInterpreterCtx(context.stateDB, context.ethState, context.preRoot, header, a.blockRW, tx.GasLimit(), a.config)
	if err != nil {
		return errors.New("init interpreter context error")
	}

	if err := interpreters[tx.Interpreter()].VerifyTx(interpreterCtx, tx); err != nil {
		return err
	}

	a.nonce.push(tx)

	a.log.Debug("validate tx end", "elapsed", dateutil.PrettyDuration(time.Since(t)))
	return nil
}

func (a *application) ValidateTxSafe(ctx protocol.AppContext, txI models.Transaction, headerTimestamp uint64) error {
	tx, ok := txI.(*stateApp.Transaction)
	if !ok {
		return fmt.Errorf("invaild tx. hash=%s", txI.Hash())
	}
	_ = tx
	return nil
}

func (a *application) GetCacheNonce(ctx protocol.AppContext, account string) (uint64, error) {
	return a.nonce.getOrNewNonceCache(account), nil
}

// Prepare 预处理，计算出stateRoot
func (a *application) Prepare(ctx protocol.AppContext, header *models.Header, txs models.TransactionSortedList, totalGas uint64) *models.TxsStatus {
	context := ctx.(*stateContext)
	t := time.Now()
	var (
		tx *stateApp.Transaction
		ok = false
	)

	interpreterCtx, err := stateApp.NewInterpreterCtx(context.stateDB, context.ethState, context.preRoot, header, a.blockRW, totalGas, a.config)
	if err != nil {
		return nil
	}

	// 计算gasUsed
	var (
		receipts []*statetype.Receipt
		tcount   int
		usedGas  = new(uint64)
		okTxs    []models.Transaction
		errTxs   []models.Transaction
	)

	// 计算gasUsed
	for _, txI := range txs {
		if tx, ok = txI.(*stateApp.Transaction); !ok {
			errTxs = append(errTxs, tx)
			a.log.Error("transaction state parse", "err", errTxParse.Error())
			continue
		}

		if totalGas < *usedGas {
			break
		}

		if err := interpreters[tx.Interpreter()].VerifyTx(interpreterCtx, tx); err != nil {
			errTxs = append(errTxs, tx)
			a.log.Error("Interpreter VerifyTx err", "err", err)
			continue
		}

		// Prepare时获取不到当前区块的hash，需要在commit是更新logs中的区块hash.
		interpreterCtx.Prepare(tx.Hash(), types.Hash{}, tcount)
		snap := interpreterCtx.Snapshot()
		receipt, err := interpreters[tx.Interpreter()].ApplyTransaction(interpreterCtx, tx, usedGas)
		if err != nil {
			if err == vm.ErrGasLimitReached {
				break
			}

			a.log.Error("Prepare ApplyTransaction", "err", err)
			continue
		}

		if err != nil {
			interpreterCtx.RevertToSnapshot(snap)
			continue
		}

		okTxs = append(okTxs, txI)
		tcount++
		if receipt != nil {
			receipts = append(receipts, receipt)
		}

		a.nonce.push(tx)

		if receipt != nil && (receipt.ContractAddress != types.Address{}) {
			a.log.Trace("deploy contract", "ContractAddress", receipt.ContractAddress)
		}
	}

	root := context.intermediateRoot()

	context.addReceipt(receipts)

	a.log.Debug("Prepare Elapsed", "elapsed", dateutil.PrettyDuration(time.Since(t)), "count", txs.Len())
	return &models.TxsStatus{
		StateRoots: root.Bytes(),
		GasUsed:    *usedGas,
		OkTxs:      okTxs,
		ErrTxs:     errTxs,
	}
}

// Commit 提交入库
func (a *application) Commit(ctx protocol.AppContext, header *models.Header) error {
	context := ctx.(*stateContext)

	t := time.Now()
	a.commitLock.Lock()
	defer a.commitLock.Unlock()

	root, err := context.commit()
	if err != nil {
		a.log.Error("Commit State", "err", err)
		return err
	}
	a.log.Debug("app stateRoot", "root", root.Hex())

	receipts := context.receipts
	for i, r := range receipts {
		updateReceipt(header, r)
		if i == 0 {
			receipts[i].CumulativeGasUsed = receipts[i].GasUsed
		} else {
			receipts[i].GasUsed = receipts[i].CumulativeGasUsed - receipts[i-1].CumulativeGasUsed
		}
	}

	a.storeReceipts(header, context.receipts)

	a.log.Debug("Commit Elapsed", "elapsed", dateutil.PrettyDuration(time.Since(t)))
	return err
}

func updateReceipt(header *models.Header, receipt *statetype.Receipt) {
	for _, l := range receipt.Logs {
		l.BlockHeight = header.Height
		l.BlockHash = header.Hash()
	}
}

func (a *application) storeReceipts(header *models.Header, receipts []*statetype.Receipt) {
	a.db.WriteReceipts(header.Hash(), header.Height, receipts)
}

func (a *application) checkInterpreters(interpreter string) error {
	if a.useEthereum {
		if interpreter != stateApp.EthereumInterpreter {
			return errInvalidInterpreter
		}
	} else {
		if interpreter == stateApp.EthereumInterpreter {
			return errInvalidInterpreter
		}
	}

	return nil
}

// DeleteErrTx 删除错误的交易
// 同时需要将错误交易的nonce后的所有缓存交易都得删除
func (a *application) DeleteErrTx(txI models.Transaction) error {
	return a.nonce.DeleteErrTx(txI)
}

// DeleteOkTx 删除成功的交易
// 需要将成功交易的nonce前的所有缓存交易删除
// 并且调用写入数据库的操作
func (a *application) DeleteOkTx(txI models.Transaction) error {
	return a.nonce.DeleteOkTx(txI)
}

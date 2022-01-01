// Package baseInterpreter
//
// @author: xwc1125
package baseInterpreter

import (
	"fmt"
	"github.com/chain5j/chain5j-pkg/util/dateutil"
	"github.com/chain5j/chain5j-protocol/models"
	"github.com/chain5j/chain5j-protocol/models/statetype"
	"github.com/chain5j/chain5j-protocol/pkg/database/statedb"
	"github.com/chain5j/chain5j-stateApp"
	"github.com/chain5j/logger"
	"math/big"
	"time"
)

type BaseInterpreter struct {
	log logger.Logger
}

func NewInterpreter() *BaseInterpreter {
	return &BaseInterpreter{
		log: logger.New("base_interpreter"),
	}
}

func (base *BaseInterpreter) VerifyTx(ctx stateApp.InterpreterCtx, tx models.StateTransaction) error {
	t := time.Now()
	stateDB := ctx.StateDB()

	signer, err := tx.Signer()
	if err != nil {
		return err
	}
	accountFrom := stateDB.GetAccount(tx.From())
	// 账户未找到
	if accountFrom == nil {
		return stateApp.ErrFromAccountNotFound
	}
	if accountFrom.IsFrozen {
		return stateApp.ErrFrozenAccount
	}

	if !accountFrom.ContainAddress(signer) {
		return stateApp.ErrInvalidSigner
	}

	accountTo := stateDB.GetAccount(tx.To())
	// 账户未找到
	if accountTo == nil {
		return stateApp.ErrToAccountNotFound
	}

	// check balance
	balance := ctx.StateDB().GetBalance(tx.From())
	if balance.Cmp(tx.Cost()) < 0 {
		return stateApp.ErrBalanceNotEnough
	}

	base.log.Debug("VerifyTx Elapsed", "elapsed", dateutil.PrettyDuration(time.Since(t)))
	return nil
}

func (base *BaseInterpreter) ApplyTransaction(ctx stateApp.InterpreterCtx, tx models.StateTransaction, usedGas *uint64) (*statetype.Receipt, error) {
	// 计算gasUsed
	//gasUsed := uint64(21000)
	stateDB := ctx.StateDB()

	*usedGas += uint64(21000)

	receipt := &statetype.Receipt{
		Status:            1,
		CumulativeGasUsed: *usedGas,
		TransactionHash:   tx.Hash(),
		GasUsed:           uint64(21000),
		Logs:              nil,
	}

	// 写状态
	err := base.writeState(stateDB, tx)
	if err != nil {
		return nil, err
	}
	return receipt, nil
}

func (base *BaseInterpreter) writeState(stateDB *statedb.StateDB, tx models.StateTransaction) error {
	account := tx.From()

	currentNonce := stateDB.GetNonce(account)
	if currentNonce != tx.Nonce() {
		base.log.Info("write state error")
		return fmt.Errorf("stateDB nonce and tx nonce is diff,stateDB nonce = %d, txNonce = %d", currentNonce, tx.Nonce())
	}
	stateDB.SetNonce(account, currentNonce+1)
	if tx.Value().Cmp(big.NewInt(0)) > 0 {
		stateDB.SubBalance(account, tx.Value())
		if tx.To() != "" {
			stateDB.AddBalance(tx.To(), tx.Value())
		}
	}
	return nil
}

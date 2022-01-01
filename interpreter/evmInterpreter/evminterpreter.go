// Package evmInterpreter
//
// @author: xwc1125
package evmInterpreter

import (
	"errors"
	"github.com/chain5j/chain5j-protocol/models"
	"github.com/chain5j/chain5j-protocol/models/statetype"
	"github.com/chain5j/chain5j-stateApp"
	"github.com/chain5j/logger"
)

var (
	errInvalidContract = errors.New("invalid contract")
)

type Interpreter struct {
	log logger.Logger
}

func NewInterpreter() *Interpreter {
	return &Interpreter{
		log: logger.New("evm_interpreter"),
	}
}

func (i *Interpreter) VerifyTx(ctx stateApp.InterpreterCtx, tx models.StateTransaction) error {
	stateDB := ctx.StateDB()

	signer, err := tx.Signer()
	if err != nil {
		i.log.Error("[VerifyTx] tx.Signer is err", "err", err)
		return err
	}

	accountFrom := stateDB.GetAccount(tx.From())
	// 账户未找到
	if accountFrom == nil {
		i.log.Error("[VerifyTx] stateDB.GetAccount err", "from", tx.From(), "err", stateApp.ErrFromAccountNotFound)
		return stateApp.ErrFromAccountNotFound
	}
	if accountFrom.IsFrozen {
		i.log.Error("[VerifyTx] accountFrom err", "from", tx.From(), "err", stateApp.ErrFrozenAccount)
		return stateApp.ErrFrozenAccount
	}

	if !accountFrom.ContainAddress(signer) {
		i.log.Error("[VerifyTx] accountFrom is not contain signer", "from", tx.From(), "err", stateApp.ErrInvalidSigner)
		return stateApp.ErrInvalidSigner
	}

	if tx.To() != "" {
		accountTo := stateDB.GetAccount(tx.To())
		// 账户未找到
		if accountTo == nil {
			i.log.Error("[VerifyTx] to address is not exist", "to", tx.To(), "err", stateApp.ErrToAccountNotFound)
			return stateApp.ErrToAccountNotFound
		}
	}

	return nil
}

func (i *Interpreter) ApplyTransaction(ctx stateApp.InterpreterCtx, tx models.StateTransaction, usedGas *uint64) (*statetype.Receipt, error) {
	conf := ctx.ChainConfig()

	// TODO 这里用 ctx.BlockReadWriter.CurrentBlock().Header() 并不太恰当，应该是当前处理的区块，而不是已经存储的区块。
	receipt, _, err := i.applyTransaction(&conf, ctx.BlockReadWriter(), ctx.Header(), tx, ctx.StateDB(), ctx.GasPool(), usedGas)
	if err != nil {
		i.log.Error("[ApplyTransaction] applyTransaction err", "err", err)
	}
	return receipt, err
}

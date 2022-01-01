// Package ethereumInterpreter
//
// @author: xwc1125
package ethereumInterpreter

import (
	"errors"
	"github.com/chain5j/chain5j-pkg/types"
	"github.com/chain5j/chain5j-protocol/models"
	"github.com/chain5j/chain5j-protocol/models/statetype"
	"github.com/chain5j/chain5j-stateApp"
	"github.com/chain5j/logger"
)

var (
	errBalanceNotEnough = errors.New("balance not enough")
)

type Interpreter struct {
	log logger.Logger
}

func NewInterpreter() *Interpreter {
	return &Interpreter{
		log: logger.New("ethereum_interpreter"),
	}
}

func (i *Interpreter) VerifyTx(ctx stateApp.InterpreterCtx, tx models.StateTransaction) error {
	stateDB := ctx.EthStateDB()

	signer, err := tx.Signer()
	if err != nil {
		return err
	}

	if types.HexToAddress(tx.From()) != signer {
		return stateApp.ErrInvalidSigner
	}

	// check balance
	balance := stateDB.GetBalance(signer)
	if balance.Cmp(tx.Cost()) < 0 {
		return errBalanceNotEnough
	}

	return nil
}

func (i *Interpreter) ApplyTransaction(ctx stateApp.InterpreterCtx, tx models.StateTransaction, usedGas *uint64) (*statetype.Receipt, error) {
	conf := ctx.ChainConfig()

	// TODO 这里用 ctx.BlockReadWriter.CurrentBlock().Header() 并不太恰当，应该是当前处理的区块，而不是已经存储的区块。
	receipt, _, err := applyTransaction(&conf, ctx.BlockReadWriter(), ctx.Header(), tx, ctx.EthStateDB(), ctx.GasPool(), usedGas)
	return receipt, err
}

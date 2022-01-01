// Package caInterpreter
//
// @author: xwc1125
package caInterpreter

import (
	"github.com/chain5j/chain5j-protocol/models"
	"github.com/chain5j/chain5j-protocol/models/statetype"
	"github.com/chain5j/chain5j-stateApp"
	"github.com/chain5j/logger"
)

type Interpreter struct {
	log logger.Logger
}

func NewInterpreter() *Interpreter {
	return &Interpreter{
		log: logger.New("ca_interpreter"),
	}
}

func (i *Interpreter) VerifyTx(ctx stateApp.InterpreterCtx, tx models.StateTransaction) error {
	return nil
}

func (i *Interpreter) ApplyTransaction(ctx stateApp.InterpreterCtx, tx models.StateTransaction, usedGas *uint64) (*statetype.Receipt, error) {
	return nil, nil
}

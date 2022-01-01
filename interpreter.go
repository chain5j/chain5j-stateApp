// Package stateApp
//
// @author: xwc1125
package stateApp

import (
	"github.com/chain5j/chain5j-pkg/types"
	"github.com/chain5j/chain5j-protocol/models"
	"github.com/chain5j/chain5j-protocol/models/statetype"
	"github.com/chain5j/chain5j-protocol/models/vm"
	"github.com/chain5j/chain5j-protocol/pkg/database/ethStatedb"
	"github.com/chain5j/chain5j-protocol/pkg/database/statedb"
	"github.com/chain5j/chain5j-protocol/protocol"
)

const (
	BaseInterpreter       = "chain5j.base"
	AccountInterpreter    = "chain5j.account"
	LostInterpreter       = "chain5j.lost"
	EvmInterpreter        = "chain5j.evm"
	CAInterpreter         = "chain5j.ca"
	EthereumInterpreter   = "chain5j.ethereum"
	PermissionInterpreter = "chain5j.permission"
)

type InterpreterContext struct {
	stateDB       *statedb.StateDB
	ethStateDB    *ethStatedb.StateDB
	preRoot       types.Hash
	blockRW       protocol.BlockReadWriter
	config        protocol.Config
	currentHeader *models.Header
	gasPool       *vm.GasPool
}

func NewInterpreterCtx(chain5jState *statedb.StateDB, ethState *ethStatedb.StateDB, preRoot types.Hash,
	header *models.Header, blockRW protocol.BlockReadWriter, totalGas uint64, config protocol.Config) (*InterpreterContext, error) {
	gasPool := new(vm.GasPool).AddGas(totalGas)

	return &InterpreterContext{
		stateDB:       chain5jState,
		ethStateDB:    ethState,
		preRoot:       preRoot,
		blockRW:       blockRW,
		config:        config,
		currentHeader: header,
		gasPool:       gasPool,
	}, nil
}

func (ctx *InterpreterContext) StateDB() *statedb.StateDB {
	return ctx.stateDB
}

func (ctx *InterpreterContext) EthStateDB() *ethStatedb.StateDB {
	return ctx.ethStateDB
}

func (ctx *InterpreterContext) BlockReadWriter() protocol.BlockReadWriter {
	return ctx.blockRW
}

func (ctx *InterpreterContext) ChainConfig() models.ChainConfig {
	return ctx.config.ChainConfig()
}

func (ctx *InterpreterContext) GasPool() *vm.GasPool {
	return ctx.gasPool
}

func (ctx *InterpreterContext) Header() *models.Header {
	return ctx.currentHeader
}

func (ctx *InterpreterContext) Prepare(thash, bhash types.Hash, tcount int) {
	if ctx.stateDB != nil {
		ctx.stateDB.Prepare(thash, bhash, tcount)
	}

	if ctx.ethStateDB != nil {
		ctx.ethStateDB.Prepare(thash, bhash, tcount)
	}
}

func (ctx *InterpreterContext) Snapshot() int {
	if ctx.stateDB != nil {
		return ctx.stateDB.Snapshot()
	}

	if ctx.ethStateDB != nil {
		return ctx.ethStateDB.Snapshot()
	}

	return 0
}

func (ctx *InterpreterContext) RevertToSnapshot(snap int) {
	if ctx.stateDB != nil {
		ctx.stateDB.RevertToSnapshot(snap)
	}

	if ctx.ethStateDB != nil {
		ctx.ethStateDB.RevertToSnapshot(snap)
	}
}

type Interpreter interface {
	VerifyTx(ctx InterpreterCtx, tx models.StateTransaction) error
	ApplyTransaction(ctx InterpreterCtx, tx models.StateTransaction, usedGas *uint64) (*statetype.Receipt, error)
}

type InterpreterCtx interface {
	Prepare(thash, bhash types.Hash, tcount int)
	Snapshot() int
	RevertToSnapshot(snap int)

	StateDB() *statedb.StateDB
	EthStateDB() *ethStatedb.StateDB
	BlockReadWriter() protocol.BlockReadWriter
	ChainConfig() models.ChainConfig
	GasPool() *vm.GasPool
	Header() *models.Header
}

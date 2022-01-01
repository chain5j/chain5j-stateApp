// Package app
//
// @author: xwc1125
package app

import (
	"github.com/chain5j/chain5j-pkg/types"
	"github.com/chain5j/chain5j-protocol/models/statetype"
	"github.com/chain5j/chain5j-protocol/pkg/database/ethStatedb"
	"github.com/chain5j/chain5j-protocol/pkg/database/statedb"
	"github.com/chain5j/chain5j-protocol/protocol"
)

var (
	_ protocol.AppContext = new(stateContext)
)

type stateContext struct {
	caller      string
	app         *application
	useEthereum bool
	stateDB     *statedb.StateDB
	ethState    *ethStatedb.StateDB
	preRoot     types.Hash

	receipts []*statetype.Receipt
}

func (ctx *stateContext) Caller() string {
	return ctx.caller
}

func (ctx *stateContext) App() protocol.Application {
	return ctx.app
}

func (ctx *stateContext) addReceipt(receipts []*statetype.Receipt) {
	ctx.receipts = append(ctx.receipts, receipts...)
}

func (ctx *stateContext) getNonce(account string) uint64 {
	if ctx.useEthereum {
		return ctx.ethState.GetNonce(types.HexToAddress(account))
	} else {
		return ctx.stateDB.GetNonce(account)
	}

	return 0
}

func (ctx *stateContext) intermediateRoot() types.Hash {
	if ctx.useEthereum {
		return ctx.ethState.IntermediateRoot(false)
	} else {
		return ctx.stateDB.IntermediateRoot(false)
	}

	return types.Hash{}
}

func (ctx *stateContext) commit() (types.Hash, error) {
	if ctx.useEthereum {
		root, err := ctx.ethState.Commit(false)
		if err == nil {
			ctx.ethState.Database().TreeDB().Commit(root, true)
		}
		return root, err
	} else {
		return ctx.stateDB.Commit(false)
	}

	return types.Hash{}, nil
}

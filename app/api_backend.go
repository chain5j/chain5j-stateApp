// Package app
//
// @author: xwc1125
package app

import (
	"context"
	"errors"
	evm "github.com/chain5j/chain5j-evm"
	"github.com/chain5j/chain5j-pkg/codec"
	"github.com/chain5j/chain5j-pkg/database/kvstore"
	"github.com/chain5j/chain5j-pkg/math"
	"github.com/chain5j/chain5j-pkg/network/rpc"
	"github.com/chain5j/chain5j-pkg/types"
	"github.com/chain5j/chain5j-protocol/models"
	"github.com/chain5j/chain5j-protocol/models/statetype"
	"github.com/chain5j/chain5j-protocol/pkg/database/ethStatedb"
	"github.com/chain5j/chain5j-protocol/pkg/database/statedb"
	"github.com/chain5j/chain5j-protocol/protocol"
	"github.com/chain5j/chain5j-stateApp/interpreter/evmInterpreter"
)

type ApiBackend struct {
	config      protocol.Config
	blockchain  protocol.BlockReader
	db          kvstore.Database
	useEthereum bool
}

func newApiBackend(config protocol.Config, reader protocol.BlockReader, db kvstore.Database, useEth bool) *ApiBackend {
	return &ApiBackend{
		config:      config,
		blockchain:  reader,
		db:          db,
		useEthereum: useEth,
	}
}

func (b *ApiBackend) StateAt(root types.Hash) (interface{}, error) {
	if b.useEthereum {
		return ethStatedb.New(root, ethStatedb.NewDatabase(b.db))
	}
	return statedb.New(root, b.db)
}

func (b *ApiBackend) StateAndHeaderByNumber(ctx context.Context, number rpc.BlockNumber) (interface{}, *models.Header, error) {
	// Otherwise resolve and return the block
	var header *models.Header
	if number == rpc.LatestBlockNumber || number == rpc.PendingBlockNumber {
		header = b.blockchain.CurrentBlock().Header()
	} else {
		header = b.blockchain.GetHeaderByNumber(uint64(number))
	}
	if header == nil {
		return nil, nil, errors.New("header not found")
	}

	roots := statetype.NewRoots()
	codec.Coder().Decode(header.StateRoots, roots)

	stateDb, err := b.StateAt(roots.GetObj("STATE"))
	return stateDb, header, err
}

func (b *ApiBackend) StateAndHeaderByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (interface{}, *models.Header, error) {
	if blockNr, ok := blockNrOrHash.Number(); ok {
		return b.StateAndHeaderByNumber(ctx, blockNr)
	}
	if hash, ok := blockNrOrHash.Hash(); ok {
		header := b.blockchain.GetHeaderByHash(hash)
		if header == nil {
			return nil, nil, errors.New("header for hash not found")
		}
		if blockNrOrHash.RequireCanonical && b.blockchain.GetHeaderByNumber(header.Height).Hash() != hash {
			return nil, nil, errors.New("hash is not currently canonical")
		}

		roots := statetype.NewRoots()
		codec.Coder().Decode(header.StateRoots, roots)

		stateDb, err := b.StateAt(roots.GetObj("STATE"))
		return stateDb, header, err
	}
	return nil, nil, errors.New("invalid arguments; neither block nor hash specified")
}

func (b *ApiBackend) RootsAndHeaderByNumber(ctx context.Context, number rpc.BlockNumber) (*statetype.StateRoots, error) {
	// Otherwise resolve and return the block
	var header *models.Header
	if number == rpc.LatestBlockNumber || number == rpc.PendingBlockNumber {
		header = b.blockchain.CurrentBlock().Header()
	} else {
		header = b.blockchain.GetHeaderByNumber(uint64(number))
	}
	if header == nil {
		return nil, errors.New("header not found")
	}

	roots := statetype.NewRoots()
	err := codec.Coder().Decode(header.StateRoots, roots)

	return roots, err
}

func (b *ApiBackend) RootsAndHeaderByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*statetype.StateRoots, error) {
	if blockNr, ok := blockNrOrHash.Number(); ok {
		return b.RootsAndHeaderByNumber(ctx, blockNr)
	}
	if hash, ok := blockNrOrHash.Hash(); ok {
		header := b.blockchain.GetHeaderByHash(hash)
		if header == nil {
			return nil, errors.New("header for hash not found")
		}
		if blockNrOrHash.RequireCanonical && b.blockchain.GetHeaderByNumber(header.Height).Hash() != hash {
			return nil, errors.New("hash is not currently canonical")
		}

		roots := statetype.NewRoots()
		err := codec.Coder().Decode(header.StateRoots, roots)

		return roots, err
	}
	return nil, errors.New("invalid arguments; neither block nor hash specified")
}

func (b *ApiBackend) GetEVM(ctx context.Context, msg models.VmMessage, state *statedb.StateDB, header *models.Header) (protocol.VM, func() error, error) {
	state.SetBalance(msg.From(), math.MaxBig256)
	vmError := func() error { return nil }
	evmdb := statedb.NewEvmStateDB(state)

	context := evmInterpreter.NewEVMContext(msg, header, b.blockchain, nil)
	return evm.NewEVM(context, evmdb, nil, evm.Config{}), vmError, nil
}

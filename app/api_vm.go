// Package app
//
// @author: xwc1125
package app

import (
	"context"
	"github.com/chain5j/chain5j-pkg/math"
	"github.com/chain5j/chain5j-pkg/network/rpc"
	"github.com/chain5j/chain5j-pkg/types"
	"github.com/chain5j/chain5j-pkg/util/hexutil"
	"github.com/chain5j/chain5j-protocol/models"
	"github.com/chain5j/chain5j-protocol/models/vm"
	"github.com/chain5j/chain5j-protocol/pkg/database/statedb"
	"github.com/chain5j/chain5j-stateApp/interpreter/evmInterpreter"
	"github.com/chain5j/logger"
	"time"
)

func (api *API) Call(ctx context.Context, args models.Message, blockNr rpc.BlockNumber) (hexutil.Bytes, error) {
	result, _, _, err := api.doCall(ctx, args, blockNr, 5*time.Second)
	return (hexutil.Bytes)(result), err
}

func (api *API) doCall(ctx context.Context, args models.Message, blockNr rpc.BlockNumber, timeout time.Duration) ([]byte, uint64, bool, error) {
	defer func(start time.Time) { logger.Trace("Executing EVM call finished", "runtime", time.Since(start)) }(time.Now())

	db, header, err := api.backend.StateAndHeaderByNumber(ctx, blockNr)
	stateDB := db.(*statedb.StateDB)
	if stateDB == nil || err != nil {
		return nil, 0, false, err
	}

	// Set default gas & gas price if none were set
	gas, gasPrice := uint64(args.GasLimit()), args.GasPrice()
	if gas == 0 {
		gas = math.MaxUint64 / 2
	}

	// Setup context so it may be cancelled the call has completed
	// or, in case of unmetered gas, setup a context with a timeout.
	var cancel context.CancelFunc
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout)
	} else {
		ctx, cancel = context.WithCancel(ctx)
	}
	// Make sure the context is cancelled when the call has completed
	// this makes sure resources are cleaned up.
	defer cancel()
	var to *types.Address
	if args.To() != "" {
		toAddress := types.HexToAddress(args.To())
		to = &toAddress
	}
	msg := models.NewEvmMessage(types.HexToAddress(args.From()), to, 0, args.Value(), gas, gasPrice, args.Input(), false)
	// Get a new instance of the EVM.
	evm, vmError, err := api.backend.GetEVM(ctx, msg, stateDB, header)
	if err != nil {
		return nil, 0, false, err
	}
	// Wait for the context to be done and cancel the evm. Even if the
	// EVM has finished, cancelling may be done (repeatedly)
	go func() {
		<-ctx.Done()
		evm.Cancel()
	}()

	// Setup the gas pool (also for unmetered requests)
	// and apply the message.
	gp := new(vm.GasPool).AddGas(math.MaxUint64)
	res, gas, failed, err := evmInterpreter.ApplyMessage(evm, msg, gp)
	if err := vmError(); err != nil {
		return nil, 0, false, err
	}
	return res, gas, failed, err
}

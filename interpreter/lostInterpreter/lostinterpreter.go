// Package lostInterpreter
//
// @author: xwc1125
package lostInterpreter

import (
	"errors"
	"github.com/chain5j/chain5j-pkg/codec"
	"github.com/chain5j/chain5j-pkg/types"
	"github.com/chain5j/chain5j-pkg/util/dateutil"
	"github.com/chain5j/chain5j-protocol/models"
	"github.com/chain5j/chain5j-protocol/models/accounts"
	"github.com/chain5j/chain5j-protocol/models/statetype"
	"github.com/chain5j/chain5j-protocol/pkg/database/statedb"
	"github.com/chain5j/chain5j-stateApp"
	"github.com/chain5j/logger"
)

var (
	errUnauthorized = errors.New("unauthorized")
)

var (
	lostFoundInterval = 60 * 3600 * 24 * 2 // 两天
	//lostFoundInterval = 10
)

type LostInterpreter struct {
	log logger.Logger
}

func NewInterpreter() *LostInterpreter {
	return &LostInterpreter{
		log: logger.New("lost_interpreter"),
	}
}

func (interpreter *LostInterpreter) VerifyTx(ctx stateApp.InterpreterCtx, tx models.StateTransaction) error {
	stateDB := ctx.StateDB()

	accountFrom := stateDB.GetAccount(tx.From())
	// 账户未找到
	if accountFrom == nil {
		return stateApp.ErrFromAccountNotFound
	}
	if accountFrom.IsFrozen {
		return stateApp.ErrFrozenAccount
	}

	// 检查签名
	signer, err := tx.Signer()
	if err != nil {
		return stateApp.ErrInvalidSigner
	}

	var txData accounts.AccountOpData
	if err := accounts.DecodeAccountOpData(tx.Input(), &txData); err != nil {
		return err
	}

	switch txData.Operation {
	case accounts.LostRequestOp:
		if err := checkSigner(accountFrom, signer); err != nil {
			return stateApp.ErrInvalidSigner
		}

		var lostRequest accounts.LostRequest
		if err := codec.Coder().Decode(txData.Data, &lostRequest); err != nil {
			return err
		}
		lostRequest.Normalize()

		if err := VerifyLostRequest(stateDB, tx.From(), &lostRequest); err != nil {
			return err
		}

	case accounts.FoundRequestOp:
		if err := VerifyFoundRequest(stateDB, ctx, tx.From(), signer); err != nil {
			return err
		}

	case accounts.LostResetOp:
		if err := checkSigner(accountFrom, signer); err != nil {
			return stateApp.ErrInvalidSigner
		}

	default:
		return stateApp.ErrInvalidAccountOp
	}

	return nil
}

func (interpreter *LostInterpreter) ApplyTransaction(ctx stateApp.InterpreterCtx, tx models.StateTransaction, usedGas *uint64) (*statetype.Receipt, error) {
	stateDB := ctx.StateDB()

	if err := interpreter.VerifyTx(ctx, tx); err != nil {
		return nil, err
	}

	var txData accounts.AccountOpData
	accounts.DecodeAccountOpData(tx.Input(), &txData)

	switch txData.Operation {
	case accounts.LostRequestOp:
		var lostRequest accounts.LostRequest
		if err := codec.Coder().Decode(txData.Data, &lostRequest); err != nil {
			return nil, err
		}
		lostRequest.Normalize()

		if err := CommitLostRequest(ctx, stateDB, &lostRequest); err != nil {
			return nil, err
		}

	case accounts.FoundRequestOp:
		CommitFoundRequest(stateDB, tx.From())

	case accounts.LostResetOp:
		CommitLostReset(stateDB, tx.From())
	default:
		return nil, stateApp.ErrInvalidAccountOp
	}

	*usedGas += uint64(21000)

	account := tx.From()
	stateDB.SetNonce(account, stateDB.GetNonce(account)+1)

	receipt := &statetype.Receipt{
		Status:            1,
		CumulativeGasUsed: *usedGas,
		TransactionHash:   tx.Hash(),
		GasUsed:           uint64(21000),
		Logs:              nil,
	}
	return receipt, nil
}

func VerifyLostRequest(state *statedb.StateDB, accountFrom string, req *accounts.LostRequest) error {
	lostAccountName := req.CN + "@" + req.Domain

	lostAccount := state.GetAccount(lostAccountName)
	if lostAccount == nil {
		return stateApp.ErrToAccountNotFound
	}

	if accountFrom != lostAccount.Partner() {
		return errUnauthorized
	}

	return nil
}

func CommitLostRequest(ctx stateApp.InterpreterCtx, state *statedb.StateDB, req *accounts.LostRequest) error {
	lostAccountName := req.CN + "@" + req.Domain

	//interval := 3600 * 24 * 2
	store := &accounts.LostStore{
		LostRequest: req,
		TimeStamp:   ctx.Header().Timestamp + uint64(lostFoundInterval),
	}

	state.SetLost(lostAccountName, store)
	return nil
}

func VerifyFoundRequest(state *statedb.StateDB, ctx stateApp.InterpreterCtx, account string, signer types.Address) error {
	now := uint64(dateutil.CurrentTime())

	lostAccount := state.GetAccount(account)
	if lostAccount == nil {
		return stateApp.ErrToAccountNotFound
	}

	var lostStore accounts.LostStore
	lostStoreBytes, ok := lostAccount.XXX[accounts.LostKey]
	if !ok {
		return errUnauthorized
	}

	codec.Coder().Decode(lostStoreBytes, &lostStore)

	if lostStore.RecoverAddr != signer {
		return errUnauthorized
	}

	if now < lostStore.TimeStamp {
		return errUnauthorized
	}

	return nil
}

func CommitFoundRequest(state *statedb.StateDB, account string) error {

	lostAccount := state.GetAccount(account)
	if lostAccount == nil {
		return stateApp.ErrToAccountNotFound
	}

	var lostStore accounts.LostStore
	lostStoreBytes := lostAccount.XXX[accounts.LostKey]

	codec.Coder().Decode(lostStoreBytes, &lostStore)

	state.SetAddress(account, lostStore.RecoverAddr)
	state.SetLost(account, nil)

	return nil
}

func CommitLostReset(state *statedb.StateDB, account string) error {
	state.SetLost(account, nil)

	return nil
}

func checkSigner(store *accounts.AccountStore, signer types.Address) error {

	if !store.ContainAddress(signer) {
		return stateApp.ErrInvalidSigner
	}

	return nil
}

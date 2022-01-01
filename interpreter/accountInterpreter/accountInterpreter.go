// Package accountInterpreter
//
// @author: xwc1125
package accountInterpreter

import (
	"errors"
	"github.com/chain5j/chain5j-pkg/codec"
	"github.com/chain5j/chain5j-pkg/types"
	"github.com/chain5j/chain5j-protocol/models"
	"github.com/chain5j/chain5j-protocol/models/accounts"
	"github.com/chain5j/chain5j-protocol/models/statetype"
	"github.com/chain5j/chain5j-stateApp"
	"github.com/chain5j/logger"
)

var (
	errInvalidInput = errors.New("invalid account operation input")
)

type AccountInterpreter struct {
	log logger.Logger
}

func NewInterpreter() *AccountInterpreter {
	return &AccountInterpreter{
		log: logger.New("account_interpreter"),
	}
}

func (interpreter *AccountInterpreter) VerifyTx(ctx stateApp.InterpreterCtx, tx models.StateTransaction) error {
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
	if err := checkSigner(accountFrom, signer); err != nil {
		return stateApp.ErrInvalidSigner
	}

	var txData accounts.AccountOpData
	if err := accounts.DecodeAccountOpData(tx.Input(), &txData); err != nil {
		return err
	}

	switch txData.Operation {
	case accounts.RegisterAccountOp:
		var accountRegister accounts.AccountStore

		if err := codec.Coder().Decode(txData.Data, &accountRegister); err != nil {
			return err
		}

		accountRegister.Normalize()
		if accountRegister.AccountName() != tx.To() {
			return errInvalidInput
		}

		if err := VerifyAccountRegister(stateDB, accountFrom, &accountRegister); err != nil {
			return err
		}

	case accounts.RegisterDomainOp:
		var accountRegister accounts.AccountStore

		if err := codec.Coder().Decode(txData.Data, &accountRegister); err != nil {
			return err
		}
		accountRegister.Normalize()

		if err := VerifyRegisterDomain(stateDB, accountFrom, &accountRegister); err != nil {
			return err
		}

	case accounts.FrozenAccountOp:
		if err := VerifyAccountFrozenOp(stateDB, accountFrom, txData.Data); err != nil {
			return err
		}

	case accounts.UpdateDataPermissionOp:
		if err := VerifyUpdateDataPermissionOp(stateDB, accountFrom, txData.Data); err != nil {
			return err
		}

	case accounts.SetPartnerOp:
		if err := VerifySetPartnerOp(stateDB, accountFrom, txData.Data); err != nil {
			return err
		}
		//case TODO:
	default:
		return stateApp.ErrInvalidAccountOp
	}

	return nil
}

func (interpreter *AccountInterpreter) ApplyTransaction(ctx stateApp.InterpreterCtx, tx models.StateTransaction, usedGas *uint64) (*statetype.Receipt, error) {
	stateDB := ctx.StateDB()

	if err := interpreter.VerifyTx(ctx, tx); err != nil {
		return nil, err
	}

	var txData accounts.AccountOpData
	accounts.DecodeAccountOpData(tx.Input(), &txData)

	switch txData.Operation {
	case accounts.RegisterAccountOp:
		var accountRegister accounts.AccountStore

		if err := codec.Coder().Decode(txData.Data, &accountRegister); err != nil {
			return nil, err
		}

		accountRegister.Normalize()
		RegisterUser(stateDB, &accountRegister)

	case accounts.RegisterDomainOp:
		var accountRegister accounts.AccountStore

		if err := codec.Coder().Decode(txData.Data, &accountRegister); err != nil {
			return nil, err
		}
		accountRegister.Normalize()
		RegisterDomain(stateDB, &accountRegister, ctx.BlockReadWriter().CurrentBlock().Height()+1)

	case accounts.FrozenAccountOp:
		FrozenAccount(stateDB, txData.Data)

	case accounts.UpdateDataPermissionOp:
		UpdatePermission(stateDB, txData.Data)

	case accounts.SetPartnerOp:
		SetPartner(stateDB, tx.From(), txData.Data)
		//case TODO:
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

func checkSigner(store *accounts.AccountStore, signer types.Address) error {

	if !store.ContainAddress(signer) {
		return stateApp.ErrInvalidSigner
	}

	return nil
}

// Package accountInterpreter
//
// @author: xwc1125
package accountInterpreter

import (
	"errors"
	"github.com/chain5j/chain5j-pkg/codec"
	"github.com/chain5j/chain5j-protocol/models/accounts"
	"github.com/chain5j/chain5j-protocol/pkg/database/statedb"
)

var (
	errAccountNonExists = errors.New("account not exists")
)

func VerifyAccountFrozenOp(state *statedb.StateDB, accountFrom *accounts.AccountStore, input []byte) error {
	var frozenData accounts.FrozenAccountData
	err := codec.Coder().Decode(input, &frozenData)
	if err != nil {
		return errInvalidInput
	}
	frozenData.Normalize()

	// 检查权限
	if !accountFrom.IsAdmin {
		return errUnauthorized
	}

	accountToName := frozenData.CN + "@" + frozenData.Domain
	accountTo := state.GetAccount(accountToName)
	if accountTo == nil {
		return errAccountNonExists
	}

	if accountFrom.Domain == accountTo.Domain {
		if accountTo.IsAdmin {
			return errUnauthorized
		}
	} else if isSubDomain(accountFrom.Domain, frozenData.Domain) {
		return nil
	} else {
		return errUnauthorized
	}

	return nil
}

func FrozenAccount(state *statedb.StateDB, input []byte) error {
	var frozenData accounts.FrozenAccountData
	err := codec.Coder().Decode(input, &frozenData)
	if err != nil {
		return errInvalidInput
	}
	frozenData.Normalize()

	accountTo := frozenData.CN + "@" + frozenData.Domain
	if frozenData.Frozen {
		state.FrozenAccount(accountTo)
	} else {
		state.UnFrozenAccount(accountTo)
	}

	return nil
}

// Package accountInterpreter
//
// @author: xwc1125
package accountInterpreter

import (
	"github.com/chain5j/chain5j-pkg/codec"
	"github.com/chain5j/chain5j-protocol/models/accounts"
	"github.com/chain5j/chain5j-protocol/pkg/database/statedb"
)

// IsValidPermission 注册、修改用户的权限不能高于调用者权限
func IsValidPermission(from, to *accounts.Permissions) bool {
	if !from.EnableRegisterUser && to.EnableRegisterUser {
		return false
	}

	if !from.EnableUpdateUser && to.EnableUpdateUser {
		return false
	}

	if !from.EnableFrozenUser && to.EnableFrozenUser {
		return false
	}

	if !from.EnableRegisterSubdomain && to.EnableRegisterSubdomain {
		return false
	}

	return true
}

func VerifyUpdateDataPermissionOp(state *statedb.StateDB, accountFrom *accounts.AccountStore, input []byte) error {
	var pData accounts.UpdatePermissionData
	err := codec.Coder().Decode(input, &pData)
	if err != nil {
		return errInvalidInput
	}
	pData.Normalize()

	// 检查权限
	if !accountFrom.IsAdmin {
		return errUnauthorized
	}

	accountToName := pData.CN + "@" + pData.Domain
	accountTo := state.GetAccount(accountToName)
	if accountTo == nil {
		return errAccountNonExists
	}

	if !isSubDomain(accountFrom.Domain, pData.Domain) || !accountTo.IsAdmin {
		return errUnauthorized
	}

	if !IsValidPermission(accountFrom.Permissions, &pData.Permissions) {
		return errUnauthorized
	}

	return nil
}

func UpdatePermission(state *statedb.StateDB, input []byte) error {
	var pData accounts.UpdatePermissionData
	err := codec.Coder().Decode(input, &pData)
	if err != nil {
		return errInvalidInput
	}
	pData.Normalize()

	accountTo := pData.CN + "@" + pData.Domain
	state.UpdatePermission(accountTo, &pData.Permissions)

	return nil
}

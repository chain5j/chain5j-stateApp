// Package accountInterpreter
//
// @author: xwc1125
package accountInterpreter

import (
	"errors"
	"github.com/chain5j/chain5j-pkg/math"
	"github.com/chain5j/chain5j-protocol/models/accounts"
	"github.com/chain5j/chain5j-protocol/pkg/database/statedb"
	"unicode"
)

var (
	errAccountExists            = errors.New("account already exists")
	errAddressExists            = errors.New("address already exists")
	errUnauthorized             = errors.New("unauthroized")
	errInvalidBalance           = errors.New("register account balance must be zero")
	errInvalidNonce             = errors.New("register account nonce must be zero")
	errInvalidAccountNameLen    = errors.New("invalid account name length")
	errInvalidAccountNameFormat = errors.New("invalid account name format")
	errInvalidDomainLen         = errors.New("invalid domain length")
	errInvalidDomainFormat      = errors.New("invalid domain format")
	errInvalidDomain            = errors.New("invalid domain")
	errInvalidPermission        = errors.New("invalid permission")
	errInvalidAdminField        = errors.New("can't register admin user")
	errDomainAlreadyExist       = errors.New("domain already exists")
)

// VerifyRegisterDomain TODO
func VerifyRegisterDomain(state *statedb.StateDB, accountFrom, accountRegister *accounts.AccountStore) error {
	enableDomain := accountFrom.Domain != ""
	if err := verifyAccountFormat(accountRegister, enableDomain); err != nil {
		return err
	}

	// 注册用户balance 必须为0
	if accountRegister.Balance.Cmp(math.Big0) != 0 {
		return errInvalidBalance
	}

	// 注册用户nonce 必须为0
	if accountRegister.Nonce != 0 {
		return errInvalidBalance
	}

	if err := checkAddress(state, accountRegister); err != nil {
		return err
	}

	// 拥有注册权限
	if !accountFrom.AuthorizedRegisterDomain() {
		return errUnauthorized
	}

	if state.GetDomain(accountRegister.Domain) != nil {
		return errDomainAlreadyExist
	}

	return nil
}

// VerifyAccountRegister TODO
func VerifyAccountRegister(state *statedb.StateDB, accountFrom, accountRegister *accounts.AccountStore) error {
	enableDomain := accountFrom.Domain != ""
	if err := verifyAccountFormat(accountRegister, enableDomain); err != nil {
		return err
	}

	// 注册用户balance 必须为0
	if accountRegister.Balance.Cmp(math.Big0) != 0 {
		return errInvalidBalance
	}

	// 注册用户nonce 必须为0
	if accountRegister.Nonce != 0 {
		return errInvalidBalance
	}

	if err := checkAddress(state, accountRegister); err != nil {
		return err
	}

	// 拥有注册权限
	if !accountFrom.AuthorizedRegisterUser() {
		return errUnauthorized
	}
	if !accountFrom.EnableDeployContract && accountRegister.EnableDeployContract {
		return errUnauthorized
	}

	// 域名检查
	if accountFrom.Domain == accountRegister.Domain {
		// 不能注册同级管理员
		if accountRegister.IsAdmin {
			return errInvalidAdminField
		}
		// 普通用户没有权限设置
		if accountRegister.Permissions != nil {
			return errInvalidPermission
		}
	} else if isSubDomain(accountFrom.Domain, accountRegister.Domain) {
		if accountRegister.IsAdmin {
			// 注册子域管理员
			if !accountFrom.Permissions.EnableRegisterSubdomain {
				return errUnauthorized
			}
			if accountRegister.Permissions == nil || !IsValidPermission(accountFrom.Permissions, accountRegister.Permissions) {
				return errInvalidPermission
			}
			if accountRegister.Permissions.EnableRegisterDomain {
				return errInvalidPermission
			}
		} else {
			// 注册子域用户
			if accountRegister.Permissions != nil {
				return errInvalidPermission
			}
		}
	} else {
		return errInvalidDomain
	}

	accountName := accountRegister.AccountName()
	store := state.GetAccount(accountName)
	if store != nil {
		return errAccountExists
	}

	return nil
}

// RegisterUser 注册用户
func RegisterUser(state *statedb.StateDB, accountRegister *accounts.AccountStore) error {
	state.CreateAccount(accountRegister)

	return nil
}

// RegisterDomain 注册域
func RegisterDomain(state *statedb.StateDB, accountRegister *accounts.AccountStore, number uint64) error {
	// 更新权限
	accountRegister.Permissions = &accounts.Permissions{
		EnableRegisterUser:      true,
		EnableUpdateUser:        true,
		EnableFrozenUser:        true,
		EnableRegisterDomain:    false,
		EnableRegisterSubdomain: true,
	}
	accountRegister.IsAdmin = true
	accountRegister.EnableDeployContract = true
	accountRegister.IsFrozen = false

	state.CreateAccount(accountRegister)

	state.AddDomain(accountRegister.Domain, accounts.DomainStore{
		Admin:  accountRegister.CN,
		Number: number,
	})

	return nil
}

func checkAddress(state *statedb.StateDB, accountRegister *accounts.AccountStore) error {
	for addr := range accountRegister.Addresses {
		if state.AddressExist(addr) {
			return errAddressExists
		}
	}

	return nil
}

func verifyAccountFormat(account *accounts.AccountStore, enableDomain bool) error {
	if len(account.CN) > MaxAccountNameLen || len(account.CN) < MinAccountNameLen {
		return errInvalidAccountNameLen
	}

	if enableDomain {
		if len(account.Domain) > MaxDomainLen || len(account.Domain) < MinDomainLen {
			return errInvalidDomainLen
		}
	} else {
		if account.Domain != "" {
			return errInvalidDomainFormat
		}
	}

	for _, v := range account.CN {
		if unicode.IsLetter(v) || unicode.IsDigit(v) || v == '_' {
			continue
		} else {
			return errInvalidAccountNameFormat
		}
	}

	if enableDomain {
		for _, v := range account.Domain {
			if unicode.IsLetter(v) || unicode.IsDigit(v) || v == '.' {
				continue
			} else {
				return errInvalidAccountNameFormat
			}
		}
	}

	return nil
}

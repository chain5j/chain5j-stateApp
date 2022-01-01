// Package accountInterpreter
//
// @author: xwc1125
package accountInterpreter

import (
	"github.com/chain5j/chain5j-pkg/codec"
	"github.com/chain5j/chain5j-protocol/models/accounts"
	"github.com/chain5j/chain5j-protocol/pkg/database/statedb"
)

func VerifySetPartnerOp(state *statedb.StateDB, accountFrom *accounts.AccountStore, input []byte) error {
	var data accounts.PartnerData
	err := codec.Coder().Decode(input, &data)
	if err != nil {
		return errInvalidInput
	}
	data.Normalize()

	if accountFrom.Domain == data.Domain || isSubDomain(data.Domain, accountFrom.Domain) || data.CN == "" {
		return nil
	} else {
		return errInvalidDomain
	}

	return nil
}

func SetPartner(state *statedb.StateDB, account string, input []byte) error {
	var data accounts.PartnerData
	codec.Coder().Decode(input, &data)
	data.Normalize()

	state.SetPartner(account, data)

	return nil
}

// Package app
//
// @author: xwc1125
package app

import (
	"context"
	"errors"
	"github.com/chain5j/chain5j-pkg/codec"
	"github.com/chain5j/chain5j-pkg/network/rpc"
	"github.com/chain5j/chain5j-pkg/types"
	"github.com/chain5j/chain5j-pkg/util/hexutil"
	"github.com/chain5j/chain5j-protocol/models/accounts"
	"github.com/chain5j/chain5j-protocol/pkg/database/ethStatedb"
	"github.com/chain5j/chain5j-protocol/pkg/database/statedb"
	"strings"
)

type API struct {
	app     *application
	backend *ApiBackend
}

func (a *application) newAPI() *API {
	return &API{
		app:     a,
		backend: newApiBackend(a.config, a.blockRW, a.kvDB, a.useEthereum),
	}
}

func (api *API) GetBalance(ctx context.Context, account string, blockNrOrHash rpc.BlockNumberOrHash) (*hexutil.Big, error) {
	db, _, err := api.backend.StateAndHeaderByNumberOrHash(ctx, blockNrOrHash)
	if db == nil || err != nil {
		return nil, err
	}

	if api.app.useEthereum {
		state := db.(*ethStatedb.StateDB)
		return (*hexutil.Big)(state.GetBalance(types.HexToAddress(account))), state.Error()
	} else {
		state := db.(*statedb.StateDB)
		return (*hexutil.Big)(state.GetBalance(strings.ToLower(account))), state.Error()
	}

	return nil, nil
}

func (api *API) GetTransactionCount(ctx context.Context, account string, blockNrOrHash rpc.BlockNumberOrHash) (uint64, error) {
	db, _, err := api.backend.StateAndHeaderByNumberOrHash(ctx, blockNrOrHash)
	if db == nil || err != nil {
		return 0, err
	}

	roots, err := api.backend.RootsAndHeaderByNumberOrHash(ctx, blockNrOrHash)
	if err != nil {
		return 0, err
	}

	ctxs, err := api.app.NewAppContexts("state_app_api", roots.GetObj("STATE"))
	if err != nil {
		return 0, err
	}
	var nonce uint64
	if api.app.useEthereum {
		state := db.(*ethStatedb.StateDB)
		nonce = state.GetNonce(types.HexToAddress(account))
	} else {
		state := db.(*statedb.StateDB)
		nonce = state.GetNonce(strings.ToLower(account))
	}

	if blockNr, ok := blockNrOrHash.Number(); ok && blockNr == rpc.PendingBlockNumber {
		nonceCache, err := api.app.GetCacheNonce(ctxs, account)
		if err != nil {
			return 0, err
		}
		if nonceCache > nonce {
			nonce = nonceCache
		}
		return nonce, nil
	}

	return nonce, nil
}

type AccountAPI struct {
	app     *application
	backend *ApiBackend
}

func (a *application) newAccountAPI() *AccountAPI {
	return &AccountAPI{
		app:     a,
		backend: newApiBackend(a.config, a.blockRW, a.kvDB, a.useEthereum),
	}
}

func (api *AccountAPI) AccountInfo(ctx context.Context, account string, blockNrOrHash rpc.BlockNumberOrHash) (*accounts.AccountStore, error) {
	if api.app.useEthereum {
		return nil, nil
	}

	db, _, err := api.backend.StateAndHeaderByNumberOrHash(ctx, blockNrOrHash)
	if db == nil || err != nil {
		return nil, err
	}
	state := db.(*statedb.StateDB)

	store := state.GetAccount(strings.ToLower(account))
	if store == nil {
		return nil, errors.New("account not exists")
	}

	return store, nil
}

func (api *AccountAPI) Partner(ctx context.Context, account string, blockNrOrHash rpc.BlockNumberOrHash) (*accounts.PartnerData, error) {
	if api.app.useEthereum {
		return nil, nil
	}

	db, _, err := api.backend.StateAndHeaderByNumberOrHash(ctx, blockNrOrHash)
	if db == nil || err != nil {
		return nil, err
	}
	state := db.(*statedb.StateDB)

	store := state.GetAccount(strings.ToLower(account))
	if store == nil {
		return nil, errors.New("account not exists")
	}

	pdata, ok := store.XXX[accounts.PartnerKey]
	if !ok {
		return nil, nil
	}

	var partner accounts.PartnerData
	codec.Coder().Decode(pdata, &partner)

	return &partner, nil
}

func (api *AccountAPI) DomainInfo(ctx context.Context, domain string, blockNrOrHash rpc.BlockNumberOrHash) (*accounts.DomainStore, error) {
	if api.app.useEthereum {
		return nil, nil
	}

	db, _, err := api.backend.StateAndHeaderByNumberOrHash(ctx, blockNrOrHash)
	if db == nil || err != nil {
		return nil, err
	}
	state := db.(*statedb.StateDB)

	store := state.GetDomain(strings.ToLower(domain))
	if store == nil {
		return nil, errors.New("domain not exists")
	}

	return store, nil
}

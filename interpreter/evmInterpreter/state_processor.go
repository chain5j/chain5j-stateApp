// Package evmInterpreter
//
// @author: xwc1125
package evmInterpreter

import (
	"encoding/json"
	evm "github.com/chain5j/chain5j-evm"
	"github.com/chain5j/chain5j-pkg/types"
	"github.com/chain5j/chain5j-protocol/models"
	"github.com/chain5j/chain5j-protocol/models/statetype"
	"github.com/chain5j/chain5j-protocol/models/vm"
	"github.com/chain5j/chain5j-protocol/pkg/crypto"
	"github.com/chain5j/chain5j-protocol/pkg/database/statedb"
	"github.com/chain5j/chain5j-protocol/protocol"
	"github.com/chain5j/chain5j-stateApp"
	"math/big"
)

func (i *Interpreter) TxAsMessage(db *statedb.StateDB, tx models.StateTransaction) (models.VmMessage, error) {
	var (
		to   *types.Address
		from types.Address
	)

	from, err := tx.Signer()
	if err != nil {
		i.log.Error("[TxAsMessage] tx.Signer is err", "err", err)
		return nil, err
	}

	if tx.To() != "" {
		account := db.GetAccount(tx.To())
		if account == nil {
			i.log.Error("[TxAsMessage] to address is not exist", "to", tx.To(), "err", stateApp.ErrToAccountNotFound)
			return nil, stateApp.ErrToAccountNotFound
		}

		if account.IsContract() {
			addr := types.HexToAddress(account.CN)
			to = &addr
		} else {
			i.log.Error("[TxAsMessage] to address is not contract", "to", tx.To(), "err", errInvalidContract)
			return nil, errInvalidContract
		}
	}

	return models.NewEvmMessage(from, to, tx.Nonce(), tx.Value(), tx.GasLimit(), new(big.Int).SetUint64(tx.GasPrice()), tx.Input(), true), nil
}

func (i *Interpreter) applyTransaction(config *models.ChainConfig, bc protocol.ChainContext, header *models.Header, tx models.StateTransaction, sdb *statedb.StateDB, gp *vm.GasPool, usedGas *uint64) (*statetype.Receipt, uint64, error) {
	msg, err := i.TxAsMessage(sdb, tx)
	if err != nil {
		return nil, 0, err
	}

	evmdb := statedb.NewEvmStateDB(sdb)

	context := NewEVMContext(msg, header, bc, nil)
	// Create a new environment which holds all relevant information
	// about the transaction and calling mechanisms.
	vmenv := evm.NewEVM(context, evmdb, config, evm.Config{DisableCreate: true})
	_, gas, failed, err := ApplyMessage(vmenv, msg, gp)

	if err != nil {
		i.log.Error("[applyTransaction] ApplyMessage is err", "err", err)
		return nil, 0, err
	}

	// Update the stateApp with pending changes
	evmdb.Finalise(true)

	*usedGas += gas

	// Create a new receipt for the transaction, storing the intermediate root and gas used by the tx
	// based on the eip phase, we're passing whether the root touch-delete accounts.
	receipt := statetype.NewReceipt(failed, *usedGas)
	receipt.TransactionHash = tx.Hash()
	receipt.GasUsed = gas
	// if the transaction created a contract, store the creation address in the receipt.
	if msg.To() == "" {
		receipt.ContractAddress = crypto.CreateAddress(vmenv.Context.Origin, tx.Nonce())
		i.log.Info("[Contract] deploy contract", "ContractAddress", receipt.ContractAddress)
	}
	// Set the receipt logs and create a bloom for filtering
	receipt.Logs = sdb.GetLogs(tx.Hash())
	receipt.LogsBloom = statetype.CreateBloom(statetype.Receipts{receipt})

	receiptStr, _ := json.Marshal(receipt)
	i.log.Trace("[applyTransaction] receipt", "receipt", string(receiptStr))

	return receipt, gas, err
}

// Package ethereumInterpreter
//
// @author: xwc1125
package ethereumInterpreter

import (
	"encoding/json"
	evm "github.com/chain5j/chain5j-evm"
	"github.com/chain5j/chain5j-protocol/models"
	"github.com/chain5j/chain5j-protocol/models/statetype"
	"github.com/chain5j/chain5j-protocol/models/vm"
	"github.com/chain5j/chain5j-protocol/pkg/crypto"
	"github.com/chain5j/chain5j-protocol/pkg/database/ethStatedb"
	"github.com/chain5j/chain5j-protocol/protocol"
	"github.com/chain5j/logger"
)

func applyTransaction(config *models.ChainConfig, bc protocol.ChainContext, header *models.Header, tx models.StateTransaction, sdb *ethStatedb.StateDB, gp *vm.GasPool, usedGas *uint64) (*statetype.Receipt, uint64, error) {
	msg, err := models.TxAsVmMessage(tx)
	if err != nil {
		return nil, 0, err
	}

	context := NewEVMContext(msg, header, bc, nil)
	// Create a new environment which holds all relevant information
	// about the transaction and calling mechanisms.
	vmenv := evm.NewEVM(context, sdb, config, evm.Config{DisableCreate: true})
	_, gas, failed, err := ApplyMessage(vmenv, msg, gp)

	if err != nil {
		return nil, 0, err
	}

	// Update the state with pending changes
	sdb.Finalise(true)

	*usedGas += gas

	// Create a new receipt for the transaction, storing the intermediate root and gas used by the tx
	// based on the eip phase, we're passing whether the root touch-delete accounts.
	receipt := statetype.NewReceipt(failed, *usedGas)
	receipt.TransactionHash = tx.Hash()
	receipt.GasUsed = gas
	// if the transaction created a contract, store the creation address in the receipt.
	if msg.To() == "" {
		receipt.ContractAddress = crypto.CreateAddress(vmenv.Context.Origin, tx.Nonce())
		logger.Info("deploy contract", "ContractAddress", receipt.ContractAddress)
	}
	// Set the receipt logs and create a bloom for filtering
	receipt.Logs = sdb.GetLogs(tx.Hash())
	receipt.LogsBloom = statetype.CreateBloom(statetype.Receipts{receipt})

	receiptStr, _ := json.Marshal(receipt)
	logger.Trace("receipt", "receipt", string(receiptStr))

	return receipt, gas, err
}

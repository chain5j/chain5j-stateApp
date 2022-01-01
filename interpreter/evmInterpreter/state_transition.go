// Package evmInterpreter
//
// @author: xwc1125
package evmInterpreter

import (
	"errors"
	evm "github.com/chain5j/chain5j-evm"
	"github.com/chain5j/chain5j-pkg/math"
	"github.com/chain5j/chain5j-pkg/types"
	"github.com/chain5j/chain5j-protocol/models"
	"github.com/chain5j/chain5j-protocol/models/vm"
	"github.com/chain5j/chain5j-protocol/protocol"
	"github.com/chain5j/logger"
	"math/big"
)

var (
	errInsufficientBalanceForGas = errors.New("insufficient balance to pay for gas")
	ErrNonceTooHigh              = errors.New("nonce too high")
	ErrNonceTooLow               = errors.New("nonce too low")
)

/*
The State Transitioning Model

A state transition is a change made when a transaction is applied to the current world state
The state transitioning model does all the necessary work to work out a valid new state root.

1) Nonce handling
2) Pre pay gas
3) Create a new state object if the recipient is \0*32
4) Value transfer
== If contract creation ==
  4a) Attempt to run transaction data
  4b) If valid, use result as code for the new state object
== end ==
5) Run Script section
6) Derive new state root
*/
type StateTransition struct {
	log        logger.Logger
	gp         *vm.GasPool
	msg        models.VmMessage
	gas        uint64
	gasPrice   *big.Int
	initialGas uint64
	value      *big.Int
	data       []byte
	state      protocol.StateDB
	vm         protocol.VM
}

// IntrinsicGas computes the 'intrinsic gas' for a message with the given data.
func IntrinsicGas(data []byte, contractCreation bool) (uint64, error) {
	// Set the starting gas for the raw transaction
	var gas uint64
	if contractCreation {
		gas = vm.TxGasContractCreation
	} else {
		gas = vm.TxGas
	}
	// Bump the required gas by the amount of transactional data
	if len(data) > 0 {
		// Zero and non-zero bytes are priced differently
		var nz uint64
		for _, byt := range data {
			if byt != 0 {
				nz++
			}
		}

		if (math.MaxUint64-gas)/vm.TxDataNonZeroGas < nz {
			return 0, ErrOutOfGas
		}
		gas += nz * vm.TxDataNonZeroGas

		z := uint64(len(data)) - nz
		if (math.MaxUint64-gas)/vm.TxDataZeroGas < z {
			return 0, ErrOutOfGas
		}
		gas += z * vm.TxDataZeroGas
	}
	return gas, nil
}

// NewStateTransition initialises and returns a new state transition object.
func NewStateTransition(evm protocol.VM, msg models.VmMessage, gp *vm.GasPool) *StateTransition {
	st := &StateTransition{
		log:      logger.New("state_transition"),
		gp:       gp,
		vm:       evm,
		msg:      msg,
		gasPrice: msg.GasPrice(),
		value:    msg.Value(),
		data:     msg.Input(),
		state:    evm.DB(),
	}

	return st
}

// ApplyMessage computes the new state by applying the given message
// against the old state within the environment.
//
// ApplyMessage returns the bytes returned by any EVM execution (if it took place),
// the gas used (which includes gas refunds) and an error if it failed. An error always
// indicates a core error meaning that the message would always fail for that particular
// state and would never be accepted within a block.
func ApplyMessage(evm protocol.VM, msg models.VmMessage, gp *vm.GasPool) ([]byte, uint64, bool, error) {
	return NewStateTransition(evm, msg, gp).TransitionDb()
}

// to returns the recipient of the message.
func (st *StateTransition) to() types.Address {
	if st.msg == nil || st.msg.To() == "" /* contract creation */ {
		return types.Address{}
	}
	return types.DomainToAddress(st.msg.To())
}

func (st *StateTransition) useGas(amount uint64) error {
	if st.gas < amount {
		st.log.Error("[useGas] st.gas <amount", "err", evm.ErrOutOfGas)
		return evm.ErrOutOfGas
	}
	st.gas -= amount

	return nil
}

func (st *StateTransition) buyGas() error {
	mgval := new(big.Int).Mul(new(big.Int).SetUint64(st.msg.GasLimit()), st.gasPrice)

	var balance *big.Int

	balance = st.state.GetBalance(types.DomainToAddress(st.msg.From()))

	if balance.Cmp(mgval) < 0 {
		st.log.Error("[buyGas] balance.Cmp(mgval) < 0", "err", errInsufficientBalanceForGas)
		return errInsufficientBalanceForGas
	}
	if err := st.gp.SubGas(st.msg.GasLimit()); err != nil {
		st.log.Error("[buyGas] SubGas err", "err", err)
		return err
	}
	st.gas += st.msg.GasLimit()

	st.initialGas = st.msg.GasLimit()

	st.state.SubBalance(types.DomainToAddress(st.msg.From()), mgval)

	return nil
}

func (st *StateTransition) preCheck() error {
	// Make sure this transaction's nonce is correct.
	if st.msg.CheckNonce() {
		nonce := st.state.GetNonce(types.DomainToAddress(st.msg.From()))
		// TODO [xwc1125]如果是预处理，应该没有这一步
		//if nonce.Cmp(st.msg.Nonce()) < 0 {
		//	st.log.Error("[preCheck] nonce < st.msg.Nonce()", "dbNonce", nonce, "txNonce", st.msg.Nonce(), "err", ErrNonceTooHigh)
		//	return ErrNonceTooHigh
		//} else
		if nonce > st.msg.Nonce() {
			st.log.Error("[preCheck] nonce > st.msg.Nonce()", "dbNonce", nonce, "txNonce", st.msg.Nonce(), "err", ErrNonceTooLow)
			return ErrNonceTooLow
		}
	}
	return st.buyGas()
}

// TransitionDb will transition the state by applying the current message and
// returning the result including the used gas. It returns an error if failed.
// An error indicates a consensus issue.
func (st *StateTransition) TransitionDb() (ret []byte, usedGas uint64, failed bool, err error) {
	if err = st.preCheck(); err != nil {
		return
	}
	msg := st.msg
	sender := models.AccountRef(types.DomainToAddress(msg.From()))
	contractCreation := msg.To() == ""

	// Pay intrinsic gas
	gas, err := IntrinsicGas(st.data, contractCreation)
	if err != nil {
		st.log.Error("[TransitionDb] IntrinsicGas err", "err", err)
		return nil, 0, false, err
	}
	if err = st.useGas(gas); err != nil {
		return nil, 0, false, err
	}

	var (
		vm = st.vm
		// vm errors do not effect consensus and are therefor
		// not assigned to err, except for insufficient balance
		// error.
		vmerr error
	)
	if contractCreation {
		// 创建合约
		ret, _, st.gas, vmerr = vm.Create(sender, st.data, st.gas, st.value)
	} else {
		// 合约调用
		// Increment the nonce for the next transaction
		st.state.SetNonce(types.DomainToAddress(msg.From()), st.state.GetNonce(sender.Address())+1)
		ret, st.gas, vmerr = vm.Call(sender, st.to(), st.data, st.gas, st.value)
	}
	if vmerr != nil {
		st.log.Error("VM returned with error", "err", vmerr)
		// The only possible consensus-error would be if there wasn't
		// sufficient balance to make the transfer happen. The first
		// balance transfer may never fail.
		if vmerr == evm.ErrInsufficientBalance {
			return nil, 0, false, vmerr
		}
	}
	st.refundGas()

	st.state.AddBalance(st.vm.Coinbase(), new(big.Int).Mul(new(big.Int).SetUint64(st.gasUsed()), st.gasPrice))

	return ret, st.gasUsed(), vmerr != nil, err
}

func (st *StateTransition) refundGas() {
	// Apply refund counter, capped to half of the used gas.
	refund := st.gasUsed() / 2
	if refund > st.state.GetRefund() {
		refund = st.state.GetRefund()
	}
	st.gas += refund

	// Return ETH for remaining gas, exchanged at the original rate.
	remaining := new(big.Int).Mul(new(big.Int).SetUint64(st.gas), st.gasPrice)

	st.state.AddBalance(types.DomainToAddress(st.msg.From()), remaining)

	// Also return remaining gas to the block gas counter so it is
	// available for the next transaction.
	st.gp.AddGas(st.gas)
}

// gasUsed returns the amount of gas used up by the state transition.
func (st *StateTransition) gasUsed() uint64 {
	return st.initialGas - st.gas
}

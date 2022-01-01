// Package permissionInterpreter
//
// @author: xwc1125
package permissionInterpreter

import (
	"errors"
	"github.com/chain5j/chain5j-pkg/codec/rlp"
	"github.com/chain5j/chain5j-protocol/models"
	"github.com/chain5j/chain5j-protocol/models/permission"
	"github.com/chain5j/chain5j-protocol/models/statetype"
	"github.com/chain5j/chain5j-protocol/protocol"
	"github.com/chain5j/chain5j-stateApp"
	"github.com/chain5j/logger"
)

var (
	ErrOpr             = errors.New("err operation")
	ErrRoleType        = errors.New("err role type")
	ErrSupportRoleType = errors.New("err support the role type")
	ErrAdminAccount    = errors.New("from address is not admin")
	ErrPeerPubKey      = errors.New("from pubKey to peerId is err")
	ErrPeerAccount     = errors.New("from address is not peer")
)

type Interpreter struct {
	log     logger.Logger
	nodeKey protocol.NodeKey
}

func NewInterpreter(nodeKey protocol.NodeKey) *Interpreter {
	return &Interpreter{
		log:     logger.New("permission_interpreter"),
		nodeKey: nodeKey,
	}
}

func (interpreter *Interpreter) VerifyTx(ctx stateApp.InterpreterCtx, t models.StateTransaction) error {
	tx := t.(*stateApp.Transaction)
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

	var txData permission.DataPermissionOpData
	if err := rlp.DecodeBytes(tx.Input(), &txData); err != nil {
		return err
	}

	per := Portals["permission"]
	if per == nil {
		return errors.New("NoPermission is nil")
	}
	nodePermission := per.(protocol.Permission)

	switch txData.RoleType {
	case permission.ADMIN:
		return ErrSupportRoleType
	case permission.SUPERVISOR:
		{
			if !nodePermission.IsAdmin(signer.Hex(), ctx.Header().Height) {
				return ErrAdminAccount
			}

			switch txData.Opt {
			case permission.AddOp:
				return nil
			case permission.DelOp:
				return nil
			default:
				return ErrOpr
			}
		}
	case permission.COLLEAGUE:
		{
			pubKey := tx.PubKey()
			// 获取peerId
			peerId, err := interpreter.nodeKey.IdFromPub(pubKey)
			if err != nil {
				return ErrPeerPubKey
			}
			_ = peerId
			//if !nodePermission.IsPeer(peerId, ctx.Header().Height) {
			//	return ErrPeerAccount
			//}
			//switch txData.Opt {
			//case permission.Add:
			//	return nil
			//case permission.Del:
			//	return nil
			//default:
			//	return ErrOpr
			//}
		}
	case permission.PEER:
		return ErrSupportRoleType
	case permission.OBSERVER:
		{
			pubKey := tx.PubKey()
			peerId, err := interpreter.nodeKey.IdFromPub(pubKey)
			if err != nil {
				return ErrPeerPubKey
			}
			_ = peerId
			//if !nodePermission.IsPeer(peerId, ctx.Header().Height) {
			//	return ErrPeerAccount
			//}
			//switch txData.Opt {
			//case permission.Add:
			//	return nil
			//case permission.Del:
			//	return nil
			//default:
			//	return ErrOpr
			//}
		}
	default:
		return ErrRoleType
	}

	return nil
}

func (interpreter *Interpreter) ApplyTransaction(ctx stateApp.InterpreterCtx, t models.StateTransaction, usedGas *uint64) (*statetype.Receipt, error) {
	tx := t.(*stateApp.Transaction)
	stateDB := ctx.StateDB()

	if err := interpreter.VerifyTx(ctx, tx); err != nil {
		return nil, err
	}

	var txData permission.DataPermissionOpData
	if err := rlp.DecodeBytes(tx.Input(), &txData); err != nil {
		return nil, err
	}

	per := Portals["permission"]
	nodePermission := per.(protocol.Permission)

	var (
		err error
	)

	switch txData.RoleType {
	case permission.ADMIN:
		return nil, ErrSupportRoleType
	case permission.SUPERVISOR:
		{
			switch txData.Opt {
			case permission.AddOp:
				err = nodePermission.AddSupervisor(txData.Addr, permission.MemberInfo{
					Name:   txData.Name,
					Height: txData.Height,
				})
			case permission.DelOp:
				err = nodePermission.DelSupervisor(txData.Addr)
			default:
				return nil, ErrOpr
			}
		}
	case permission.COLLEAGUE:
		//{
		//	pubKey := tx.PubKey()
		//	peerId, err := interpreter.nodeKey.IdFromPub(pubKey)
		//	if err != nil {
		//		return nil, ErrPeerPubKey
		//	}
		//	if !nodePermission.IsPeer(peerId, ctx.Header().Height) {
		//		return nil, ErrPeerAccount
		//	}
		//	switch txData.Opt {
		//	case permission.Add:
		//		err = nodePermission.AddPermission(peerId, txData.Addr, permission.MemberInfo{
		//			Name:   txData.Name,
		//			Height: txData.Height,
		//		}, permission.COLLEAGUE)
		//	case permission.Del:
		//		err = nodePermission.DelPermission(peerId, txData.Addr, permission.COLLEAGUE)
		//	default:
		//		return nil, ErrOpr
		//	}
		//}
	case permission.PEER:
		return nil, ErrSupportRoleType
	case permission.OBSERVER:
		//{
		//	pubKey := tx.PubKey()
		//	peerId, err := interpreter.nodeKey.IdFromPub(pubKey)
		//	if err != nil {
		//		return nil, ErrPeerPubKey
		//	}
		//	if !nodePermission.IsPeer(peerId, ctx.Header().Height) {
		//		return nil, ErrPeerAccount
		//	}
		//	switch txData.Opt {
		//	case permission.Add:
		//		err = nodePermission.AddPermission(peerId, txData.Addr, permission.MemberInfo{
		//			Name:   txData.Name,
		//			Height: txData.Height,
		//		}, permission.OBSERVER)
		//	case permission.Del:
		//		err = nodePermission.DelPermission(peerId, txData.Addr, permission.OBSERVER)
		//	default:
		//		return nil, ErrOpr
		//	}
		//}
	default:
		return nil, ErrRoleType
	}
	if err != nil {
		return nil, err
	}

	gasUsed := uint64(21000)
	usedGas = &gasUsed

	account := tx.From()
	stateDB.SetNonce(account, stateDB.GetNonce(account)+1)

	receipt := &statetype.Receipt{
		//PostState: ctx.PreRoot.Bytes(),
		Status:          1,
		TransactionHash: tx.Hash(),
		GasUsed:         gasUsed,
		Logs:            nil,
	}
	return receipt, nil
}

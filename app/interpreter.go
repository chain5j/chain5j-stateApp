// Package app
//
// @author: xwc1125
package app

import (
	"github.com/chain5j/chain5j-protocol/protocol"
	"github.com/chain5j/chain5j-stateApp"
	"github.com/chain5j/chain5j-stateApp/interpreter/accountInterpreter"
	"github.com/chain5j/chain5j-stateApp/interpreter/baseInterpreter"
	"github.com/chain5j/chain5j-stateApp/interpreter/caInterpreter"
	"github.com/chain5j/chain5j-stateApp/interpreter/ethereumInterpreter"
	"github.com/chain5j/chain5j-stateApp/interpreter/evmInterpreter"
	"github.com/chain5j/chain5j-stateApp/interpreter/lostInterpreter"
	"github.com/chain5j/chain5j-stateApp/interpreter/permissionInterpreter"
)

var interpreters map[string]stateApp.Interpreter

// initInterpreter 初始化解析器
func initInterpreter(nodeKey protocol.NodeKey) {
	interpreters = make(map[string]stateApp.Interpreter)
	interpreters[stateApp.BaseInterpreter] = baseInterpreter.NewInterpreter()
	interpreters[stateApp.AccountInterpreter] = accountInterpreter.NewInterpreter()
	interpreters[stateApp.LostInterpreter] = lostInterpreter.NewInterpreter()
	interpreters[stateApp.EvmInterpreter] = evmInterpreter.NewInterpreter()
	interpreters[stateApp.CAInterpreter] = caInterpreter.NewInterpreter()
	interpreters[stateApp.EthereumInterpreter] = ethereumInterpreter.NewInterpreter()
	interpreters[stateApp.PermissionInterpreter] = permissionInterpreter.NewInterpreter(nodeKey)
}

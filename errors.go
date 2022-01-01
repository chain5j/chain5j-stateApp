// Package stateApp
//
// @author: xwc1125
package stateApp

import "errors"

var (
	ErrFromAccountNotFound = errors.New("from account not found")
	ErrFrozenAccount       = errors.New("frozen Account")
	ErrInvalidSigner       = errors.New("invalid account signature")
	ErrToAccountNotFound   = errors.New("to account not found")
	ErrBalanceNotEnough    = errors.New("balance not enough")
	ErrInvalidAccountOp    = errors.New("invalid account operation")
)

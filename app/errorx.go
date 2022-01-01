// Package app
//
// @author: xwc1125
package app

import "errors"

var (
	errTxLooLarge         = errors.New("tx size is over")
	errTxParse            = errors.New("parse tx is error")
	errInvalidInterpreter = errors.New("invalid interpreter")

	ErrReplaceUnderpriced = errors.New("replacement transaction underpriced")
)

// Package txpool
//
// @author: xwc1125
package txpool

import (
	"errors"
)

var (
	errTxDuplicate = errors.New("transaction duplicate")
	errTxType      = errors.New("unsupported the txType")
	errPoolFull    = errors.New("tx_pool is full")
	errTxDiscard   = errors.New("old transaction is better, discard the new one")
)

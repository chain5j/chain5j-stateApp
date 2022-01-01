// Package txpool
//
// @author: xwc1125
package txpool

import (
	"fmt"
	"github.com/chain5j/chain5j-protocol/protocol"
)

type option func(f *txPool) error

func apply(f *txPool, opts ...option) error {
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(f); err != nil {
			return fmt.Errorf("option apply err:%v", err)
		}
	}
	return nil
}

func WithConfig(config protocol.Config) option {
	return func(f *txPool) error {
		f.config = config
		return nil
	}
}

func WithApps(apps protocol.Apps) option {
	return func(f *txPool) error {
		f.apps = apps
		return nil
	}
}

func WithBlockReader(blockReader protocol.BlockReader) option {
	return func(f *txPool) error {
		f.blockReader = blockReader
		return nil
	}
}

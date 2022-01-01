// Package app
//
// @author: xwc1125
package app

import (
	"fmt"
	"github.com/chain5j/chain5j-pkg/database/kvstore"
	"github.com/chain5j/chain5j-protocol/protocol"
)

type option func(f *application) error

func apply(f *application, opts ...option) error {
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
	return func(f *application) error {
		f.config = config
		return nil
	}
}
func WithDatabase(db protocol.Database) option {
	return func(f *application) error {
		f.db = db
		return nil
	}
}
func WithBlockReadWriter(blockRW protocol.BlockReadWriter) option {
	return func(f *application) error {
		f.blockRW = blockRW
		return nil
	}
}
func WithNodeKey(nodeKey protocol.NodeKey) option {
	return func(f *application) error {
		f.nodeKey = nodeKey
		return nil
	}
}
func WithKVDatabase(kvDB kvstore.Database) option {
	return func(f *application) error {
		f.kvDB = kvDB
		return nil
	}
}
func WithApis(apis protocol.APIs) option {
	return func(f *application) error {
		f.apis = apis
		return nil
	}
}

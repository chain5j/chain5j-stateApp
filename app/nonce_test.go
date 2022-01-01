// Package app
//
// @author: xwc1125
package app

import (
	"fmt"
	stateApp "github.com/chain5j/chain5j-stateApp"
	"math/big"
	"sync"
	"testing"
)

func TestTxList_Add(t *testing.T) {
	txList := newTxList(true)
	var (
		wg   sync.WaitGroup
		lock sync.RWMutex
	)
	for i := uint64(20); i > 0; i-- {
		if i%7 == 0 {
			continue
		}
		wg.Add(1)
		go func(i uint64) {
			defer wg.Done()
			transaction := stateApp.NewTransaction("123", "456", stateApp.BaseInterpreter, i, 0, 21000, big.NewInt(0), []byte("123"), 10, []byte("123"))
			lock.Lock()
			txList.Add(transaction, 10)
			lock.Unlock()
		}(i)
	}
	wg.Wait()
	t.Log(txList.Len())

	{
		// 创建一个有序的交易集合
		txs := txList.Flatten()
		for _, tx := range txs {
			fmt.Println("Flatten", tx)
		}
	}

	{
		removedTxs, invalidTxs := txList.Filter(big.NewInt(10), 21000)
		if removedTxs != nil {
			for _, tx := range removedTxs {
				fmt.Println("removedTxs", tx)
			}
		}
		if invalidTxs != nil {
			for _, tx := range invalidTxs {
				fmt.Println("invalidTxs", tx)
			}
		}
	}

	{
		// 获取小于阀值nonce的交易，并将缓存交易删除
		txs := txList.Forward(10)
		for _, tx := range txs {
			fmt.Println("Forward", tx)
		}
	}

	{
		// 只包括指定数量的交易
		txs := txList.Cap(5)
		for _, tx := range txs {
			fmt.Println("Cap", tx)
		}
	}

}

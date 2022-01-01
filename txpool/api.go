// Package txpool
//
// @author: xwc1125
package txpool

import (
	"github.com/chain5j/chain5j-protocol/models"
	"github.com/chain5j/chain5j-protocol/protocol"
	"sort"
)

// API API
type API struct {
	pool *txPool
}

// NewAPI new txPool API
func NewAPI(t *txPool) *API {
	return &API{
		pool: t,
	}
}

// APIs impl the API
func (t *txPool) APIs() []protocol.API {
	return []protocol.API{
		{
			Namespace: "txpool", // namespace
			Version:   "1.0",
			Service:   NewAPI(t),
			Public:    true,
		},
	}
}

type NonceHashInfo struct {
	Nonce uint64 `json:"nonce"`
	Hash  string `json:"hash"`
}

type NonceHashList []NonceHashInfo

func (a NonceHashList) Len() int {
	return len(a)
}
func (a NonceHashList) Less(i, j int) bool {
	if a[i].Nonce < a[j].Nonce {
		return true
	}
	return false
}
func (a NonceHashList) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}

// AddressStatus 地址状态
type AddressStatus struct {
	Pending NonceHashList `json:"pending"`
	Count   uint64        `json:"count"`
}

type TxPoolStatus struct {
	Count   uint64                    `json:"count"`
	Pending map[string]*AddressStatus `json:"pending"`
}

func (api *API) Status() TxPoolStatus {
	result := TxPoolStatus{
		Count:   0,
		Pending: make(map[string]*AddressStatus),
	}

	var (
		nonceHashInfo NonceHashInfo
		decode        models.Transaction
	)
	keys := api.pool.txQueue.GetAllKeys()
	for _, k := range keys {
		bytes, err := api.pool.txQueue.Get(k)
		if err == nil {
			decode, err = models.TxDecode(bytes)
		}
		if err == nil {
			tx := decode.(models.StateTransaction)
			nonceHashInfo = NonceHashInfo{
				Nonce: tx.Nonce(),
				Hash:  k,
			}
			infos := result.Pending[tx.From()]
			if infos == nil {
				infos = &AddressStatus{
					Pending: NonceHashList{},
					Count:   0,
				}
			}
			infos.Pending = append(infos.Pending, nonceHashInfo)
			infos.Count++

			result.Pending[tx.From()] = infos
			result.Count++
		}
	}

	for address, _ := range result.Pending {
		addressStatus := result.Pending[address]
		pending := addressStatus.Pending
		sort.Sort(&pending)
	}
	return result
}

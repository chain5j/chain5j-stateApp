// Package txpool
//
// @author: xwc1125
package txpool

import (
	"errors"
	"github.com/chain5j/chain5j-pkg/codec"
	"github.com/chain5j/chain5j-pkg/types"
	"github.com/chain5j/chain5j-pkg/util/hexutil"
	"github.com/chain5j/chain5j-protocol/models"
	"github.com/chain5j/chain5j-protocol/protocol"
	"github.com/chain5j/logger"
	mapset "github.com/deckarep/golang-set"
	"sync"
)

const (
	maxKnownTxs  = 32768 // 保存最大的可知交易 (prevent DOS)
	maxQueuedTxs = 128   // 最大的交易发送队列
)

var (
	errClosed            = errors.New("peer set is closed")
	errAlreadyRegistered = errors.New("peer is already registered")
	errNotRegistered     = errors.New("peer is not registered")
)

// ===========p2p===========
// peer 单个peer
type peer struct {
	log           logger.Logger
	currentPeerId models.P2PID // 此peer的ID
	p2pService    protocol.P2PService

	knownTxs  mapset.Set                        // 节点已经拥有的tx
	queuedTxs chan models.TransactionSortedList // 发送交易的队列

	stopCh chan struct{}
}

func newPeer(currentPeerId models.P2PID, p2pService protocol.P2PService) *peer {
	peer := &peer{
		log:           logger.New("txPool"),
		currentPeerId: currentPeerId,
		p2pService:    p2pService,

		knownTxs:  mapset.NewSet(),
		queuedTxs: make(chan models.TransactionSortedList, maxQueuedTxs),

		stopCh: make(chan struct{}),
	}
	return peer
}

// SendTransactions 发送交易
func (p *peer) SendTransactions(txs models.TransactionSortedList) error {
	// 自己发送出去的，默认节点已经知道
	for _, tx := range txs {
		p.knownTxs.Add(tx.Hash())
	}

	p.log.Debug("sendTransactions", "txs", txs.Hashes())

	toBytes, err := codec.Coder().Encode(txs)
	if err != nil {
		return err
	}
	p.log.Debug("sendTransactions", "data", hexutil.Encode(toBytes))

	err = p.p2pService.Send(p.currentPeerId, &models.P2PMessage{
		Type: models.TransactionSend,
		Peer: "", // 发是自己的，接收时，是对方
		Data: toBytes,
	})

	if err != nil {
		p.log.Error("p.p2pService.Send err", "err", err)
	}
	return err
}

// AsyncSendTransactions 异步发送
func (p *peer) AsyncSendTransactions(txs models.TransactionSortedList) {
	select {
	// 将交易放进队列中，并且标记交易为已发送
	case p.queuedTxs <- txs:
		for _, tx := range txs {
			p.knownTxs.Add(tx.Hash())
		}
	default:
		p.log.Debug("Dropping transaction propagation", "count", txs.Len())
	}
}

// broadcast 广播交易
func (p *peer) broadcast() {
	for {
		select {
		case txs := <-p.queuedTxs:
			// 队列中添加了交易就会触发发送交易
			if err := p.SendTransactions(txs); err != nil {
				p.log.Error("sendTransactions is error", "err", err)
				break
			}
			p.log.Trace("Broadcast transactions", "count", txs.Len())

		case <-p.stopCh:
			p.log.Trace("broadcast stopCh")
			return
		}
	}
}

// MarkTransaction 标记Hash为已知。如果队列超过最大值，那么将旧数据清理
func (p *peer) MarkTransaction(hash types.Hash) {
	// If we reached the memory allowance, drop a previously known transaction hash
	for p.knownTxs.Cardinality() >= maxKnownTxs {
		p.knownTxs.Pop()
	}
	p.knownTxs.Add(hash)
}

func (p *peer) close() {
	close(p.stopCh)
}

// peerSet peer集合
type peerSet struct {
	log logger.Logger

	localPeerId models.P2PID // 当前的peerID

	peers map[models.P2PID]*peer
	lock  sync.RWMutex

	closed bool
}

func newPeerSet(localPeerId models.P2PID) *peerSet {
	return &peerSet{
		log:         logger.New("txpool"),
		localPeerId: localPeerId,
		peers:       make(map[models.P2PID]*peer),
	}
}

// Register 注册新的peer
func (ps *peerSet) Register(p *peer) error {
	ps.lock.Lock()
	defer ps.lock.Unlock()

	// 如果已经closed，那么不能再添加新的peer
	if ps.closed {
		return errClosed
	}
	if _, ok := ps.peers[p.currentPeerId]; ok {
		return errAlreadyRegistered
	}
	ps.peers[p.currentPeerId] = p
	go p.broadcast()

	return nil
}

// Deregister 取消注册
func (ps *peerSet) Deregister(id models.P2PID) error {
	ps.lock.Lock()
	defer ps.lock.Unlock()

	p, ok := ps.peers[id]
	if !ok {
		return errNotRegistered
	}
	delete(ps.peers, id)
	p.close()

	return nil
}

// Peer 获取peerID对象
func (ps *peerSet) Peer(id models.P2PID) *peer {
	ps.lock.RLock()
	defer ps.lock.RUnlock()
	return ps.peers[id]
}

// Len 获取peer个数
func (ps *peerSet) Len() int {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	return len(ps.peers)
}

// PeersWithoutTx 返回peer集合
// peerId 数据来源的peerId
// isForce true不管来源peerId是否包含都需要进行广播
func (ps *peerSet) PeersWithoutTx(peerId *models.P2PID, hash types.Hash, isForce bool) []*peer {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	list := make([]*peer, 0, len(ps.peers))
	for _, p := range ps.peers {
		if !isForce {
			if peerId != nil && p.currentPeerId == *peerId {
				// 来源ID等于当前的peerId，那么代表已经知道
				p.knownTxs.Add(hash)
			} else {
				// 如果peer不含hash，代表需要进行p2p的节点
				if !p.knownTxs.Contains(hash) {
					list = append(list, p)
				}
			}
		} else {
			if !p.knownTxs.Contains(hash) {
				list = append(list, p)
			}
		}
	}
	return list
}

// Close 关闭
func (ps *peerSet) Close() {
	ps.lock.Lock()
	defer ps.lock.Unlock()

	for _, p := range ps.peers {
		p.close()
	}
	ps.closed = true
}

// BroadcastTxs 广播交易集合
// peerId 数据来源的peerId
// isForce true不管来源peerId是否包含都需要进行广播
func (ps *peerSet) BroadcastTxs(peerId *models.P2PID, txs models.TransactionSortedList, isForce bool) {
	if txs == nil {
		return
	}
	var txSet = make(map[*peer]models.TransactionSortedList)

	// Broadcast transactions to a batch of peers not knowing about it
	for _, tx := range txs {
		peers := ps.PeersWithoutTx(peerId, tx.Hash(), isForce)
		for _, peer := range peers {
			// if _, ok := txSet[peer]; !ok {
			//	txSet[peer] = new(models.TransactionSortedList)
			// }

			txSet[peer] = append(txSet[peer], tx)
			ps.log.Trace("BroadcastTxs", "hash", tx.Hash())
		}
		ps.log.Trace("Broadcast transaction", "hash", tx.Hash(), "recipients", len(peers))
	}
	for peer, txs := range txSet {
		peer.AsyncSendTransactions(txs)
	}
}

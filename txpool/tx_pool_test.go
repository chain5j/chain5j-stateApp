// Package txpool
//
// @author: xwc1125
package txpool

import (
	"context"
	"github.com/chain5j/chain5j-pkg/codec/json"
	"github.com/chain5j/chain5j-pkg/event"
	"github.com/chain5j/chain5j-pkg/types"
	"github.com/chain5j/chain5j-pkg/util/convutil"
	"github.com/chain5j/chain5j-pkg/util/dateutil"
	"github.com/chain5j/chain5j-protocol/mock"
	"github.com/chain5j/chain5j-protocol/models"
	"github.com/chain5j/logger"
	"github.com/chain5j/logger/zap"
	"github.com/golang/mock/gomock"
	"math/big"
	"testing"
)

func init() {
	zap.InitWithConfig(&logger.LogConfig{
		Console: logger.ConsoleLogConfig{
			Level:    4,
			Modules:  "*",
			ShowPath: false,
			Format:   "",
			UseColor: true,
			Console:  true,
		},
		File: logger.FileLogConfig{},
	})
}

func TestFactory_NewFactory(t *testing.T) {
	mockCtl := gomock.NewController(nil)
	// config
	mockConfig := mock.NewMockConfig(mockCtl)
	mockConfig.EXPECT().ChainConfig().Return(models.ChainConfig{
		GenesisHeight: 10,
		ChainID:       1,
		ChainName:     "chain5j",
		VersionName:   "v1.0.0",
		VersionCode:   1,
		TxSizeLimit:   128,
		Packer: &models.PackerConfig{
			WorkerType:           models.Timing,
			BlockMaxTxsCapacity:  10000,
			BlockMaxSize:         2048,
			BlockMaxIntervalTime: 1000,
			BlockGasLimit:        5000000,
			Period:               3000,
			EmptyBlocks:          0,
			Timeout:              1000,
			MatchTxsCapacity:     true,
		},
		StateApp: nil,
	}).AnyTimes()
	mockConfig.EXPECT().TxPoolConfig().Return(models.TxPoolLocalConfig{
		Capacity:     10000,
		CacheDir:     "./logs/txPool/cache",
		Metrics:      true,
		MetricsLevel: 2,
	}).AnyTimes()

	// apps
	mockContexts := mock.NewMockAppContexts(mockCtl)
	mockContexts.EXPECT().Ctx(gomock.Any()).Return(nil).AnyTimes()
	mockApplication := mock.NewMockApplication(mockCtl)
	mockApplication.EXPECT().DeleteOkTx(gomock.Any()).Return(nil).AnyTimes()
	mockApplication.EXPECT().DeleteErrTx(gomock.Any()).Return(nil).AnyTimes()
	mockApplication.EXPECT().ValidateTx(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mockApps := mock.NewMockApps(mockCtl)
	mockApps.EXPECT().NewAppContexts("txPool", gomock.Any()).Return(mockContexts, nil).AnyTimes()
	mockApps.EXPECT().App(gomock.Any()).Return(mockApplication, nil).AnyTimes()

	// mockBlockReader
	mockBlockReader := mock.NewMockBlockReader(mockCtl)
	mockBlockReader.EXPECT().SubscribeChainHeadEvent(gomock.Any()).Return(event.NewSubscription(func(quit <-chan struct{}) error {
		return nil
	})).AnyTimes()
	mockBlockReader.EXPECT().CurrentBlock().Return(models.NewBlock(&models.Header{
		ParentHash:  types.EmptyHash,
		Height:      10,
		StateRoots:  []byte("123456"),
		TxsRoot:     []types.Hash{types.EmptyHash},
		Timestamp:   uint64(dateutil.CurrentTime()),
		GasUsed:     0,
		GasLimit:    0,
		ArchiveHash: nil,
		Consensus:   nil,
		Extra:       nil,
		Signature:   nil,
	}, nil, nil)).AnyTimes()

	// mockBroadcaster
	mockBroadcaster := mock.NewMockBroadcaster(mockCtl)
	mockBroadcaster.EXPECT().SubscribeMsg(gomock.Any(), gomock.Any()).Return(event.NewSubscription(func(quit <-chan struct{}) error {
		return nil
	})).AnyTimes()
	mockBroadcaster.EXPECT().BroadcastTxs(gomock.Any(), gomock.Any(), gomock.Any()).Return().AnyTimes()

	txPool, err := NewTxPool(context.Background(),
		WithConfig(mockConfig),
		WithApps(mockApps),
		WithBlockReader(mockBlockReader),
	)
	if err != nil {
		t.Fatal(err)
	}
	txPool.Start()

	// 添加
	{
		models.RegisterTransaction(&MockTransaction{})
		for i := 0; i < 10; i++ {
			tx := MockTransaction{
				Thash:  types.HexToHash(convutil.ToString(i)),
				Tnonce: big.NewInt(int64(i)),
			}
			if err := txPool.Add(nil, &tx); err != nil {
				t.Error(err)
			}
		}
	}
	// 拉取
	{
		txs := txPool.FetchTxs(10, uint64(dateutil.CurrentTime()))
		t.Logf("txsLen=%d", len(txs))
		// 删除
		for _, tx := range txs {
			txPool.Delete(tx, false)
		}
		t.Logf("txsLen=%d", txPool.Len())
		txPool.Fallback(txs[:5])
	}
}

var (
	_ models.Transaction = new(MockTransaction)
)

type MockTransaction struct {
	Thash  types.Hash
	Tnonce *big.Int
}

func (m MockTransaction) Less(tx2 models.Transaction) bool {
	//TODO implement me
	panic("implement me")
}

func (m MockTransaction) TxType() types.TxType {
	return types.TxType("TEST")
}

func (m MockTransaction) ChainId() string {
	return "chain5j"
}

func (m MockTransaction) Hash() types.Hash {
	return m.Thash
}

func (m MockTransaction) From() string {
	return "0x9254E62FBCA63769DFd4Cc8e23f630F0785610CE"
}

func (m MockTransaction) To() string {
	return "0x9254E62FBCA63769DFd4Cc8e23f630F0785610CE"
}

func (m MockTransaction) GasLimit() uint64 {
	return 21000
}

func (m MockTransaction) Value() *big.Int {
	return big.NewInt(0)
}

func (m MockTransaction) Input() []byte {
	return []byte("123")
}

func (m MockTransaction) GasPrice() uint64 {
	return 0
}

func (m MockTransaction) Nonce() *big.Int {
	return m.Tnonce
}

func (m MockTransaction) Signer() (types.Address, error) {
	return types.HexToAddress("0x9254E62FBCA63769DFd4Cc8e23f630F0785610CE"), nil
}

func (m MockTransaction) Cost() *big.Int {
	return big.NewInt(0)
}

func (m MockTransaction) Size() types.StorageSize {
	return 0
}

func (m MockTransaction) Serialize() ([]byte, error) {
	return json.Marshal(m)
}

func (m *MockTransaction) Deserialize(d []byte) error {
	var tx MockTransaction
	json.Unmarshal(d, &tx)
	*m = tx
	return nil
}

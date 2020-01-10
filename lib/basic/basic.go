package basic

import (
	"github.com/corestario/cosmos-utils/client/authtypes"
	"github.com/corestario/cosmos-utils/client/context"
	"github.com/corestario/cosmos-utils/client/utils"
	"github.com/corestario/dkglib/lib/offChain"
	"github.com/corestario/dkglib/lib/onChain"
	dkg "github.com/corestario/dkglib/lib/types"
	"github.com/cosmos/cosmos-sdk/client/keys"
	"github.com/tendermint/go-amino"
	tmtypes "github.com/tendermint/tendermint/alias"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/libs/events"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/types"
	"os"
	"sync"
)

type DKGBasic struct {
	offChain  *offChain.OffChainDKG
	onChain   *onChain.OnChainDKG
	mtx       sync.Mutex
	isOnChain bool
	logger    log.Logger

	// TODO:maybe better is to make the chan buf
	blockNotifier chan bool
}

var _ dkg.DKG = &DKGBasic{}

func NewDKGBasic(
	evsw events.EventSwitch,
	cdc *amino.Codec,
	chainID string,
	nodeEndpoint string,
	homeString string,
	options ...offChain.DKGOption,
) (dkg.DKG, error) {
	cliCtx, err := context.NewContextWithDelay(chainID, nodeEndpoint, homeString)
	if err != nil {
		return nil, err
	}

	kb, err := keys.NewKeyBaseFromDir(cliCtx.Home)
	if err != nil {
		return nil, err
	}
	txBldr := authtypes.NewTxBuilder(
		utils.GetTxEncoder(cdc),
		0,
		0,
		400000,
		0.0,
		false,
		chainID,
		"",
		nil,
		nil,
	).WithKeybase(kb)
	logger := log.NewTMLogger(os.Stdout)
	return &DKGBasic{
		offChain: offChain.NewOffChainDKG(evsw, chainID, options...),
		onChain:  onChain.NewOnChainDKG(cliCtx, &txBldr),
		logger:   logger,
	}, nil
}

type MockFirer struct{}

func (m *MockFirer) FireEvent(event string, data events.EventData) {}

func (m *DKGBasic) NewBlockNotify() {
	m.blockNotifier <- true
}

func (m *DKGBasic) HandleOffChainShare(
	dkgMsg *dkg.DKGDataMessage,
	height int64,
	validators *types.ValidatorSet,
	pubKey crypto.PubKey,
) bool {
	// check if on-chain dkg is running
	m.mtx.Lock()
	if m.isOnChain {
		m.mtx.Unlock()
		m.logger.Info("On-chain DKG is running, stop off-chain attempt")
		return false
	}

	switchToOnChain := m.offChain.HandleOffChainShare(dkgMsg, height, validators, pubKey)
	// have to switch to on-chain
	if switchToOnChain {
		m.logger.Info("Switch to on-chain DKG")
		m.isOnChain = true
		// unlock here for not to wait isOnChain check
		m.mtx.Unlock()

		// try on-chain till success
		for {
			if m.runOnChainDKG(validators, m.logger) {
				break
			}
		}
		m.mtx.Lock()
		m.isOnChain = false
		m.mtx.Unlock()
	} else {
		m.mtx.Unlock()
	}
	m.logger.Info("Handle off-chain share end")

	// TODO check return statement
	return false
}

func (m *DKGBasic) runOnChainDKG(validators *types.ValidatorSet, logger log.Logger) bool {
	err := m.onChain.StartRound(
		validators,
		m.offChain.GetPrivValidator(),
		&MockFirer{},
		logger,
		0,
	)
	if err != nil {
		m.logger.Info("On-chain DKG start round failed", "error", err)
		panic(err)
	}

	for {
		select {
		case <-m.blockNotifier:
			if err, ok := m.onChain.ProcessBlock(); err != nil {
				m.logger.Info("on-chain DKG process block failed", "error", err)
				return false
			} else if ok {
				m.logger.Info("All instances finished on-chain DKG, O.K.")
				return true
			}
		}
	}
}

func (m *DKGBasic) CheckDKGTime(height int64, validators *types.ValidatorSet) {
	m.offChain.CheckDKGTime(height, validators)
}

func (m *DKGBasic) SetVerifier(verifier dkg.Verifier) {
	m.offChain.SetVerifier(verifier)
}

func (m *DKGBasic) Verifier() dkg.Verifier {
	return m.offChain.Verifier()
}

func (m *DKGBasic) MsgQueue() chan *dkg.DKGDataMessage {
	return m.offChain.MsgQueue()
}

func (m *DKGBasic) GetLosers() []*tmtypes.Validator {
	return append(m.offChain.GetLosers(), m.onChain.GetLosers()...)
}

func (m *DKGBasic) StartDKGRound(validators *tmtypes.ValidatorSet) error {
	return m.offChain.StartDKGRound(validators)
}

package basic

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/cosmos/cosmos-sdk/client/keys"
	"github.com/dgamingfoundation/cosmos-utils/client/authtypes"
	"github.com/dgamingfoundation/cosmos-utils/client/context"
	"github.com/dgamingfoundation/cosmos-utils/client/utils"
	"github.com/dgamingfoundation/dkglib/lib/offChain"
	"github.com/dgamingfoundation/dkglib/lib/onChain"
	dkg "github.com/dgamingfoundation/dkglib/lib/types"
	"github.com/tendermint/go-amino"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/libs/events"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/types"
)

type DKGBasic struct {
	offChain  *offChain.OffChainDKG
	onChain   *onChain.OnChainDKG
	mtx       sync.Mutex
	isOnChain bool
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

	return &DKGBasic{
		offChain: offChain.NewOffChainDKG(evsw, chainID, options...),
		onChain:  onChain.NewOnChainDKG(cliCtx, &txBldr),
	}, nil
}

type MockFirer struct{}

func (m *MockFirer) FireEvent(event string, data events.EventData) {}

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
		return false
	}

	switchToOnChain := m.offChain.HandleOffChainShare(dkgMsg, height, validators, pubKey)
	// have to switch to on-chain
	if switchToOnChain {
		m.isOnChain = true
		// unlock here for not to wait isOnChain check
		m.mtx.Unlock()

		logger := log.NewTMLogger(os.Stdout)

		// try on-chain till success
		for {
			if m.runOnChainDKG(validators, logger) {
				break
			}
		}
		m.mtx.Lock()
		m.isOnChain = false
		m.mtx.Unlock()
	} else {
		m.mtx.Unlock()
	}

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
		panic(err)
	}

	tk := time.NewTicker(time.Millisecond * 3000)
	for {
		select {
		case <-tk.C:
			if err, ok := m.onChain.ProcessBlock(); err != nil {
				// slash here
				return false
			} else if ok {
				fmt.Println("All instances finished DKG, O.K.")
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

func (m *DKGBasic) GetLosers() []crypto.Address {
	return append(m.offChain.GetLosers(), m.onChain.GetLosers()...)
}

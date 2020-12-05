package basic

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/corestario/cosmos-utils/client/authtypes"
	"github.com/corestario/cosmos-utils/client/context"
	"github.com/corestario/cosmos-utils/client/utils"
	"github.com/corestario/dkglib/lib/msgs"
	"github.com/corestario/dkglib/lib/offChain"
	"github.com/corestario/dkglib/lib/onChain"
	dkg "github.com/corestario/dkglib/lib/types"
	"github.com/cosmos/cosmos-sdk/client/keys"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authTypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/tendermint/go-amino"
	tmtypes "github.com/tendermint/tendermint/alias"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/libs/events"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/types"
)

type DKGBasic struct {
	offChain      *offChain.OffChainDKG
	onChain       *onChain.OnChainDKG
	mtx           sync.RWMutex
	isOnChain     bool
	logger        log.Logger
	OnChainParams OnChainParams
	blockNotifier chan bool
	roundID       int
}

type OnChainParams struct {
	Cdc          *amino.Codec
	ChainID      string
	NodeEndpoint string
	HomeString   string
	PassPhrase   string
}

var _ dkg.DKG = &DKGBasic{}

func NewDKGBasic(
	evsw events.EventSwitch,
	cdc *amino.Codec,
	chainID string,
	nodeEndpoint string,
	passPhrase string,
	homeString string,
	options ...offChain.DKGOption,
) (dkg.DKG, error) {
	logger := log.NewTMLogger(os.Stdout)
	d := &DKGBasic{
		offChain:      offChain.NewOffChainDKG(evsw, chainID, options...),
		logger:        logger,
		blockNotifier: make(chan bool, 2),
		OnChainParams: OnChainParams{
			Cdc:          cdc,
			ChainID:      chainID,
			NodeEndpoint: nodeEndpoint,
			HomeString:   homeString,
			PassPhrase:   passPhrase,
		},
	}
	return d, nil
}

type MockFirer struct{}

func (m *MockFirer) FireEvent(event string, data events.EventData) {}

func (m *DKGBasic) NewBlockNotify() {
	if len(m.blockNotifier) == 0 {
		m.blockNotifier <- true
	}
}

func (m *DKGBasic) HandleOffChainShare(
	dkgMsg *dkg.DKGDataMessage,
	height int64,
	validators *types.ValidatorSet,
	pubKey crypto.PubKey,
) bool {
	// check if on-chain dkg is running
	m.mtx.RLock()

	if m.isOnChain {
		m.mtx.RUnlock()
		m.logger.Info("On-chain DKG is running, stop off-chain attempt")
		return false
	}
	m.mtx.RUnlock()

	switchToOnChain := m.offChain.HandleOffChainShare(dkgMsg, height, validators, pubKey)
	// have to switch to on-chain
	if switchToOnChain {
		m.logger.Info("Switch to on-chain DKG")
		m.mtx.Lock()
		m.isOnChain = true
		m.mtx.Unlock()

		err := m.initOnChain()
		if err != nil {
			m.logger.Error("could not init On chain dkg", "error", err)
			return false
		}

		err = m.onChain.StartRound(
			validators,
			m.offChain.GetPrivValidator(),
			&MockFirer{},
			m.logger,
			m.roundID,
		)
		if err != nil {
			m.logger.Info("On-chain DKG start round failed", "error", err)
			panic(err)
		}
		roundID := m.roundID
		m.roundID++

		go func() {
			for {
				select {
				case <-m.blockNotifier:
					m.logger.Info("DKG ticker in switch")
					if err, ok := m.onChain.ProcessBlock(roundID); err != nil {
						m.logger.Info("on-chain DKG process block failed", "error", err)
						m.mtx.Lock()
						m.isOnChain = false
						m.mtx.Unlock()
						return
					} else if ok {
						m.logger.Info("All instances finished on-chain DKG, O.K.")
						m.mtx.Lock()
						m.isOnChain = false

						verifier, err := m.onChain.GetVerifier()
						if err != nil {
							m.logger.Error("On-chain DKG verifier error", "error", err)
							panic(err)
						}
						m.offChain.SetNextVerifier(verifier)
						m.offChain.SetChangeHeight((height + offChain.BlocksAhead) - ((height + offChain.BlocksAhead) % 5))

						m.mtx.Unlock()
						return
					}
				default:
					time.Sleep(time.Second * 1)
				}
			}
		}()
		// try on-chain till success
	}

	// returning bool to implement interface, return value, probably, will not be used
	return true
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

func (m *DKGBasic) IsOnChain() bool {
	m.mtx.RLock()
	defer m.mtx.RUnlock()
	return m.isOnChain
}

func (m *DKGBasic) initOnChain() error {
	if m.onChain != nil {
		return nil
	}

	m.logger.Info("Init on-chain DKG")

	cliCtx, err := context.NewContextWithDelay(m.OnChainParams.ChainID, m.OnChainParams.NodeEndpoint, m.OnChainParams.HomeString)
	if err != nil {
		m.logger.Error("Init on-chain DKG error", "function", "NewContextWithDelay", "error", err)
		return err
	}

	kb, err := keys.NewKeyBaseFromDir(cliCtx.Home)
	if err != nil {
		m.logger.Error("Init on-chain DKG error", "function", "NewKeyBaseFromDir", "error", err)
		return err
	}
	keysList, err := kb.List()
	if err != nil {
		m.logger.Error("Init on-chain DKG error", "function", "List", "error", err)
		return err
	}
	if len(keysList) == 0 {
		err := fmt.Errorf("key list error: account does not exist")
		m.logger.Error("Init on-chain DKG error", "error", err)
		return err
	}

	cliCtx.WithFromName(keysList[0].GetName()).
		WithPassphrase(m.OnChainParams.PassPhrase).
		WithFromAddress(keysList[0].GetAddress()).
		WithFrom(keysList[0].GetName())
	authTypes.RegisterCodec(m.OnChainParams.Cdc)
	m.OnChainParams.Cdc.RegisterConcrete(msgs.MsgSendDKGData{}, msgs.MsgSendDKGDataTypeName, nil)
	sdk.RegisterCodec(m.OnChainParams.Cdc)
	cliCtx.WithCodec(m.OnChainParams.Cdc)

	accRetriever := authTypes.NewAccountRetriever(cliCtx)
	accNumber, accSequence, err := accRetriever.GetAccountNumberSequence(keysList[0].GetAddress())
	if err != nil {
		m.logger.Error("Init on-chain DKG error", "function", "GetAccountNumberSequence", "error", err)
		return err
	}

	txBldr := authtypes.NewTxBuilder(
		utils.GetTxEncoder(m.OnChainParams.Cdc),
		accNumber,
		accSequence,
		400000*100,
		0.0,
		false,
		m.OnChainParams.ChainID,
		"",
		nil,
		nil,
	).WithKeybase(kb)

	m.onChain = onChain.NewOnChainDKG(cliCtx, &txBldr)
	return nil
}

func (m *DKGBasic) ProcessBlock(roundID int) (error, bool) {
	return m.onChain.ProcessBlock(roundID)
}

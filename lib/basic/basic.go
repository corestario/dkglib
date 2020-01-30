package basic

import (
	"errors"
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
	OnChainParams *onChain.OnChainParams
	blockNotifier chan bool
}

func (m *DKGBasic) initOnChain() error {
	if m.onChain != nil {
		return nil
	}

	m.logger.Info("Init on-chain DKG")

	cliCtx, err := context.NewContextWithDelay(m.OnChainParams.ChainID, m.OnChainParams.NodeEndpoint, m.OnChainParams.HomeString)
	if err != nil {
		return err
	}

	kb, err := keys.NewKeyBaseFromDir(cliCtx.Home)
	if err != nil {
		return err
	}
	keysList, err := kb.List()
	if err != nil {
		return err
	}
	if len(keysList) == 0 {
		return errors.New("account is not exist")
	}

	cliCtx.WithFromName(keysList[0].GetName()).WithPassphrase("12345678").WithFromAddress(keysList[0].GetAddress()).WithFrom(keysList[0].GetName())
	authTypes.RegisterCodec(m.OnChainParams.Cdc)
	m.OnChainParams.Cdc.RegisterConcrete(msgs.MsgSendDKGData{}, "randapp/SendDKGData", nil)
	sdk.RegisterCodec(m.OnChainParams.Cdc)
	cliCtx.WithCodec(m.OnChainParams.Cdc)

	accRetriever := authTypes.NewAccountRetriever(cliCtx)
	accNumber, accSequence, err := accRetriever.GetAccountNumberSequence(keysList[0].GetAddress())
	if err != nil {
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
	m.onChain.OnChainParams = m.OnChainParams
	return nil
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
	logger := log.NewTMLogger(os.Stdout)
	d := &DKGBasic{
		offChain:      offChain.NewOffChainDKG(evsw, chainID, options...),
		logger:        logger,
		blockNotifier: make(chan bool, 2),
		OnChainParams: &onChain.OnChainParams{
			Cdc:          cdc,
			ChainID:      chainID,
			NodeEndpoint: nodeEndpoint,
			HomeString:   homeString,
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

		// unlock here for not to wait isOnChain check
		//m.mtx.Unlock()

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
			0,
		)
		if err != nil {
			m.logger.Info("On-chain DKG start round failed", "error", err)
			panic(err)
		}

		go func() {
			for {
				select {
				case <-m.blockNotifier:
					m.logger.Info("DKG ticker in switch")
					if err, ok := m.onChain.ProcessBlock(); err != nil {
						m.logger.Info("on-chain DKG process block failed", "error", err)
						m.mtx.Lock()
						m.isOnChain = false
						m.mtx.Unlock()
						return
					} else if ok {
						m.logger.Info("All instances finished on-chain DKG, O.K.")
						m.mtx.Lock()
						m.isOnChain = false
						m.mtx.Unlock()
						return
					} else {
						//return false
					}
				default:
					time.Sleep(time.Second * 1)
				}
			}
		}()
		// try on-chain till success
		_ = func() {
			//m.logger.Info("GO ROUTINE START")
			//
			//for {
			//	if m.runOnChainDKG(validators, m.logger) {
			//		break
			//	}
			//}
			//m.logger.Info("GO ROUTINE BEFORE EXIT")
			//m.mtx.Lock()
			//m.isOnChain = false
			//m.mtx.Unlock()
		}
	}
	//m.logger.Info("Handle off-chain share end")

	// TODO check return statement
	return false
}

/*
func (m *DKGBasic) runOnChainDKG(validators *types.ValidatorSet, logger log.Logger) bool {
	m.logger.Info("Run on-chain DKG")
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

	ticker := time.NewTicker(3 * time.Second)
	for {
		select {
		case <-ticker.C:
			m.logger.Info("DKG ticker in switch")

			if err, ok := m.onChain.ProcessBlock(); err != nil {
				m.logger.Info("on-chain DKG process block failed", "error", err)
				return false
			} else if ok {
				m.logger.Info("All instances finished on-chain DKG, O.K.")
				return true
			} else {
				//return false
			}
		default:
			m.logger.Info("DKG default in switch")
			time.Sleep(time.Second)
		}
	}
}
*/
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

func (m *DKGBasic) ProcessBlock() (error, bool) {
	return m.onChain.ProcessBlock()
}

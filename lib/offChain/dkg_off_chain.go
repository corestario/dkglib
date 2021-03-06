package offChain

import (
	"encoding/hex"
	"fmt"
	"sync"

	dkgalias "github.com/corestario/dkglib/lib/alias"
	"github.com/corestario/dkglib/lib/blsShare"
	dkglib "github.com/corestario/dkglib/lib/dealer"
	dkgtypes "github.com/corestario/dkglib/lib/types"
	"github.com/tendermint/tendermint/alias"
	tmtypes "github.com/tendermint/tendermint/alias"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/libs/events"
	"github.com/tendermint/tendermint/libs/log"
)

const (
	BlocksAhead         = 20  // Agree to swap verifier after around this number of blocks.
	DefaultDKGNumBlocks = 100 //DefaultDKGNumBlocks sets how often node should make DKG(in blocks)
)

type OffChainDKG struct {
	mtx sync.RWMutex

	verifier     dkgtypes.Verifier
	nextVerifier dkgtypes.Verifier
	changeHeight int64

	dkgMsgQueue      chan *dkgtypes.DKGDataMessage // message queue used for dkgState-related messages.
	dkgRoundToDealer map[int]dkglib.Dealer
	dkgRoundID       int
	dkgNumBlocks     int64
	newDKGDealer     dkglib.DKGDealerConstructor
	privValidator    alias.PrivValidator

	Logger  log.Logger
	evsw    events.EventSwitch
	chainID string
}

var _ dkgtypes.DKG = &OffChainDKG{}

func NewOffChainDKG(evsw events.EventSwitch, chainID string, options ...DKGOption) *OffChainDKG {
	dkg := &OffChainDKG{
		evsw:             evsw,
		dkgMsgQueue:      make(chan *dkgtypes.DKGDataMessage, alias.MsgQueueSize),
		dkgRoundToDealer: make(map[int]dkglib.Dealer),
		newDKGDealer:     dkglib.NewDKGDealer,
		dkgNumBlocks:     DefaultDKGNumBlocks,
		chainID:          chainID,
	}

	for _, option := range options {
		option(dkg)
	}

	if dkg.dkgNumBlocks == 0 {
		dkg.dkgNumBlocks = DefaultDKGNumBlocks // We do not want to panic if the value is not provided.
	}

	return dkg
}

// DKGOption sets an optional parameter on the dkgState.
type DKGOption func(*OffChainDKG)

func WithVerifier(verifier dkgtypes.Verifier) DKGOption {
	return func(d *OffChainDKG) { d.verifier = verifier }
}

func WithDKGNumBlocks(numBlocks int64) DKGOption {
	return func(d *OffChainDKG) { d.dkgNumBlocks = numBlocks }
}

func WithLogger(l log.Logger) DKGOption {
	return func(d *OffChainDKG) { d.Logger = l }
}

func WithPVKey(pv alias.PrivValidator) DKGOption {
	return func(d *OffChainDKG) { d.privValidator = pv }
}

func WithDKGDealerConstructor(newDealer dkglib.DKGDealerConstructor) DKGOption {
	return func(d *OffChainDKG) {
		if newDealer == nil {
			return
		}
		d.newDKGDealer = newDealer
	}
}

func (m *OffChainDKG) NewBlockNotify() {
	return
}

func (m *OffChainDKG) HandleOffChainShare(
	dkgMsg *dkgtypes.DKGDataMessage,
	height int64,
	validators *alias.ValidatorSet,
	pubKey crypto.PubKey,
) (switchToOnChain bool) {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	var msg = dkgMsg.Data
	dealer, ok := m.dkgRoundToDealer[msg.RoundID]
	if !ok {
		m.Logger.Debug("dkgState: dealer not found, creating a new dealer", "round_id", msg.RoundID)
		dealer = m.newDKGDealer(validators, m.privValidator, m.sendSignedMessage, m.evsw, m.Logger, msg.RoundID)
		m.dkgRoundToDealer[msg.RoundID] = dealer
		if err := dealer.Start(); err != nil {
			m.Logger.Debug("dealer start failed, panic", "error", err.Error())
			panic(fmt.Sprintf("failed to start a dealer (round %d): %v", m.dkgRoundID, err))
		}
	}
	if dealer == nil {
		m.Logger.Debug("dkgState: received message for inactive round:", "round", msg.RoundID)
		return false
	}
	m.Logger.Debug("dkgState: received message with signature:", "signature", hex.EncodeToString(dkgMsg.Data.Signature))

	if err := dealer.VerifyMessage(*dkgMsg); err != nil {
		m.Logger.Info("DKG: can't verify message:", "error", err.Error())
		return false
	}
	m.Logger.Info("DKG: message verified")

	fromAddr := crypto.Address(msg.Addr).String()

	var err error
	switch msg.Type {
	case dkgalias.DKGPubKey:
		m.Logger.Info("dkgState: received PubKey message", "from", fromAddr, "own", m.privValidator.GetPubKey().Address())
		err = dealer.HandleDKGPubKey(msg)
	case dkgalias.DKGDeal:
		m.Logger.Info("dkgState: received Deal message", "from", fromAddr)
		err = dealer.HandleDKGDeal(msg)
	case dkgalias.DKGResponse:
		m.Logger.Info("dkgState: received Response message", "from", fromAddr)
		err = dealer.HandleDKGResponse(msg)
	case dkgalias.DKGJustification:
		m.Logger.Info("dkgState: received Justification message", "from", fromAddr)
		err = dealer.HandleDKGJustification(msg)
	case dkgalias.DKGCommits:
		m.Logger.Info("dkgState: received Commit message", "from", fromAddr)
		err = dealer.HandleDKGCommit(msg)
	case dkgalias.DKGComplaint:
		m.Logger.Info("dkgState: received Complaint message", "from", fromAddr)
		err = dealer.HandleDKGComplaint(msg)
	case dkgalias.DKGReconstructCommit:
		m.Logger.Info("dkgState: received ReconstructCommit message", "from", fromAddr)
		err = dealer.HandleDKGReconstructCommit(msg)
	}
	if err != nil {
		m.Logger.Error("dkgState: failed to handle message", "error", err, "type", msg.Type)
		m.dkgRoundToDealer[msg.RoundID] = nil
		return false
	}

	verifier, err := dealer.GetVerifier()
	if err == dkgtypes.ErrDKGVerifierNotReady {
		m.Logger.Debug("dkgState: verifier not ready")
		return false
	}
	if err != nil {
		m.Logger.Debug("dkgState: verifier should be ready, but it's not ready:", "error", err)
		m.dkgRoundToDealer[msg.RoundID] = nil
		return true
	}
	m.Logger.Info("dkgState: verifier is ready, killing older rounds")
	for roundID := range m.dkgRoundToDealer {
		if roundID < msg.RoundID {
			m.dkgRoundToDealer[msg.RoundID] = nil
		}
	}
	m.nextVerifier = verifier
	m.changeHeight = (height + BlocksAhead) - ((height + BlocksAhead) % 5)
	m.evsw.FireEvent(dkgtypes.EventDKGSuccessful, m.changeHeight)

	m.Logger.Info("handle off-chain share success")

	return false
}

func (m *OffChainDKG) startRound(validators *alias.ValidatorSet) error {
	m.dkgRoundID++
	m.Logger.Info("OffChainDKG: starting round", "round_id", m.dkgRoundID)
	_, ok := m.dkgRoundToDealer[m.dkgRoundID]
	if !ok {
		dealer := m.newDKGDealer(validators, m.privValidator, m.sendSignedMessage, m.evsw, m.Logger, m.dkgRoundID)
		m.dkgRoundToDealer[m.dkgRoundID] = dealer
		m.evsw.FireEvent(dkgtypes.EventDKGStart, m.dkgRoundID)
		return dealer.Start()
	}

	return nil
}

func (m *OffChainDKG) sendDKGMessage(msg *dkgalias.DKGData) {
	// Broadcast to peers. This will not lead to processing the message
	// on the sending node, we need to send it manually (see below).
	m.evsw.FireEvent(dkgtypes.EventDKGData, msg)
	mi := &dkgtypes.DKGDataMessage{msg}
	select {
	case m.dkgMsgQueue <- mi:
	default:
		m.Logger.Info("dkgMsgQueue is full. Using a go-routine")
		go func() { m.dkgMsgQueue <- mi }()
	}
}

func (m *OffChainDKG) sendSignedMessage(data []*dkgalias.DKGData) error {
	if len(data) < 1 {
		return fmt.Errorf("send signed message error: no data passed to this call")
	}

	for _, v := range data {
		item := v
		if err := m.Sign(item); err != nil {
			m.Logger.Debug("Off-chain DKG: failed to sign data", "error", err)
			return err
		}
		m.Logger.Info("DKG: msg signed with signature", "signature", hex.EncodeToString(item.Signature))
		m.sendDKGMessage(item)
	}

	return nil
}

// Sign sign message by dealer's secret key
func (m *OffChainDKG) Sign(data *dkgalias.DKGData) error {
	if err := m.privValidator.SignData(m.chainID, data); err != nil {
		return fmt.Errorf("failed to sign data: %v", err)
	}
	return nil
}

func (m *OffChainDKG) CheckDKGTime(height int64, validators *alias.ValidatorSet) {
	if (height == -1) && m.nextVerifier == nil {
		return
	}

	if (height == -1) || m.changeHeight == height {
		m.Logger.Info("dkgState: time to update verifier", m.changeHeight, height)
		m.verifier, m.nextVerifier = m.nextVerifier, nil
		m.changeHeight = 0
		m.evsw.FireEvent(dkgtypes.EventDKGKeyChange, height)
	}

	if height > 1 && height%m.dkgNumBlocks == 0 {
		if err := m.startRound(validators); err != nil {
			m.Logger.Debug("failed to start a dealer", "round", m.dkgRoundID, "error", err)
			panic(fmt.Sprintf("failed to start a dealer (round %d): %v", m.dkgRoundID, err))
		}
	}
}

func (m *OffChainDKG) StartDKGRound(validators *alias.ValidatorSet) error {
	return m.startRound(validators)
}

func (m *OffChainDKG) MsgQueue() chan *dkgtypes.DKGDataMessage {
	return m.dkgMsgQueue
}

func (m *OffChainDKG) Verifier() dkgtypes.Verifier {
	return m.verifier
}

func (m *OffChainDKG) SetVerifier(v dkgtypes.Verifier) {
	m.verifier = v
}

func (m *OffChainDKG) GetPrivValidator() alias.PrivValidator {
	return m.privValidator
}

func (m *OffChainDKG) ProcessBlock(roundID int) (error, bool) {
	return nil, true
}

func (m *OffChainDKG) GetLosers() []*tmtypes.Validator {
	m.mtx.Lock()
	defer m.mtx.Unlock()

	dealer, ok := m.dkgRoundToDealer[m.dkgRoundID]
	if !ok {
		m.Logger.Debug("failed to get dealer for current", "roundID", m.dkgRoundID)
		panic(fmt.Sprintf("failed to get dealer for current round ID (%d)", m.dkgRoundID))
	}

	return dealer.PopLosers()
}

type verifierFunc func(s string, i int) dkgtypes.Verifier

func GetVerifier(T, N int) verifierFunc {
	return func(s string, i int) dkgtypes.Verifier {
		return blsShare.NewTestBLSVerifierByID(s, i, T, N)
	}
}

func GetMockVerifier() verifierFunc {
	return func(s string, i int) dkgtypes.Verifier {
		return new(dkgtypes.MockVerifier)
	}
}

func (m *OffChainDKG) IsOnChain() bool {
	return false
}

package types

import (
	"errors"
	"fmt"

	"github.com/dgamingfoundation/cosmos-utils/client/authtypes"
	"github.com/dgamingfoundation/cosmos-utils/client/context"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/libs/events"
	"github.com/tendermint/tendermint/types"

	"github.com/dgamingfoundation/dkglib/lib/alias"
	"github.com/dgamingfoundation/dkglib/lib/offChain"
	"github.com/dgamingfoundation/dkglib/lib/onChain"
)

var (
	ErrDKGVerifierNotReady = errors.New("verifier not ready yet")
)

type DKGDataMessage struct {
	Data *alias.DKGData
}

func (m *DKGDataMessage) ValidateBasic() error {
	return nil
}

func (m *DKGDataMessage) String() string {
	return fmt.Sprintf("[Proposal %+v]", m.Data)
}

type DKGBasic struct {
	offChainDKG *offChain.OffChainDKG
	onChain     *onChain.OnChainDKG
}

func NewDKGBasic(
	evsw events.EventSwitch,
	cliCtx *context.Context,
	txBldr *authtypes.TxBuilder,
	options ...offChain.DKGOption,
) DKG {
	return &DKGBasic{
		offChainDKG: offChain.NewOffChainDKG(evsw, options...),
		onChain:     onChain.NewOnChainDKG(cliCtx, txBldr),
	}
}

func (m *DKGBasic) HandleOffChainShare(
	dkgMsg *DKGDataMessage,
	height int64,
	validators *types.ValidatorSet,
	pubKey crypto.PubKey,
) {
	if switchToOnChain := m.offChainDKG.HandleOffChainShare(dkgMsg, height, validators, pubKey); switchToOnChain {
		// TODO: implement.
	}
}

func (m *DKGBasic) CheckDKGTime(height int64, validators *types.ValidatorSet) {
	m.offChainDKG.CheckDKGTime(height, validators)
}

func (m *DKGBasic) SetVerifier(verifier Verifier) {
	m.offChainDKG.SetVerifier(verifier)
}

func (m *DKGBasic) Verifier() Verifier {
	return m.offChainDKG.Verifier()
}

func (m *DKGBasic) MsgQueue() chan *DKGDataMessage {
	return m.offChainDKG.MsgQueue()
}

package basic

import (
	"github.com/dgamingfoundation/cosmos-utils/client/authtypes"
	"github.com/dgamingfoundation/cosmos-utils/client/context"
	"github.com/dgamingfoundation/dkglib/lib/offChain"
	"github.com/dgamingfoundation/dkglib/lib/onChain"
	dkg "github.com/dgamingfoundation/dkglib/lib/types"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/libs/events"
	"github.com/tendermint/tendermint/types"
)

type DKGBasic struct {
	offChainDKG *offChain.OffChainDKG
	onChain     *onChain.OnChainDKG
}

func NewDKGBasic(
	evsw events.EventSwitch,
	cliCtx *context.Context,
	txBldr *authtypes.TxBuilder,
	chainID string,
	options ...offChain.DKGOption,
) dkg.DKG {
	return &DKGBasic{
		offChainDKG: offChain.NewOffChainDKG(evsw, chainID, options...),
		onChain:     onChain.NewOnChainDKG(cliCtx, txBldr),
	}
}

func (m *DKGBasic) HandleOffChainShare(
	dkgMsg *dkg.DKGDataMessage,
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

func (m *DKGBasic) SetVerifier(verifier dkg.Verifier) {
	m.offChainDKG.SetVerifier(verifier)
}

func (m *DKGBasic) Verifier() dkg.Verifier {
	return m.offChainDKG.Verifier()
}

func (m *DKGBasic) MsgQueue() chan *dkg.DKGDataMessage {
	return m.offChainDKG.MsgQueue()
}

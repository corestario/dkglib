package basic

import (
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
	"github.com/tendermint/tendermint/types"
)

type DKGBasic struct {
	offChainDKG *offChain.OffChainDKG
	onChain     *onChain.OnChainDKG
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
		offChainDKG: offChain.NewOffChainDKG(evsw, chainID, options...),
		onChain:     onChain.NewOnChainDKG(cliCtx, &txBldr),
	}, nil
}

func (m *DKGBasic) HandleOffChainShare(
	dkgMsg *dkg.DKGDataMessage,
	height int64,
	validators *types.ValidatorSet,
	pubKey crypto.PubKey,
) (switchToOnChain bool) {
	if switchToOnChain := m.offChainDKG.HandleOffChainShare(dkgMsg, height, validators, pubKey); switchToOnChain {
		// TODO: implement.
	}
	return true
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

func (m *DKGBasic) GetLosers() []crypto.Address {
	return append(m.offChainDKG.GetLosers(), m.onChainDKG.GetLosers()...)
}

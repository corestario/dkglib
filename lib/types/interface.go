package types

import (
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/types"
)

type DKG interface {
	HandleOffChainShare(dkgMsg *DKGDataMessage, height int64, validators *types.ValidatorSet, pubKey crypto.PubKey) (switchToOnChain bool)
	CheckDKGTime(height int64, validators *types.ValidatorSet)
	SetVerifier(verifier Verifier)
	Verifier() Verifier
	MsgQueue() chan *DKGDataMessage
	GetLosers() []*DKGLoser
}

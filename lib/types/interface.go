package types

import (
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/types"
)

type DKG interface {
	IsReady() bool
	HandleOffChainShare(dkgMsg *DKGDataMessage, height int64, validators *types.ValidatorSet, pubKey crypto.PubKey) (switchToOnChain bool)
	CheckDKGTime(height int64, validators *types.ValidatorSet)
	SetVerifier(verifier Verifier)
	Verifier() Verifier
	MsgQueue() chan *DKGDataMessage
	GetLosers() []*DKGLoser
	CheckLoserDuplicateData(loser *DKGLoser) bool
	CheckLoserMissingData(loser *DKGLoser) bool
	CheckLoserCorruptData(loser *DKGLoser) bool
	CheckLoserCorruptJustification(loser *DKGLoser) bool
}

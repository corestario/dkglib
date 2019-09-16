package alias

import (
	"github.com/tendermint/go-amino"
	tmalias "github.com/tendermint/tendermint/alias"
	"github.com/tendermint/tendermint/crypto"
)

type DKGDataType int

var Cdc = amino.NewCodec()
var RegisterBlockAmino = tmalias.RegisterBlockAmino

const (
	DKGPubKey DKGDataType = iota
	DKGDeal
	DKGResponse
	DKGJustification
	DKGCommits
	DKGComplaint
	DKGReconstructCommit
)

type DKGData struct {
	Type        DKGDataType
	Addr        []byte
	RoundID     int
	Data        []byte // Data is going to keep serialized kyber objects.
	ToIndex     int    // ID of the participant for whom the message is; might be not set
	NumEntities int    // Number of sub-entities in the Data array, sometimes required for unmarshaling.

	//Signature for verifying data
	Signature []byte
}

func (m DKGData) SignBytes() ([]byte, error) {
	var (
		sb  []byte
		err error
	)
	m.Signature = nil
	if sb, err = Cdc.MarshalBinaryLengthPrefixed(m); err != nil {
		return nil, err
	}
	return sb, nil
}

func (m *DKGData) GetAddrString() string {
	return crypto.Address(m.Addr).String()
}

func (m *DKGData) ValidateBasic() error {
	return nil
}

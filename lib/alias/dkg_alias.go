package alias

import (
	"os"

	"github.com/tendermint/go-amino"
	tmalias "github.com/tendermint/tendermint/alias"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/libs/log"
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
	Signature   []byte //Signature for verifying data
}

func init() {
	RegisterBlockAmino(Cdc)
}

func (m DKGData) SignBytes(string) []byte {
	m.Signature = nil
	sb, err := Cdc.MarshalBinaryLengthPrefixed(m)
	if err != nil {
		logger := log.NewTMLogger(os.Stdout)
		logger.Error("Codec MarshalBinaryLengthPrefixed error",
			"DKGData type", m.Type, "RoundID", m.RoundID, "ToIndex", m.ToIndex, "Error", err)
		panic(err)
	}
	return sb
}

func (m *DKGData) SetSignature(sig []byte) {
	m.Signature = sig
}

func (m *DKGData) GetAddrString() string {
	return crypto.Address(m.Addr).String()
}

func (m *DKGData) ValidateBasic() error {
	return nil
}

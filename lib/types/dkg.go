package types

import (
	"errors"
	"fmt"

	"github.com/dgamingfoundation/dkglib/lib/alias"
	tmtypes "github.com/tendermint/tendermint/alias"
)

var (
	ErrDKGVerifierNotReady = errors.New("verifier not ready yet")
)

type LoserType string

const (
	LoserTypeCorruptData          LoserType = "loser_type_corrupt_data"
	LoserTypeMissingData          LoserType = "loser_type_missing_data"
	LoserTypeDuplicateData        LoserType = "loser_type_duplicate_data"
	LoserTypeCorruptJustification LoserType = "loser_type_corrupt_justification"
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

type DKGLoser struct {
	Type      LoserType
	Data      DKGDataMessage
	Validator *tmtypes.Validator
}

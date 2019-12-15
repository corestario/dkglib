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
	Height    int64
	Reason    DKGDataMessage
	Validator *tmtypes.Validator
}

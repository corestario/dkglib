package lib

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtxb "github.com/dgamingfoundation/cosmos-utils/client/authtypes"
	"github.com/dgamingfoundation/cosmos-utils/client/context"
	"github.com/dgamingfoundation/cosmos-utils/client/utils"
	"github.com/dgamingfoundation/dkglib/lib/types"
	"github.com/tendermint/tendermint/libs/events"
	"github.com/tendermint/tendermint/libs/log"
	tmtypes "github.com/tendermint/tendermint/types"
)

var (
	ErrDKGVerifierNotReady = errors.New("verifier not ready yet")
)

type DKGDataMessage struct {
	Data *types.DKGData
}

func (m *DKGDataMessage) ValidateBasic() error {
	return nil
}

func (m *DKGDataMessage) String() string {
	return fmt.Sprintf("[Proposal %+v]", m.Data)
}

type OnChainDKG struct {
	cli       *context.Context
	txBldr    *authtxb.TxBuilder
	dealer    Dealer
	typesList []types.DKGDataType
}

func NewOnChainDKG(cli *context.Context, txBldr *authtxb.TxBuilder) *OnChainDKG {
	return &OnChainDKG{
		cli:    cli,
		txBldr: txBldr,
	}
}

func (m *OnChainDKG) GetVerifier() (types.Verifier, error) {
	return m.dealer.GetVerifier()
}

func (m *OnChainDKG) ProcessBlock() (error, bool) {
	for _, dataType := range []types.DKGDataType{
		types.DKGPubKey,
		types.DKGDeal,
		types.DKGResponse,
		types.DKGJustification,
		types.DKGCommits,
		types.DKGComplaint,
		types.DKGReconstructCommit,
	} {
		messages, err := m.getDKGMessages(dataType)
		if err != nil {
			return fmt.Errorf("failed to getDKGMessages: %v", err), false
		}
		var handler func(msg *types.DKGData) error
		switch dataType {
		case types.DKGPubKey:
			handler = m.dealer.HandleDKGPubKey
		case types.DKGDeal:
			handler = m.dealer.HandleDKGDeal
		case types.DKGResponse:
			handler = m.dealer.HandleDKGResponse
		case types.DKGJustification:
			handler = m.dealer.HandleDKGJustification
		case types.DKGCommits:
			handler = m.dealer.HandleDKGCommit
		case types.DKGComplaint:
			handler = m.dealer.HandleDKGComplaint
		case types.DKGReconstructCommit:
			handler = m.dealer.HandleDKGReconstructCommit
		}
		for _, msg := range messages {
			if err := handler(msg.Data); err != nil {
				return fmt.Errorf("failed to handle message: %v", err), false
			}
		}
	}

	if _, err := m.dealer.GetVerifier(); err == ErrDKGVerifierNotReady {
		return nil, false
	} else if err != nil {
		return fmt.Errorf("DKG round failed: %v", err), false
	}
	return nil, true
}

func (m *OnChainDKG) StartRound(
	validators *tmtypes.ValidatorSet,
	pv tmtypes.PrivValidator,
	eventFirer events.Fireable,
	logger log.Logger,
	startRound int) error {
	m.dealer = NewDKGDealer(validators, pv, m.sendMsg, eventFirer, logger, startRound)
	if err := m.dealer.Start(); err != nil {
		return fmt.Errorf("failed to start dealer: %v", err)
	}

	return nil
}

func (m *OnChainDKG) sendMsg(data *types.DKGData) error {
	msg := types.NewMsgSendDKGData(data, m.cli.GetFromAddress())
	if err := msg.ValidateBasic(); err != nil {
		return err
	}

	err := utils.GenerateOrBroadcastMsgs(*m.cli, *m.txBldr, []sdk.Msg{msg}, false)
	tempTxBldr := m.txBldr.WithSequence(m.txBldr.Sequence() + 1)
	m.txBldr = &tempTxBldr
	return err
}

func (m *OnChainDKG) getDKGMessages(dataType types.DKGDataType) ([]*types.RandDKGData, error) {
	res, _, err := m.cli.QueryWithData(fmt.Sprintf("custom/randapp/dkgData/%d", dataType), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to query for DKG data: %v", err)
	}

	var data []*types.RandDKGData
	var dec = gob.NewDecoder(bytes.NewBuffer(res))
	if err := dec.Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode DKG data: %v", err)
	}

	if dataType == 0 {
		fmt.Println("DATA LEN=", data)
	}

	return data, nil
}

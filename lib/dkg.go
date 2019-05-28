package lib

import (
	"bytes"
	"encoding/gob"
	"fmt"

	"github.com/cosmos/cosmos-sdk/client/context"
	"github.com/cosmos/cosmos-sdk/client/utils"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtxb "github.com/cosmos/cosmos-sdk/x/auth/client/txbuilder"
	"github.com/dgamingfoundation/randapp/x/randapp"
	"github.com/tendermint/tendermint/consensus"
	"github.com/tendermint/tendermint/libs/events"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/types"
)

type OnChainDKG struct {
	cli    *context.CLIContext
	txBldr *authtxb.TxBuilder
	dealer consensus.Dealer
}

func NewOnChainDKG(cli *context.CLIContext, txBldr *authtxb.TxBuilder) *OnChainDKG {
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
			if err := handler(msg); err != nil {
				return fmt.Errorf("failed to handle message: %v", err), false
			}
		}
	}

	if _, err := m.dealer.GetVerifier(); err == consensus.ErrDKGVerifierNotReady {
		return nil, true
	} else if err != nil {
		return fmt.Errorf("DKG round failed: %v", err), false
	}

	return nil, false
}

func (m *OnChainDKG) StartRound(
	validators *types.ValidatorSet,
	pv types.PrivValidator,
	eventFirer events.Fireable,
	logger log.Logger,
	startRound uint64) error {
	m.dealer = consensus.NewDKGDealer(validators, pv, m.sendMsg, eventFirer, logger)
	if err := m.dealer.Start(); err != nil {
		return fmt.Errorf("failed to start dealer: %v", err)
	}

	return nil
}

func (m *OnChainDKG) sendMsg(data *types.DKGData) error {
	msg := randapp.NewMsgSendDKGData(data, m.cli.GetFromAddress())
	if err := msg.ValidateBasic(); err != nil {
		return err
	}

	return utils.GenerateOrBroadcastMsgs(*m.cli, *m.txBldr, []sdk.Msg{msg}, false)
}

func (m *OnChainDKG) getDKGMessages(dataType types.DKGDataType) ([]*types.DKGData, error) {
	res, err := m.cli.QueryWithData(fmt.Sprintf("custom/randapp/dkgData/%d", dataType), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to query for DKG data: %v", err)
	}

	var data []*types.DKGData
	var dec = gob.NewDecoder(bytes.NewBuffer(res))
	if err := dec.Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode DKG data: %v", err)
	}

	return data, nil
}

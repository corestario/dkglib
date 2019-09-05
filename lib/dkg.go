package lib

import (
	"bytes"
	"encoding/gob"
	"fmt"

	l "log"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtxb "github.com/dgamingfoundation/cosmos-utils/client/authtypes"
	"github.com/dgamingfoundation/cosmos-utils/client/context"
	"github.com/dgamingfoundation/cosmos-utils/client/utils"
	"github.com/dgamingfoundation/randapp/x/randapp"
	"github.com/tendermint/tendermint/consensus"
	"github.com/tendermint/tendermint/libs/events"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/types"
)

type OnChainDKG struct {
	cli       *context.Context
	txBldr    *authtxb.TxBuilder
	dealer    consensus.Dealer
	logger    l.Logger
	typesList []types.DKGDataType
	c         int
}

func NewOnChainDKG(cli *context.Context, txBldr *authtxb.TxBuilder, l l.Logger) *OnChainDKG {
	return &OnChainDKG{
		cli:    cli,
		txBldr: txBldr,
		logger: l,
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
		m.logger.Println("DEALS ARE READY", m.dealer.IsDealsReady())
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
		c := 0
		for _, msg := range messages {
			c++
			if err := handler(msg.Data); err != nil {
				return fmt.Errorf("failed to handle message: %v", err), false
			}
		}
		if c > 0 {
			m.logger.Println(fmt.Sprintf("GOT %d messages of type %d", c, dataType))
		}
	}

	if _, err := m.dealer.GetVerifier(); err == consensus.ErrDKGVerifierNotReady {
		return nil, false
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
	startRound int) error {
	m.dealer = consensus.NewDKGDealer(validators, pv, m.sendMsg, eventFirer, logger, startRound)
	if err := m.dealer.Start(); err != nil {
		return fmt.Errorf("failed to start dealer: %v", err)
	}

	return nil
}

func (m *OnChainDKG) sendMsg(data *types.DKGData) error {
	if data.Type == 1 {
		m.logger.Println("SENDING DEALS", data.Addr)
		m.c++
	}
	msg := randapp.NewMsgSendDKGData(data, m.cli.GetFromAddress())
	if err := msg.ValidateBasic(); err != nil {
		return err
	}

	err := utils.GenerateOrBroadcastMsgs(*m.cli, *m.txBldr, []sdk.Msg{msg}, false)
	tempTxBldr := m.txBldr.WithSequence(m.txBldr.Sequence() + 1)
	m.txBldr = &tempTxBldr
	return err
}

func (m *OnChainDKG) getDKGMessages(dataType types.DKGDataType) ([]*randapp.DKGData, error) {
	res, _, err := m.cli.QueryWithData(fmt.Sprintf("custom/randapp/dkgData/%d", dataType), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to query for DKG data: %v", err)
	}

	var data []*randapp.DKGData
	var dec = gob.NewDecoder(bytes.NewBuffer(res))
	if err := dec.Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode DKG data: %v", err)
	}

	m.logger.Println("DEALS NUMBER ", m.c)

	return data, nil
}

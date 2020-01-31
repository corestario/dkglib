package onChain

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"os"

	"github.com/tendermint/go-amino"

	authtxb "github.com/corestario/cosmos-utils/client/authtypes"
	"github.com/corestario/cosmos-utils/client/context"
	"github.com/corestario/cosmos-utils/client/utils"
	"github.com/corestario/dkglib/lib/alias"
	"github.com/corestario/dkglib/lib/dealer"
	"github.com/corestario/dkglib/lib/msgs"
	"github.com/corestario/dkglib/lib/types"
	"github.com/cosmos/cosmos-sdk/client/keys"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authTypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	tmtypes "github.com/tendermint/tendermint/alias"
	"github.com/tendermint/tendermint/libs/events"
	"github.com/tendermint/tendermint/libs/log"
)

type OnChainDKG struct {
	cli            *context.Context
	txBldr         *authtxb.TxBuilder
	dealer         dealer.Dealer
	typesList      []alias.DKGDataType
	logger         log.Logger
	OnChainParams  *OnChainParams
	uniqueMessages map[string]bool
}

type OnChainParams struct {
	Cdc          *amino.Codec
	ChainID      string
	NodeEndpoint string
	HomeString   string
}

func NewOnChainDKG(cli *context.Context, txBldr *authtxb.TxBuilder) *OnChainDKG {
	return &OnChainDKG{
		cli:            cli,
		txBldr:         txBldr,
		logger:         log.NewTMLogger(os.Stdout),
		uniqueMessages: make(map[string]bool),
	}
}

func (m *OnChainDKG) GetVerifier() (types.Verifier, error) {
	return m.dealer.GetVerifier()
}

func (m *OnChainDKG) ProcessBlock() (error, bool) {
	for _, dataType := range []alias.DKGDataType{
		alias.DKGPubKey,
		alias.DKGDeal,
		alias.DKGResponse,
		alias.DKGJustification,
		alias.DKGCommits,
		alias.DKGComplaint,
		alias.DKGReconstructCommit,
	} {
		messages, err := m.getDKGMessages(dataType)
		if err != nil {
			return fmt.Errorf("failed to getDKGMessages: %v", err), false
		}
		var handler func(msg *alias.DKGData) error
		switch dataType {
		case alias.DKGPubKey:
			handler = m.dealer.HandleDKGPubKey
		case alias.DKGDeal:
			handler = m.dealer.HandleDKGDeal
		case alias.DKGResponse:
			handler = m.dealer.HandleDKGResponse
		case alias.DKGJustification:
			handler = m.dealer.HandleDKGJustification
		case alias.DKGCommits:
			handler = m.dealer.HandleDKGCommit
		case alias.DKGComplaint:
			handler = m.dealer.HandleDKGComplaint
		case alias.DKGReconstructCommit:
			handler = m.dealer.HandleDKGReconstructCommit
		}
		for _, msg := range messages {
			if err := handler(msg.Data); err != nil {
				return fmt.Errorf("failed to handle message: %v", err), false
			}
		}
	}

	if _, err := m.dealer.GetVerifier(); err == types.ErrDKGVerifierNotReady {
		m.logger.Debug("Process Block: Verifier Not ready")
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
	m.dealer = dealer.NewDKGDealer(validators, pv, m.sendMsg, eventFirer, logger, startRound)
	if err := m.dealer.Start(); err != nil {
		m.logger.Debug("Start on-chain dkg")
		return fmt.Errorf("failed to start dealer: %v", err)
	}

	return nil
}

func (m *OnChainDKG) GetLosers() []*tmtypes.Validator {
	return m.dealer.GetLosers()
}

func (m *OnChainDKG) sendMsg(data []*alias.DKGData) error {
	var messages []sdk.Msg
	for _, item := range data {
		msg := msgs.NewMsgSendDKGData(item, m.cli.GetFromAddress())
		if err := msg.ValidateBasic(); err != nil {
			return fmt.Errorf("failed to validate basic: %v", err)
		}
		messages = append(messages, msg)
	}

	kb, err := keys.NewKeyBaseFromDir(m.cli.Home)
	if err != nil {
		m.logger.Error("on-chain DKG send msg error", "function", "NewKeyBaseFromDir", "error", err)
		return err
	}
	keysList, err := kb.List()
	if err != nil {
		m.logger.Error("on-chain DKG send msg error", "function", "List", "error", err)
		return err
	}
	if len(keysList) == 0 {
		err := fmt.Errorf("key list error: account does not exist")
		m.logger.Error("on-chain DKG send msg error", "error", err)
		return err
	}

	accRetriever := authTypes.NewAccountRetriever(m.cli)
	accNumber, accSequence, err := accRetriever.GetAccountNumberSequence(keysList[0].GetAddress())
	if err != nil {
		m.logger.Error("on-chain DKG send msg error", "function", "GetAccountNumberSequence", "error", err)
		return err
	}
	txBldr := authtxb.NewTxBuilder(
		utils.GetTxEncoder(m.OnChainParams.Cdc),
		accNumber,
		accSequence,
		400000*100,
		0.0,
		false,
		m.OnChainParams.ChainID,
		"",
		nil,
		nil,
	).WithKeybase(kb)

	m.txBldr = &txBldr

	err = utils.GenerateOrBroadcastMsgs(*m.cli, txBldr, messages, false)
	if err != nil {
		return fmt.Errorf("failed to broadcast msg: %v", err)
	}

	return nil
}

func (m *OnChainDKG) getDKGMessages(dataType alias.DKGDataType) ([]*msgs.MsgSendDKGData, error) {
	res, _, err := m.cli.QueryWithData(fmt.Sprintf("custom/randapp/dkgData/%d", dataType), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to query for DKG data: %v", err)
	}

	var data []*msgs.MsgSendDKGData
	var dec = gob.NewDecoder(bytes.NewBuffer(res))
	if err := dec.Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode DKG data: %v", err)
	}

	return data, nil
}

func (m *OnChainDKG) StartDKGRound(validators *tmtypes.ValidatorSet) error {
	return nil
}

func (m *OnChainDKG) IsOnChain() bool {
	return true
}

package dealer

import (
	"bytes"
	"encoding/gob"
	"encoding/hex"
	"errors"
	"fmt"
	"math"

	"github.com/corestario/dkglib/lib/blsShare"
	"go.dedis.ch/kyber/v3/share"

	"go.dedis.ch/kyber/v3"

	"github.com/tendermint/tendermint/crypto"

	"github.com/corestario/dkglib/lib/alias"
	"github.com/corestario/dkglib/lib/types"
	tmtypes "github.com/tendermint/tendermint/alias"
	"github.com/tendermint/tendermint/libs/events"
	"github.com/tendermint/tendermint/libs/log"
	dkg "go.dedis.ch/kyber/v3/share/dkg/pedersen"
	vss "go.dedis.ch/kyber/v3/share/vss/pedersen"
)

type onChainDealer struct {
	*DKGDealer
	instance *dkg.DistKeyGenerator
	deals    map[string]*dkg.Deal
}

func (d *onChainDealer) GenerateTransitions() {
	d.transitions = []transition{
		d.SendCommits,
		d.SendDeals,
		d.ProcessDeals,
		d.ProcessResponses,
	}
}

func NewOnChainDKGDealer(
	validators *tmtypes.ValidatorSet,
	pv tmtypes.PrivValidator,
	sendMsgCb func([]*alias.DKGData) error,
	eventFirer events.Fireable,
	logger log.Logger,
	startRound int,
) Dealer {
	return &onChainDealer{
		deals:     make(map[string]*dkg.Deal),
		DKGDealer: NewDKGDealer(validators, pv, sendMsgCb, eventFirer, logger, startRound).(*DKGDealer),
	}
}

func (d *onChainDealer) Start() error {
	d.secKey = d.suiteG2.Scalar().Pick(d.suiteG2.RandomStream())
	d.pubKey = d.suiteG2.Point().Mul(d.secKey, nil)

	d.GenerateTransitions()

	var (
		buf = bytes.NewBuffer(nil)
		enc = gob.NewEncoder(buf)
	)
	if err := enc.Encode(d.pubKey); err != nil {
		return fmt.Errorf("failed to encode public key: %v", err)
	}

	d.logger.Info("dkgState: sending pub key", "key", d.pubKey.String())
	err := d.SendMsgCb([]*alias.DKGData{{
		Type:    alias.DKGPubKey,
		RoundID: d.roundID,
		Addr:    d.addrBytes,
		Data:    buf.Bytes(),
	}})
	if err != nil {
		return fmt.Errorf("failed to sign message: %v", err)
	}

	return nil
}

func (d *onChainDealer) SendCommits() (error, bool) {
	if !d.IsPubKeysReady() {
		d.logger.Debug("DKG send commits: dealer is not ready")
		return nil, false
	}

	// TODO: fire event.

	instance, err := dkg.NewDistKeyGenerator(d.suiteG2, d.secKey, d.pubKeys.GetPKs(), d.validators.Size())
	if err != nil {
		return fmt.Errorf("failed to execute NewDistKeyGenerator: %w", err), false
	}
	d.instance = instance

	var commitMessages []*alias.DKGData
	for _, commit := range d.instance.GetDealer().Commits() {
		var (
			buf = bytes.NewBuffer(nil)
			enc = gob.NewEncoder(buf)
		)
		if err := enc.Encode(commit); err != nil {
			return fmt.Errorf("failed to encode public key: %v", err), false
		}
		commitMessages = append(commitMessages, &alias.DKGData{
			Type:    alias.DKGCommits,
			RoundID: d.roundID,
			Addr:    d.addrBytes,
			Data:    buf.Bytes(),
		})

	}

	err = d.SendMsgCb(commitMessages)
	if err != nil {
		return fmt.Errorf("failed to send commit: %v", err), false
	}

	d.logger.Debug("sent all commits ")

	return nil, true
}

func (d *onChainDealer) HandleDKGCommit(msg *alias.DKGData) error {
	dec := gob.NewDecoder(bytes.NewBuffer(msg.Data))
	commit := d.suiteG2.Point()

	if err := dec.Decode(commit); err != nil {
		d.losers = append(d.losers, crypto.Address(msg.Addr))
		return fmt.Errorf("failed to decode commit: %v", err)
	}
	d.commits.add(msg.GetAddrString(), 0, commit)

	if err := d.Transit(); err != nil {
		return fmt.Errorf("failed to Transit: %v", err)
	}

	return nil
}

func (d *onChainDealer) SendDeals() (error, bool) {
	d.logger.Debug("SendDeals, awaiting commits", "have", len(d.commits.addrToData), "want", d.validators.Size()-1)
	if len(d.commits.addrToData) != d.validators.Size()-1 {
		d.logger.Debug("DKG send deals: dealer is not ready", "have", len(d.commits.addrToData))
		return nil, false
	}
	d.eventFirer.FireEvent(types.EventDKGPubKeyReceived, nil)

	deals, err := d.instance.Deals()
	if err != nil {
		return fmt.Errorf("failed to get deals: %v", err), true
	}
	for _, deal := range deals {
		d.participantID = int(deal.Index) // Same for each deal.
		break
	}

	d.logger.Debug("SendDeals, generated deals", "num_deals", len(deals))

	var dealMessages []*alias.DKGData
	for toIndex, deal := range deals {
		buf, err := deal.Encode()
		if err != nil {
			return fmt.Errorf("SendDeals: failed to encode deal: %w", err), false
		}

		dealMessage := &alias.DKGData{
			Type:    alias.DKGDeal,
			RoundID: d.roundID,
			Addr:    d.addrBytes,
			Data:    buf,
			ToIndex: toIndex,
		}

		dealMessages = append(dealMessages, dealMessage)
	}

	d.logger.Debug("SendDeals, sending deal messages", "num_messages", len(dealMessages))

	if err = d.SendMsgCb(dealMessages); err != nil {
		return fmt.Errorf("failed to send deals: %v", err), true
	}

	d.logger.Info("dkgState: sent deals", "num_messages", len(dealMessages))

	return err, true
}

func (d *onChainDealer) HandleDKGDeal(msg *alias.DKGData) error {
	d.logger.Info("HandleDKGDeal: received Deal message", "from", msg.GetAddrString())
	var deal = &dkg.Deal{}
	if err := deal.Decode(msg.Data); err != nil {
		d.losers = append(d.losers, msg.Addr)
		return fmt.Errorf("HandleDKGDeal: failed to decode deal: %v", err)
	}

	// We expect to keep N - 1 deals (we don't care about the deals sent to other participants).
	if d.participantID != msg.ToIndex {
		d.logger.Debug("HandleDKGDeal: rejecting deal (intended for another participant)", "intended", msg.ToIndex)
		return nil
	}

	d.logger.Info("dkgState: deal is intended for us, storing")
	if _, exists := d.deals[msg.GetAddrString()]; exists {
		d.logger.Debug("HandleDKGDeal: deals message already exists", "roundID", msg.RoundID, "msgAddr", msg.Addr)
		return nil
	}

	d.deals[msg.GetAddrString()] = deal
	if err := d.Transit(); err != nil {
		return fmt.Errorf("HandleDKGDeal: failed to Transit: %v", err)
	}

	return nil
}

func (d *onChainDealer) IsDealsReady() bool {
	return len(d.deals) >= d.validators.Size()-1
}

func (d *onChainDealer) ProcessDeals() (error, bool) {
	d.logger.Debug("onChainDealer: ProcessDeals: awaiting deals", "have", len(d.deals), "want", d.validators.Size()-1)
	if !d.IsDealsReady() {
		d.logger.Debug("onChainDealer: ProcessDeals: process deals, deals are not ready")
		return nil, false
	}

	var responseMessages []*alias.DKGData
	for dealerID, deal := range d.deals {
		// Party does not have to verify its own deal.
		if deal.Index == uint32(d.participantID) {
			continue
		}
		resp, err := d.instance.ProcessDeal(deal)
		if err != nil {
			return err, false
		}

		// Commits verification.
		allVerifiers := d.instance.Verifiers()
		verifier := allVerifiers[deal.Index]
		commitsOK, _ := d.ProcessDealCommits(verifier, deal)

		// If something goes wrong, party complains.
		if !resp.Response.Status || !commitsOK {
			loserAddress := crypto.Address{}
			addrBytes, err := hex.DecodeString(dealerID)
			if err != nil {
				return fmt.Errorf("failed to decode loser address: %w", err), false
			}
			if err := loserAddress.Unmarshal(addrBytes); err != nil {
				return fmt.Errorf("failed to unmarshal loser address: %w", err), false
			}
			d.losers = append(d.losers, loserAddress)
		}

		var (
			buf = bytes.NewBuffer(nil)
			enc = gob.NewEncoder(buf)
		)
		if err := enc.Encode(resp); err != nil {
			return fmt.Errorf("failed to encode public key: %v", err), false
		}
		responseMessages = append(responseMessages, &alias.DKGData{
			Type:    alias.DKGResponse,
			RoundID: d.roundID,
			Addr:    d.addrBytes,
			Data:    buf.Bytes(),
		})
		fmt.Println("...............................................................", "DST INDEX", resp.Index, "SRC INDEX", resp.Response.Index)
	}

	d.logger.Debug("SendDeals, sending response messages", "num_messages", len(responseMessages))

	if err := d.SendMsgCb(responseMessages); err != nil {
		return fmt.Errorf("failed to send responses: %v", err), true
	}

	d.logger.Debug("DKG process deals success")
	return nil, true
}

func (d *onChainDealer) ProcessDealCommits(verifier *vss.Verifier, deal *dkg.Deal) (bool, error) {
	// Verifier decryptDeal.
	decryptedDeal, err := verifier.DecryptDeal(deal.Deal)
	if err != nil {
		return false, err
	}

	commitsData, ok := d.commits.indexToData[int(deal.Index)]
	if !ok {
		return false, err
	}
	var originalCommits []kyber.Point
	for _, commitData := range commitsData {
		commit, ok := commitData.(kyber.Point)
		if !ok {
			return false, fmt.Errorf("failed to cast commit data to commit type")
		}
		originalCommits = append(originalCommits, commit)
	}

	// Check that commits on the chain and commits in the deal are met

	if len(originalCommits) != len(decryptedDeal.Commitments) {
		return false, errors.New("number of original commitments and number of commitments in the deal are not met")
	}

	for i := range originalCommits {
		if !originalCommits[i].Equal(decryptedDeal.Commitments[i]) {
			return false, errors.New("commits are different")
		}
	}

	return true, nil
}

func (d *onChainDealer) HandleDKGResponse(msg *alias.DKGData) error {
	var (
		dec  = gob.NewDecoder(bytes.NewBuffer(msg.Data))
		resp = &dkg.Response{}
	)
	if err := dec.Decode(resp); err != nil {
		d.losers = append(d.losers, crypto.Address(msg.Addr))
		return fmt.Errorf("failed to response deal: %v", err)
	}

	// Unlike the procedure for deals, with responses we do care about other
	// participants state of affairs. All responses sent make N * (N - 1) responses,
	// but we skip the responses produced by  ourselves, which gives
	// N * (N - 1) - (N - 1) responses, which gives (N - 1) ^ 2 responses.
	if uint32(d.participantID) == resp.Response.Index {
		d.logger.Debug("dkgState: skipping response")
		return nil
	}

	d.logger.Info("dkgState: response is intended for us, storing")

	d.responses.add(msg.GetAddrString(), int(resp.Response.Index), resp)

	if err := d.Transit(); err != nil {
		return fmt.Errorf("failed to Transit: %v", err)
	}

	return nil
}

func (d *onChainDealer) ProcessResponses() (error, bool) {
	d.logger.Debug("onChainDealer: ProcessResponses: awaiting responses", "have", d.responses.messagesCount, "want", int(math.Pow(float64(d.validators.Size()-1), 2)))

	if !d.IsResponsesReady() {
		d.logger.Debug("DKGDealer process responses: responses are not ready")
		return nil, false
	}

	for _, peerResponses := range d.responses.indexToData {
		for _, response := range peerResponses {
			resp := response.(*dkg.Response)
			if int(resp.Response.Index) == d.participantID {
				continue
			}

			_, err := d.instance.ProcessResponse(resp)
			if err != nil {
				return fmt.Errorf("failed to ProcessResponse: %w", err), false
			}
		}
	}

	if !d.instance.Certified() {
		return fmt.Errorf("praticipant %v is not certified", d.participantID), false
	}

	return nil, true
}

func (d *onChainDealer) GetVerifier() (types.Verifier, error) {
	if d.instance == nil || !d.instance.Certified() {
		return nil, types.ErrDKGVerifierNotReady
	}

	distKeyShare, err := d.instance.DistKeyShare()
	if err != nil {
		return nil, fmt.Errorf("failed to get DistKeyShare: %v", err)
	}

	masterPubKey := share.NewPubPoly(d.suiteG2, nil, distKeyShare.Commitments())

	newShare := &blsShare.BLSShare{
		ID:   d.participantID,
		Pub:  &share.PubShare{I: d.participantID, V: d.pubKey},
		Priv: distKeyShare.PriShare(),
	}
	t, n := (d.validators.Size()/3)*2, d.validators.Size()

	verificationKey := masterPubKey.Eval(distKeyShare.PriShare().I)
	if verificationKey == nil {
		return nil, fmt.Errorf("can't get verification key for %v participant", d.participantID)
	}

	return blsShare.NewBLSVerifier(masterPubKey, newShare, t, n), nil
}

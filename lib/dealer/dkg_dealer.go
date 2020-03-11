package dealer

import (
	"bytes"
	"encoding/gob"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"sort"

	"github.com/corestario/dkglib/lib/alias"
	"github.com/corestario/dkglib/lib/blsShare"
	"github.com/corestario/dkglib/lib/types"
	tmtypes "github.com/tendermint/tendermint/alias"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/libs/events"
	"github.com/tendermint/tendermint/libs/log"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/pairing/bn256"
	"go.dedis.ch/kyber/v3/share"
	dkg "go.dedis.ch/kyber/v3/share/dkg/rabin"
	vss "go.dedis.ch/kyber/v3/share/vss/rabin"
)

type Dealer interface {
	Start() error
	GetState() DealerState
	Transit() error
	GenerateTransitions()
	GetLosers() []*tmtypes.Validator
	PopLosers() []*tmtypes.Validator
	HandleDKGPubKey(msg *alias.DKGData) error
	SetTransitions(t []transition)
	SendDeals() (err error, ready bool)
	IsPubKeysReady() bool
	GetDeals() ([]*alias.DKGData, error)
	HandleDKGDeal(msg *alias.DKGData) error
	ProcessDeals() (err error, ready bool)
	IsDealsReady() bool
	GetResponses() ([]*alias.DKGData, error)
	HandleDKGResponse(msg *alias.DKGData) error
	ProcessResponses() (err error, ready bool)
	HandleDKGJustification(msg *alias.DKGData) error
	ProcessJustifications() (err error, ready bool)
	IsResponsesReady() bool
	GetJustifications() ([]*alias.DKGData, error)
	HandleDKGCommit(msg *alias.DKGData) error
	ProcessCommits() (err error, ready bool)
	IsJustificationsReady() bool
	GetCommits() (*dkg.SecretCommits, error)
	HandleDKGComplaint(msg *alias.DKGData) error
	ProcessComplaints() (err error, ready bool)
	HandleDKGReconstructCommit(msg *alias.DKGData) error
	ProcessReconstructCommits() (err error, ready bool)
	GetVerifier() (types.Verifier, error)
	SendMsgCb([]*alias.DKGData) error
	VerifyMessage(msg types.DKGDataMessage) error
}

type DKGDealer struct {
	DealerState
	eventFirer events.Fireable

	sendMsgCb func([]*alias.DKGData) error
	logger    log.Logger

	pubKey      kyber.Point
	secKey      kyber.Scalar
	suiteG1     *bn256.Suite
	suiteG2     *bn256.Suite
	instance    *dkg.DistKeyGenerator
	transitions []transition

	pubKeys            PKStore
	deals              map[string]*dkg.Deal
	responses          *messageStore
	justifications     *messageStore
	commits            *messageStore
	complaints         *messageStore
	reconstructCommits *messageStore

	losers []crypto.Address
}

type DealerState struct {
	validators *tmtypes.ValidatorSet
	addrBytes  []byte

	participantID int
	roundID       int
}

func (ds DealerState) GetValidatorsCount() int {
	if ds.validators == nil {
		return 0
	}
	return ds.validators.Size()
}

func (ds DealerState) GetRoundID() int { return ds.roundID }

type DKGDealerConstructor func(validators *tmtypes.ValidatorSet, pv tmtypes.PrivValidator, sendMsgCb func([]*alias.DKGData) error, eventFirer events.Fireable, logger log.Logger, startRound int) Dealer

func NewDKGDealer(validators *tmtypes.ValidatorSet, pv tmtypes.PrivValidator, sendMsgCb func([]*alias.DKGData) error, eventFirer events.Fireable, logger log.Logger, startRound int) Dealer {
	return &DKGDealer{
		DealerState: DealerState{
			validators: validators,
			addrBytes:  pv.GetPubKey().Address().Bytes(),
			roundID:    startRound,
		},
		sendMsgCb:  sendMsgCb,
		eventFirer: eventFirer,
		logger:     logger,
		suiteG1:    bn256.NewSuiteG1(),
		suiteG2:    bn256.NewSuiteG2(),

		responses:          newMessageStore(validators.Size() - 1),
		justifications:     newMessageStore(int(math.Pow(float64(validators.Size()-1), 2))),
		commits:            newMessageStore(1),
		complaints:         newMessageStore(1),
		reconstructCommits: newMessageStore(1),

		deals: make(map[string]*dkg.Deal),
	}
}

func (d *DKGDealer) Start() error {
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

func (d *DKGDealer) GetState() DealerState {
	return d.DealerState
}

func (d *DKGDealer) Transit() error {
	for len(d.transitions) > 0 {
		var tn = d.transitions[0]
		err, ready := tn()
		if !ready {
			d.logger.Debug("DKGDealer Transition not ready", "transition current length", len(d.transitions))
			return nil
		}
		if err != nil {
			d.logger.Info("DKGDealer Transit failed", "transition current length", len(d.transitions), "error", err)
			return err
		}
		d.transitions = d.transitions[1:]
	}

	return nil
}

func (d *DKGDealer) GenerateTransitions() {
	d.transitions = []transition{
		// Phase I
		d.SendDeals,
		d.ProcessDeals,
		d.ProcessResponses,
		d.ProcessJustifications,
		// Phase II
		d.ProcessCommits,
		d.ProcessComplaints,
		d.ProcessReconstructCommits,
	}
}

func (d *DKGDealer) SetTransitions(t []transition) {
	d.transitions = t
}

func (d *DKGDealer) GetLosers() []*tmtypes.Validator {
	var out []*tmtypes.Validator
	for _, loser := range d.losers {
		_, validator := d.validators.GetByAddress(loser)
		d.logger.Debug("got looser", "address", loser, "validator", validator.String())
		out = append(out, validator)
	}

	return out
}

func (d *DKGDealer) PopLosers() []*tmtypes.Validator {
	out := d.GetLosers()
	d.losers = nil
	return out
}

//////////////////////////////////////////////////////////////////////////////
//
// PHASE I
//
//////////////////////////////////////////////////////////////////////////////

func (d *DKGDealer) HandleDKGPubKey(msg *alias.DKGData) error {
	var (
		dec    = gob.NewDecoder(bytes.NewBuffer(msg.Data))
		pubKey = d.suiteG2.Point()
	)
	if err := dec.Decode(pubKey); err != nil {
		d.losers = append(d.losers, crypto.Address(msg.Addr))
		return fmt.Errorf("dkgState: failed to decode public key from %s: %v", msg.Addr, err)
	}
	// TODO: check if we want to slash validators who send duplicate keys
	// (we probably do).
	d.pubKeys.Add(&PK2Addr{PK: pubKey, Addr: crypto.Address(msg.Addr)})

	if err := d.Transit(); err != nil {
		return fmt.Errorf("failed to Transit: %v", err)
	}

	return nil
}

func (d *DKGDealer) SendDeals() (error, bool) {
	if !d.IsPubKeysReady() {
		d.logger.Debug("DKG send deals: dealer is not ready")
		return nil, false
	}
	d.eventFirer.FireEvent(types.EventDKGPubKeyReceived, nil)

	messages, err := d.GetDeals()
	if err != nil {
		return fmt.Errorf("failed to get deals: %v", err), true
	}

	if err = d.SendMsgCb(messages); err != nil {
		return fmt.Errorf("failed to sign message: %v", err), true
	}

	d.logger.Info("dkgState: sending deals", "deals", len(messages))

	return err, true
}

func (d *DKGDealer) IsPubKeysReady() bool {
	return len(d.pubKeys) == d.validators.Size()
}

func (d *DKGDealer) GetDeals() ([]*alias.DKGData, error) {
	d.logger.Debug("DKGDealer get deals start")
	// It's needed for DistKeyGenerator and for binary search in array
	sort.Sort(d.pubKeys)
	dkgInstance, err := dkg.NewDistKeyGenerator(d.suiteG2, d.secKey, d.pubKeys.GetPKs(), (d.validators.Size()*2)/3)
	if err != nil {
		return nil, fmt.Errorf("failed to create dkgState instance: %v", err)
	}
	d.instance = dkgInstance

	// We have N - 1 deals produced here (here and below N stands for the number of validators).
	deals, err := d.instance.Deals()
	if err != nil {
		return nil, fmt.Errorf("failed to populate deals: %v", err)
	}
	for _, deal := range deals {
		d.participantID = int(deal.Index) // Same for each deal.
		break
	}

	var dealMessages []*alias.DKGData
	for toIndex, deal := range deals {
		var (
			buf = bytes.NewBuffer(nil)
			enc = gob.NewEncoder(buf)
		)

		if err := enc.Encode(deal); err != nil {
			return dealMessages, fmt.Errorf("failed to encode deal #%d: %v", deal.Index, err)
		}

		dealMessage := &alias.DKGData{
			Type:    alias.DKGDeal,
			RoundID: d.roundID,
			Addr:    d.addrBytes,
			Data:    buf.Bytes(),
			ToIndex: toIndex,
		}

		dealMessages = append(dealMessages, dealMessage)
	}

	d.logger.Info("DKGDealer get deals success")
	return dealMessages, nil
}

func (d *DKGDealer) HandleDKGDeal(msg *alias.DKGData) error {
	var (
		dec  = gob.NewDecoder(bytes.NewBuffer(msg.Data))
		deal = &dkg.Deal{ // We need to initialize everything down to the kyber.Point to avoid nil panics.
			Deal: &vss.EncryptedDeal{
				DHKey: d.suiteG2.Point(),
			},
		}
	)
	if err := dec.Decode(deal); err != nil {
		d.losers = append(d.losers, crypto.Address(msg.Addr))
		return fmt.Errorf("failed to decode deal: %v", err)
	}

	// We expect to keep N - 1 deals (we don't care about the deals sent to other participants).
	if d.participantID != msg.ToIndex {
		d.logger.Debug("dkgState: rejecting deal (intended for another participant)", "intended", msg.ToIndex, "own_index", d.participantID)
		return nil
	}

	d.logger.Info("dkgState: deal is intended for us, storing")
	if _, exists := d.deals[msg.GetAddrString()]; exists {
		d.logger.Debug("DKGDealer deals message already exists", "roundID", msg.RoundID, "msgAddr", msg.Addr)
		return nil
	}

	d.deals[msg.GetAddrString()] = deal
	if err := d.Transit(); err != nil {
		return fmt.Errorf("failed to Transit: %v", err)
	}

	return nil
}

func (d *DKGDealer) ProcessDeals() (error, bool) {
	if !d.IsDealsReady() {
		d.logger.Debug("DKGDealer process deals, deals are not ready")
		return nil, false
	}

	d.logger.Info("dkgState: processing deals")
	responseMessages, err := d.GetResponses()
	if err != nil {
		return fmt.Errorf("failed to get responses: %v", err), true
	}

	if err = d.SendMsgCb(responseMessages); err != nil {
		return fmt.Errorf("failed to sign message: %v", err), true
	}

	d.logger.Debug("DKG process deals success")
	return err, true
}

func (d *DKGDealer) IsDealsReady() bool {
	return len(d.deals) >= d.validators.Size()-1
}

func (d *DKGDealer) GetResponses() ([]*alias.DKGData, error) {
	var messages []*alias.DKGData
	d.logger.Debug("DKGDealer get responses start")
	// Each deal produces a response for the deal's issuer (that makes N - 1 responses).
	for _, deal := range d.deals {
		resp, err := d.instance.ProcessDeal(deal)
		if err != nil {
			return messages, fmt.Errorf("failed to ProcessDeal: %v", err)
		}
		var (
			buf = bytes.NewBuffer(nil)
			enc = gob.NewEncoder(buf)
		)
		if err := enc.Encode(resp); err != nil {
			return messages, fmt.Errorf("failed to encode response: %v", err)
		}

		messages = append(messages, &alias.DKGData{
			Type:    alias.DKGResponse,
			RoundID: d.roundID,
			Addr:    d.addrBytes,
			Data:    buf.Bytes(),
		})
	}
	d.eventFirer.FireEvent(types.EventDKGDealsProcessed, d.roundID)

	d.logger.Debug("DKGDealer get responses finish")
	return messages, nil
}

func (d *DKGDealer) HandleDKGResponse(msg *alias.DKGData) error {
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

	d.responses.add(msg.GetAddrString(), 0, resp)

	if err := d.Transit(); err != nil {
		return fmt.Errorf("failed to Transit: %v", err)
	}

	return nil
}

func (d *DKGDealer) ProcessResponses() (error, bool) {
	if !d.IsResponsesReady() {
		d.logger.Debug("DKGDealer process responses: responses are not ready")
		return nil, false
	}

	messages, err := d.GetJustifications()
	if err != nil {
		return fmt.Errorf("failed to get justifications: %v", err), true
	}

	if err = d.SendMsgCb(messages); err != nil {
		return fmt.Errorf("failed to sign message: %v", err), true
	}

	d.logger.Debug("DKG process responses success")
	return err, true
}

func (d *DKGDealer) IsResponsesReady() bool {
	return d.responses.messagesCount >= int(math.Pow(float64(d.validators.Size()-1), 2))
}

func (d *DKGDealer) processResponse(resp *dkg.Response) ([]byte, error) {
	if resp.Response.Approved {
		d.logger.Info("dkgState: deal is approved", "to", resp.Index, "from", resp.Response.Index)
	}

	justification, err := d.instance.ProcessResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to ProcessResponse: %v", err)
	}
	if justification == nil {
		d.logger.Debug("justification is nil")
		return nil, nil
	}

	var (
		buf = bytes.NewBuffer(nil)
		enc = gob.NewEncoder(buf)
	)
	if err := enc.Encode(justification); err != nil {
		return nil, fmt.Errorf("failed to encode response: %v", err)
	}

	return buf.Bytes(), nil
}

func (d *DKGDealer) GetJustifications() ([]*alias.DKGData, error) {
	var messages []*alias.DKGData
	d.logger.Debug("DKG delaer get justification start")
	for _, peerResponses := range d.responses.addrToData {
		for _, response := range peerResponses {
			resp := response.(*dkg.Response)
			var msg = &alias.DKGData{
				Type:    alias.DKGJustification,
				RoundID: d.roundID,
				Addr:    d.addrBytes,
			}

			// Each of (N - 1) ^ 2 received response generates a (possibly nil) justification.
			// Nil justifications (and other nil messages) are used to avoid having timeouts
			// (i.e., this allows us to know exactly how many messages should be received to
			// proceed). This might be changed in the future.
			justificationBytes, err := d.processResponse(resp)
			if err != nil {
				return messages, err
			}

			msg.Data = justificationBytes
			// We will nave N * (N - 1) ^ 2 justifications. This looks rather bad, actually
			messages = append(messages, msg)
		}
	}

	d.logger.Debug("DKG dealer get justification finish")
	d.eventFirer.FireEvent(types.EventDKGResponsesProcessed, d.roundID)
	return messages, nil
}

func (d *DKGDealer) HandleDKGJustification(msg *alias.DKGData) error {
	var justification *dkg.Justification
	if msg.Data != nil {
		dec := gob.NewDecoder(bytes.NewBuffer(msg.Data))
		justification = &dkg.Justification{}
		if err := dec.Decode(justification); err != nil {
			d.losers = append(d.losers, crypto.Address(msg.Addr))
			return fmt.Errorf("failed to decode justification: %v", err)
		}
	}

	d.justifications.add(msg.GetAddrString(), 0, justification)

	if err := d.Transit(); err != nil {
		return fmt.Errorf("failed to Transit: %v", err)
	}

	return nil
}

func (d *DKGDealer) ProcessJustifications() (error, bool) {
	if !d.IsJustificationsReady() {
		d.logger.Debug("justifications are not ready")

		return nil, false
	}
	d.logger.Info("dkgState: processing justifications")

	commits, err := d.GetCommits()
	if err != nil {
		d.logger.Debug("DKG dealer process justification failed", "error", err)
		return err, true
	}

	var (
		buf = bytes.NewBuffer(nil)
		enc = gob.NewEncoder(buf)
	)
	if err = enc.Encode(commits); err != nil {
		return fmt.Errorf("failed to encode response: %v", err), true
	}

	message := &alias.DKGData{
		Type:        alias.DKGCommits,
		RoundID:     d.roundID,
		Addr:        d.addrBytes,
		Data:        buf.Bytes(),
		NumEntities: len(commits.Commitments),
	}

	err = d.SendMsgCb([]*alias.DKGData{message})
	if err != nil {
		return fmt.Errorf("failed to sign message: %v", err), true
	}

	d.logger.Debug("DKG process justifications success")
	return nil, true
}

func (d *DKGDealer) IsJustificationsReady() bool {
	// N * (N - 1) ^ 2.
	return d.justifications.messagesCount >= d.validators.Size()*int(math.Pow(float64(d.validators.Size()-1), 2))
}

func (d DKGDealer) GetCommits() (*dkg.SecretCommits, error) {
	for _, peerJustifications := range d.justifications.addrToData {
		for _, just := range peerJustifications {
			justification := just.(*dkg.Justification)
			if justification != nil {
				d.logger.Info("dkgState: processing non-empty justification", "from", justification.Index)
				if err := d.instance.ProcessJustification(justification); err != nil {
					return nil, fmt.Errorf("failed to ProcessJustification: %v", err)
				}
			} else {
				d.logger.Info("dkgState: empty justification, everything is o.k.")
			}
		}
	}
	d.eventFirer.FireEvent(types.EventDKGJustificationsProcessed, d.roundID)

	if !d.instance.Certified() {
		return nil, errors.New("instance is not certified")
	}
	d.eventFirer.FireEvent(types.EventDKGInstanceCertified, d.roundID)

	qual := d.instance.QUAL()
	d.logger.Info("dkgState: got the QUAL set", "qual", qual)
	if len(qual) < d.validators.Size() {
		qualSet := map[int]bool{}
		for _, idx := range qual {
			qualSet[idx] = true
		}

		for idx, pk2addr := range d.pubKeys {
			if !qualSet[idx] {
				d.losers = append(d.losers, pk2addr.Addr)
			}
		}

		return nil, errors.New("some of participants failed to complete phase I")
	}

	commits, err := d.instance.SecretCommits()
	if err != nil {
		return nil, fmt.Errorf("failed to get commits: %v", err)
	}

	return commits, nil
}

//////////////////////////////////////////////////////////////////////////////
//
// PHASE II
//
//////////////////////////////////////////////////////////////////////////////

func (d *DKGDealer) HandleDKGCommit(msg *alias.DKGData) error {
	dec := gob.NewDecoder(bytes.NewBuffer(msg.Data))
	commits := &dkg.SecretCommits{}
	for i := 0; i < msg.NumEntities; i++ {
		commits.Commitments = append(commits.Commitments, d.suiteG2.Point())
	}
	if err := dec.Decode(commits); err != nil {
		d.losers = append(d.losers, crypto.Address(msg.Addr))
		return fmt.Errorf("failed to decode commit: %v", err)
	}
	d.commits.add(msg.GetAddrString(), 0, commits)

	if err := d.Transit(); err != nil {
		return fmt.Errorf("failed to Transit: %v", err)
	}

	return nil
}

func (d *DKGDealer) ProcessCommits() (error, bool) {
	if d.commits.messagesCount < len(d.instance.QUAL()) {
		d.logger.Debug("commits messages count is not enough", "commits", d.commits.messagesCount, "qual len", len(d.instance.QUAL()))
		return nil, false
	}
	d.logger.Info("dkgState: processing commits")

	var alreadyFinished = true
	var messages []*alias.DKGData
	for _, commitsFromAddr := range d.commits.addrToData {
		for _, c := range commitsFromAddr {
			commits := c.(*dkg.SecretCommits)
			var msg = &alias.DKGData{
				Type:    alias.DKGComplaint,
				RoundID: d.roundID,
				Addr:    d.addrBytes,
			}
			complaint, err := d.instance.ProcessSecretCommits(commits)
			if err != nil {
				return fmt.Errorf("failed to ProcessSecretCommits: %v", err), true
			}
			// TODO: check if we *really* need to add the complained dealer to losers.
			if complaint != nil {
				alreadyFinished = false
				var (
					buf = bytes.NewBuffer(nil)
					enc = gob.NewEncoder(buf)
				)
				if err := enc.Encode(complaint); err != nil {
					return fmt.Errorf("failed to encode response: %v", err), true
				}
				msg.Data = buf.Bytes()
				msg.NumEntities = len(complaint.Deal.Commitments)
			}
			messages = append(messages, msg)
		}
	}
	d.eventFirer.FireEvent(types.EventDKGCommitsProcessed, d.roundID)

	if !alreadyFinished {
		for _, msg := range messages {
			if err := d.SendMsgCb([]*alias.DKGData{msg}); err != nil {
				return fmt.Errorf("failed to sign message: %v", err), true
			}

		}
	}

	d.logger.Debug("DKG process commits success")
	return nil, true
}

func (d *DKGDealer) HandleDKGComplaint(msg *alias.DKGData) error {
	var complaint *dkg.ComplaintCommits
	if msg.Data != nil {
		dec := gob.NewDecoder(bytes.NewBuffer(msg.Data))
		complaint = &dkg.ComplaintCommits{
			Deal: &vss.Deal{},
		}
		for i := 0; i < msg.NumEntities; i++ {
			complaint.Deal.Commitments = append(complaint.Deal.Commitments, d.suiteG2.Point())
		}
		if err := dec.Decode(complaint); err != nil {
			d.losers = append(d.losers, crypto.Address(msg.Addr))
			return fmt.Errorf("failed to decode complaint: %v", err)
		}
	}

	d.complaints.add(msg.GetAddrString(), 0, complaint)

	if err := d.Transit(); err != nil {
		return fmt.Errorf("failed to Transit: %v", err)
	}

	return nil
}

func (d *DKGDealer) ProcessComplaints() (error, bool) {
	if d.complaints.messagesCount < len(d.instance.QUAL())-1 {
		d.logger.Debug("complaints messages count is not enough", "commits", d.complaints.messagesCount, "qual len", len(d.instance.QUAL())-1)
		return nil, false
	}
	d.logger.Info("dkgState: processing commits")

	for _, peerComplaints := range d.complaints.addrToData {
		for _, c := range peerComplaints {
			complaint := c.(*dkg.ComplaintCommits)
			var msg = &alias.DKGData{
				Type:    alias.DKGReconstructCommit,
				RoundID: d.roundID,
				Addr:    d.addrBytes,
			}
			if complaint != nil {
				reconstructionMsg, err := d.instance.ProcessComplaintCommits(complaint)
				if err != nil {
					return fmt.Errorf("failed to ProcessComplaintCommits: %v", err), true
				}
				if reconstructionMsg != nil {
					var (
						buf = bytes.NewBuffer(nil)
						enc = gob.NewEncoder(buf)
					)
					if err = enc.Encode(complaint); err != nil {
						return fmt.Errorf("failed to encode response: %v", err), true
					}
					msg.Data = buf.Bytes()
				}
			}

			if err := d.SendMsgCb([]*alias.DKGData{msg}); err != nil {
				return fmt.Errorf("failed to sign message: %v", err), true
			}

		}
	}
	d.logger.Debug("DKG process complaints success")
	d.eventFirer.FireEvent(types.EventDKGComplaintProcessed, d.roundID)
	return nil, true
}

func (d *DKGDealer) HandleDKGReconstructCommit(msg *alias.DKGData) error {
	var rc *dkg.ReconstructCommits
	if msg.Data != nil {
		dec := gob.NewDecoder(bytes.NewBuffer(msg.Data))
		rc = &dkg.ReconstructCommits{}
		if err := dec.Decode(rc); err != nil {
			d.losers = append(d.losers, crypto.Address(msg.Addr))
			return fmt.Errorf("failed to decode complaint: %v", err)
		}
	}

	d.reconstructCommits.add(msg.GetAddrString(), 0, rc)

	if err := d.Transit(); err != nil {
		return fmt.Errorf("failed to Transit: %v", err)
	}

	return nil
}

func (d *DKGDealer) ProcessReconstructCommits() (error, bool) {
	if d.reconstructCommits.messagesCount < len(d.instance.QUAL())-1 {
		d.logger.Debug("reconstruct commits low messages count", "messages count", d.reconstructCommits.messagesCount,
			"QUAL - 1", len(d.instance.QUAL())-1)
		return nil, false
	}

	for _, peerReconstructCommits := range d.reconstructCommits.addrToData {
		for _, reconstructCommit := range peerReconstructCommits {
			rc := reconstructCommit.(*dkg.ReconstructCommits)
			if rc == nil {
				continue
			}
			if err := d.instance.ProcessReconstructCommits(rc); err != nil {
				return fmt.Errorf("failed to ProcessReconstructCommits: %v", err), true
			}
		}
	}
	d.eventFirer.FireEvent(types.EventDKGReconstructCommitsProcessed, d.roundID)

	if !d.instance.Finished() {
		return errors.New("dkgState round is finished, but dkgState instance is not ready"), true
	}
	d.logger.Debug("DKG process reconstruct commits success")
	return nil, true
}

func (d *DKGDealer) GetVerifier() (types.Verifier, error) {
	if d.instance == nil || !d.instance.Finished() {
		return nil, types.ErrDKGVerifierNotReady
	}

	distKeyShare, err := d.instance.DistKeyShare()
	if err != nil {
		return nil, fmt.Errorf("failed to get DistKeyShare: %v", err)
	}

	var (
		masterPubKey = share.NewPubPoly(bn256.NewSuiteG2(), nil, distKeyShare.Commitments())
		newShare     = &blsShare.BLSShare{
			ID:   d.participantID,
			Pub:  &share.PubShare{I: d.participantID, V: d.pubKey},
			Priv: distKeyShare.PriShare(),
		}
		t, n = (d.validators.Size() / 3) * 2 + 1, d.validators.Size()
	)

	return blsShare.NewBLSVerifier(masterPubKey, newShare, t, n), nil
}

// VerifyMessage verify message by signature
func (d *DKGDealer) VerifyMessage(msg types.DKGDataMessage) error {
	var (
		signBytes []byte
	)
	_, validator := d.validators.GetByAddress(msg.Data.Addr)
	if validator == nil {
		return fmt.Errorf("can't find validator by address: %s", msg.Data.GetAddrString())
	}

	signBytes = msg.Data.SignBytes("")
	if !validator.PubKey.VerifyBytes(signBytes, msg.Data.Signature) {
		return fmt.Errorf("invalid DKG message signature: %s", hex.EncodeToString(msg.Data.Signature))
	}
	return nil
}

func (d *DKGDealer) SendMsgCb(msg []*alias.DKGData) error {
	return d.sendMsgCb(msg)
}

type PK2Addr struct {
	Addr crypto.Address
	PK   kyber.Point
}

type PKStore []*PK2Addr

func (s *PKStore) Add(newPk *PK2Addr) bool {
	for _, pk := range *s {
		if pk.Addr.String() == newPk.Addr.String() && pk.PK.Equal(newPk.PK) {
			return false
		}
	}
	*s = append(*s, newPk)

	return true
}

func (s PKStore) Len() int           { return len(s) }
func (s PKStore) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s PKStore) Less(i, j int) bool { return s[i].Addr.String() < s[j].Addr.String() }
func (s PKStore) GetPKs() []kyber.Point {
	var out = make([]kyber.Point, len(s))
	for idx, val := range s {
		out[idx] = val.PK
	}
	return out
}

type transition func() (error, bool)

type Justification struct {
	Void          bool
	Justification *dkg.Justification
}

// messageStore is used to store only required number of messages from every peer
type messageStore struct {
	// Common number of messages of the same type from peers
	messagesCount int

	// Max number of messages of the same type from one peer per round
	maxMessagesFromPeer int

	// Map which stores messages. Key is a peer's address, value is data
	addrToData map[string][]interface{}

	// Map which stores messages (same as addrToData). Key is a peer's index, value is data.
	indexToData map[int][]interface{}
}

func newMessageStore(n int) *messageStore {
	return &messageStore{
		maxMessagesFromPeer: n,
		addrToData:          make(map[string][]interface{}),
		indexToData:         make(map[int][]interface{}),
	}
}

func (ms *messageStore) add(addr string, index int, val interface{}) {
	data := ms.addrToData[addr]
	if len(data) == ms.maxMessagesFromPeer {
		return
	}
	data = append(data, val)
	ms.addrToData[addr] = data

	data = ms.indexToData[index]
	data = append(data, val)
	ms.indexToData[index] = data

	ms.messagesCount++
}

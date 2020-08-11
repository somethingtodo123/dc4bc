package dkg_proposal_fsm

import (
	"errors"
	"fmt"
	"github.com/depools/dc4bc/fsm/config"
	"github.com/depools/dc4bc/fsm/fsm"
	"github.com/depools/dc4bc/fsm/state_machines/internal"
	"github.com/depools/dc4bc/fsm/types/requests"
	"time"
)

// Pub keys

func (m *DKGProposalFSM) actionPubKeyConfirmationReceived(inEvent fsm.Event, args ...interface{}) (outEvent fsm.Event, response interface{}, err error) {
	m.payloadMu.Lock()
	defer m.payloadMu.Unlock()

	if len(args) != 1 {
		err = errors.New("{arg0} required {DKGProposalPubKeyConfirmationRequest}")
		return
	}

	request, ok := args[0].(requests.DKGProposalPubKeyConfirmationRequest)

	if !ok {
		err = errors.New("cannot cast {arg0} to type {DKGProposalPubKeyConfirmationRequest}")
		return
	}

	if err = request.Validate(); err != nil {
		return
	}

	dkgProposalParticipant, ok := m.payload.DKGProposalPayload[request.ParticipantId]

	if !ok {
		err = errors.New("{ParticipantId} not exist in quorum")
		return
	}

	if dkgProposalParticipant.Status != internal.PubKeyAwaitConfirmation {
		err = errors.New(fmt.Sprintf("cannot confirm pubkey with {Status} = {\"%s\"}", dkgProposalParticipant.Status))
		return
	}

	copy(dkgProposalParticipant.PubKey, request.PubKey)
	dkgProposalParticipant.UpdatedAt = request.CreatedAt
	dkgProposalParticipant.Status = internal.PubKeyConfirmed

	m.payload.DKGProposalPayload[request.ParticipantId] = dkgProposalParticipant

	return
}

func (m *DKGProposalFSM) actionValidateDkgProposalPubKeys(inEvent fsm.Event, args ...interface{}) (outEvent fsm.Event, response interface{}, err error) {
	var (
		isContainsError, isContainsExpired bool
	)

	m.payloadMu.Lock()
	defer m.payloadMu.Unlock()

	tm := time.Now()

	unconfirmedParticipants := len(m.payload.DKGProposalPayload)
	for _, participant := range m.payload.DKGProposalPayload {
		if participant.Status == internal.PubKeyAwaitConfirmation {
			if participant.UpdatedAt.Add(config.DkgConfirmationDeadline).Before(tm) {
				isContainsExpired = true
			}
		} else {
			if participant.Status == internal.PubKeyConfirmationError {
				isContainsError = true
			} else if participant.Status == internal.PubKeyConfirmed {
				unconfirmedParticipants--
			}
		}
	}

	if isContainsError {
		outEvent = eventDKGSetPubKeysConfirmationCanceledByErrorInternal
		return
	}

	if isContainsExpired {
		outEvent = eventDKGSetPubKeysConfirmationCanceledByTimeoutInternal
		return
	}

	// The are no declined and timed out participants, check for all confirmations
	if unconfirmedParticipants > 0 {
		return
	}

	outEvent = eventDKGSetPubKeysConfirmedInternal

	for _, participant := range m.payload.DKGProposalPayload {
		participant.Status = internal.CommitAwaitConfirmation
	}

	return
}

// Commits

func (m *DKGProposalFSM) actionCommitConfirmationReceived(inEvent fsm.Event, args ...interface{}) (outEvent fsm.Event, response interface{}, err error) {
	m.payloadMu.Lock()
	defer m.payloadMu.Unlock()

	if len(args) != 1 {
		err = errors.New("{arg0} required {DKGProposalCommitConfirmationRequest}")
		return
	}

	request, ok := args[0].(requests.DKGProposalCommitConfirmationRequest)

	if !ok {
		err = errors.New("cannot cast {arg0} to type {DKGProposalCommitConfirmationRequest}")
		return
	}

	if err = request.Validate(); err != nil {
		return
	}

	dkgProposalParticipant, ok := m.payload.DKGProposalPayload[request.ParticipantId]

	if !ok {
		err = errors.New("{ParticipantId} not exist in quorum")
		return
	}

	if dkgProposalParticipant.Status != internal.CommitAwaitConfirmation {
		err = errors.New(fmt.Sprintf("cannot confirm commit with {Status} = {\"%s\"}", dkgProposalParticipant.Status))
		return
	}

	copy(dkgProposalParticipant.Commit, request.Commit)
	dkgProposalParticipant.UpdatedAt = request.CreatedAt
	dkgProposalParticipant.Status = internal.CommitConfirmed

	m.payload.DKGProposalPayload[request.ParticipantId] = dkgProposalParticipant

	return
}

func (m *DKGProposalFSM) actionValidateDkgProposalAwaitCommits(inEvent fsm.Event, args ...interface{}) (outEvent fsm.Event, response interface{}, err error) {
	var (
		isContainsError, isContainsExpired bool
	)

	m.payloadMu.Lock()
	defer m.payloadMu.Unlock()

	tm := time.Now()

	unconfirmedParticipants := len(m.payload.DKGProposalPayload)
	for _, participant := range m.payload.DKGProposalPayload {
		if participant.Status == internal.CommitAwaitConfirmation {
			if participant.UpdatedAt.Add(config.SignatureProposalConfirmationDeadline).Before(tm) {
				isContainsExpired = true
			}
		} else {
			if participant.Status == internal.CommitConfirmationError {
				isContainsError = true
			} else if participant.Status == internal.CommitConfirmed {
				unconfirmedParticipants--
			}
		}
	}

	if isContainsError {
		outEvent = eventDKGCommitsConfirmationCancelByTimeoutInternal
		return
	}

	if isContainsExpired {
		outEvent = eventDKGCommitsConfirmationCancelByErrorInternal
		return
	}

	// The are no declined and timed out participants, check for all confirmations
	if unconfirmedParticipants > 0 {
		return
	}

	outEvent = eventDKGCommitsConfirmedInternal

	for _, participant := range m.payload.DKGProposalPayload {
		participant.Status = internal.DealAwaitConfirmation
	}

	return
}

// Deals

func (m *DKGProposalFSM) actionDealConfirmationReceived(inEvent fsm.Event, args ...interface{}) (outEvent fsm.Event, response interface{}, err error) {
	m.payloadMu.Lock()
	defer m.payloadMu.Unlock()

	if len(args) != 1 {
		err = errors.New("{arg0} required {DKGProposalDealConfirmationRequest}")
		return
	}

	request, ok := args[0].(requests.DKGProposalDealConfirmationRequest)

	if !ok {
		err = errors.New("cannot cast {arg0} to type {DKGProposalDealConfirmationRequest}")
		return
	}

	if err = request.Validate(); err != nil {
		return
	}

	dkgProposalParticipant, ok := m.payload.DKGProposalPayload[request.ParticipantId]

	if !ok {
		err = errors.New("{ParticipantId} not exist in quorum")
		return
	}

	if dkgProposalParticipant.Status != internal.DealAwaitConfirmation {
		err = errors.New(fmt.Sprintf("cannot confirm deal with {Status} = {\"%s\"}", dkgProposalParticipant.Status))
		return
	}

	copy(dkgProposalParticipant.Deal, request.Deal)
	dkgProposalParticipant.UpdatedAt = request.CreatedAt
	dkgProposalParticipant.Status = internal.DealConfirmed

	m.payload.DKGProposalPayload[request.ParticipantId] = dkgProposalParticipant

	return
}

func (m *DKGProposalFSM) actionValidateDkgProposalAwaitDeals(inEvent fsm.Event, args ...interface{}) (outEvent fsm.Event, response interface{}, err error) {
	var (
		isContainsError, isContainsExpired bool
	)

	m.payloadMu.Lock()
	defer m.payloadMu.Unlock()

	tm := time.Now()

	unconfirmedParticipants := len(m.payload.DKGProposalPayload)
	for _, participant := range m.payload.DKGProposalPayload {
		if participant.Status == internal.DealAwaitConfirmation {
			if participant.UpdatedAt.Add(config.SignatureProposalConfirmationDeadline).Before(tm) {
				isContainsExpired = true
			}
		} else {
			if participant.Status == internal.DealConfirmationError {
				isContainsError = true
			} else if participant.Status == internal.DealConfirmed {
				unconfirmedParticipants--
			}
		}
	}

	if isContainsError {
		outEvent = eventDKGDealsConfirmationCancelByErrorInternal
		return
	}

	if isContainsExpired {
		outEvent = eventDKGDealsConfirmationCancelByTimeoutInternal
		return
	}

	// The are no declined and timed out participants, check for all confirmations
	if unconfirmedParticipants > 0 {
		return
	}

	outEvent = eventDKGDealsConfirmedInternal

	for _, participant := range m.payload.DKGProposalPayload {
		participant.Status = internal.ResponseAwaitConfirmation
	}

	return
}

// Responses

func (m *DKGProposalFSM) actionResponseConfirmationReceived(inEvent fsm.Event, args ...interface{}) (outEvent fsm.Event, response interface{}, err error) {
	m.payloadMu.Lock()
	defer m.payloadMu.Unlock()

	if len(args) != 1 {
		err = errors.New("{arg0} required {DKGProposalResponseConfirmationRequest}")
		return
	}

	request, ok := args[0].(requests.DKGProposalResponseConfirmationRequest)

	if !ok {
		err = errors.New("cannot cast {arg0} to type {DKGProposalResponseConfirmationRequest}")
		return
	}

	if err = request.Validate(); err != nil {
		return
	}

	dkgProposalParticipant, ok := m.payload.DKGProposalPayload[request.ParticipantId]

	if !ok {
		err = errors.New("{ParticipantId} not exist in quorum")
		return
	}

	if dkgProposalParticipant.Status != internal.ResponseAwaitConfirmation {
		err = errors.New(fmt.Sprintf("cannot confirm response with {Status} = {\"%s\"}", dkgProposalParticipant.Status))
		return
	}

	copy(dkgProposalParticipant.Response, request.Response)
	dkgProposalParticipant.UpdatedAt = request.CreatedAt
	dkgProposalParticipant.Status = internal.ResponseConfirmed

	m.payload.DKGProposalPayload[request.ParticipantId] = dkgProposalParticipant

	return
}

func (m *DKGProposalFSM) actionValidateDkgProposalAwaitResponses(inEvent fsm.Event, args ...interface{}) (outEvent fsm.Event, response interface{}, err error) {
	var (
		isContainsError, isContainsExpired bool
	)

	m.payloadMu.Lock()
	defer m.payloadMu.Unlock()

	tm := time.Now()

	unconfirmedParticipants := len(m.payload.DKGProposalPayload)
	for _, participant := range m.payload.DKGProposalPayload {
		if participant.Status == internal.ResponseAwaitConfirmation {
			if participant.UpdatedAt.Add(config.SignatureProposalConfirmationDeadline).Before(tm) {
				isContainsExpired = true
			}
		} else {
			if participant.Status == internal.ResponseConfirmationError {
				isContainsError = true
			} else if participant.Status == internal.ResponseConfirmed {
				unconfirmedParticipants--
			}
		}
	}

	if isContainsError {
		outEvent = eventDKGResponseConfirmationCancelByErrorInternal
		return
	}

	if isContainsExpired {
		outEvent = eventDKGResponseConfirmationCancelByTimeoutInternal
		return
	}

	// The are no declined and timed out participants, check for all confirmations
	if unconfirmedParticipants > 0 {
		return
	}

	outEvent = eventDKGResponsesConfirmedInternal

	return
}

// Errors
func (m *DKGProposalFSM) actionConfirmationError(inEvent fsm.Event, args ...interface{}) (outEvent fsm.Event, response interface{}, err error) {
	m.payloadMu.Lock()
	defer m.payloadMu.Unlock()

	if len(args) != 1 {
		err = errors.New("{arg0} required {DKGProposalConfirmationErrorRequest}")
		return
	}

	request, ok := args[0].(requests.DKGProposalConfirmationErrorRequest)

	if !ok {
		err = errors.New("cannot cast {arg0} to type {DKGProposalConfirmationErrorRequest}")
		return
	}

	if err = request.Validate(); err != nil {
		return
	}

	dkgProposalParticipant, ok := m.payload.DKGProposalPayload[request.ParticipantId]

	if !ok {
		err = errors.New("{ParticipantId} not exist in quorum")
		return
	}

	// TODO: Move to methods
	switch inEvent {
	case EventDKGPubKeyConfirmationError:
		switch dkgProposalParticipant.Status {
		case internal.PubKeyAwaitConfirmation:
			dkgProposalParticipant.Status = internal.PubKeyConfirmationError
		case internal.PubKeyConfirmed:
			err = errors.New("{Status} already confirmed")
		case internal.PubKeyConfirmationError:
			err = errors.New(fmt.Sprintf("{Status} already has {\"%s\"}", internal.PubKeyConfirmationError))
		default:
			err = errors.New(fmt.Sprintf(
				"{Status} now is \"%s\" and cannot set to {\"%s\"}",
				dkgProposalParticipant.Status,
				internal.PubKeyConfirmationError,
			))
		}
	case EventDKGCommitConfirmationError:
		switch dkgProposalParticipant.Status {
		case internal.CommitAwaitConfirmation:
			dkgProposalParticipant.Status = internal.CommitConfirmationError
		case internal.CommitConfirmed:
			err = errors.New("{Status} already confirmed")
		case internal.CommitConfirmationError:
			err = errors.New(fmt.Sprintf("{Status} already has {\"%s\"}", internal.CommitConfirmationError))
		default:
			err = errors.New(fmt.Sprintf(
				"{Status} now is \"%s\" and cannot set to {\"%s\"}",
				dkgProposalParticipant.Status,
				internal.PubKeyConfirmationError,
			))
		}
	case EventDKGDealConfirmationError:
		switch dkgProposalParticipant.Status {
		case internal.DealAwaitConfirmation:
			dkgProposalParticipant.Status = internal.PubKeyConfirmationError
		case internal.DealConfirmed:
			err = errors.New("{Status} already confirmed")
		case internal.DealConfirmationError:
			err = errors.New(fmt.Sprintf("{Status} already has {\"%s\"}", internal.DealConfirmationError))
		default:
			err = errors.New(fmt.Sprintf(
				"{Status} now is \"%s\" and cannot set to {\"%s\"}",
				dkgProposalParticipant.Status,
				internal.PubKeyConfirmationError,
			))
		}
	case EventDKGResponseConfirmationError:
		switch dkgProposalParticipant.Status {
		case internal.ResponseAwaitConfirmation:
			dkgProposalParticipant.Status = internal.PubKeyConfirmationError
		case internal.ResponseConfirmed:
			err = errors.New("{Status} already confirmed")
		case internal.ResponseConfirmationError:
			err = errors.New(fmt.Sprintf("{Status} already has {\"%s\"}", internal.ResponseConfirmationError))
		default:
			err = errors.New(fmt.Sprintf(
				"{Status} now is \"%s\" and cannot set to {\"%s\"}",
				dkgProposalParticipant.Status,
				internal.PubKeyConfirmationError,
			))
		}
	default:
		err = errors.New(fmt.Sprintf("{%s} event cannot be used for action {actionConfirmationError}", inEvent))
	}

	if err != nil {
		return
	}

	dkgProposalParticipant.UpdatedAt = request.CreatedAt

	m.payload.DKGProposalPayload[request.ParticipantId] = dkgProposalParticipant

	// TODO: Add outEvent

	return
}

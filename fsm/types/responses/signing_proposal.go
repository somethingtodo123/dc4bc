package responses

type SigningProposalParticipantInvitationsResponse struct {
	InitiatorId  int
	Participants []*SigningProposalParticipantInvitationEntry
	SigningId    string
	// Source message for signing
	SrcPayload []byte
}

type SigningProposalParticipantInvitationEntry struct {
	ParticipantId int
	Addr          string
	Status        uint8
}

type SigningProposalParticipantStatusResponse []*SignatureProposalParticipantStatusEntry

type SigningProposalParticipantStatusEntry struct {
	ParticipantId int
	Addr          string
	Status        uint8
}

package responses

type DKGProposalCommitParticipantResponse []*DKGProposalCommitParticipantEntry

type DKGProposalCommitParticipantEntry struct {
	ParticipantId int
	Title         string
	Commit        []byte
}

type DKGProposalDealParticipantResponse []*DKGProposalDealParticipantEntry

type DKGProposalDealParticipantEntry struct {
	ParticipantId int
	Title         string
	Deal          []byte
}
package dbtypes

type ExplorerState struct {
	Key   string `db:"key"`
	Value string `db:"value"`
}

type ValidatorName struct {
	Index uint64 `db:"index"`
	Name  string `db:"name"`
}

type Block struct {
	Root                  []byte  `db:"root"`
	Slot                  uint64  `db:"slot"`
	ParentRoot            []byte  `db:"parent_root"`
	StateRoot             []byte  `db:"state_root"`
	Orphaned              uint8   `db:"orphaned"`
	Proposer              uint64  `db:"proposer"`
	Graffiti              []byte  `db:"graffiti"`
	GraffitiText          string  `db:"graffiti_text"`
	AttestationCount      uint64  `db:"attestation_count"`
	DepositCount          uint64  `db:"deposit_count"`
	ExitCount             uint64  `db:"exit_count"`
	WithdrawCount         uint64  `db:"withdraw_count"`
	WithdrawAmount        uint64  `db:"withdraw_amount"`
	AttesterSlashingCount uint64  `db:"attester_slashing_count"`
	ProposerSlashingCount uint64  `db:"proposer_slashing_count"`
	BLSChangeCount        uint64  `db:"bls_change_count"`
	EthTransactionCount   uint64  `db:"eth_transaction_count"`
	EthBlockNumber        *uint64 `db:"eth_block_number"`
	EthBlockHash          []byte  `db:"eth_block_hash"`
	SyncParticipation     float32 `db:"sync_participation"`
}

type BlockOrphanedRef struct {
	Root     []byte `db:"root"`
	Orphaned bool   `db:"orphaned"`
}

type Epoch struct {
	Epoch                 uint64  `db:"epoch"`
	ValidatorCount        uint64  `db:"validator_count"`
	ValidatorBalance      uint64  `db:"validator_balance"`
	Eligible              uint64  `db:"eligible"`
	VotedTarget           uint64  `db:"voted_target"`
	VotedHead             uint64  `db:"voted_head"`
	VotedTotal            uint64  `db:"voted_total"`
	BlockCount            uint16  `db:"block_count"`
	OrphanedCount         uint16  `db:"orphaned_count"`
	AttestationCount      uint64  `db:"attestation_count"`
	DepositCount          uint64  `db:"deposit_count"`
	ExitCount             uint64  `db:"exit_count"`
	WithdrawCount         uint64  `db:"withdraw_count"`
	WithdrawAmount        uint64  `db:"withdraw_amount"`
	AttesterSlashingCount uint64  `db:"attester_slashing_count"`
	ProposerSlashingCount uint64  `db:"proposer_slashing_count"`
	BLSChangeCount        uint64  `db:"bls_change_count"`
	EthTransactionCount   uint64  `db:"eth_transaction_count"`
	SyncParticipation     float32 `db:"sync_participation"`
}

type OrphanedBlock struct {
	Root      []byte `db:"root"`
	HeaderVer uint64 `db:"header_ver"`
	HeaderSSZ []byte `db:"header_ssz"`
	BlockVer  uint64 `db:"block_ver"`
	BlockSSZ  []byte `db:"block_ssz"`
}

type SlotAssignment struct {
	Slot     uint64 `db:"slot"`
	Proposer uint64 `db:"proposer"`
}

type SyncAssignment struct {
	Period    uint64 `db:"period"`
	Index     uint32 `db:"index"`
	Validator uint64 `db:"validator"`
}

type UnfinalizedBlock struct {
	Root      []byte `db:"root"`
	Slot      uint64 `db:"slot"`
	HeaderVer uint64 `db:"header_ver"`
	HeaderSSZ []byte `db:"header_ssz"`
	BlockVer  uint64 `db:"block_ver"`
	BlockSSZ  []byte `db:"block_ssz"`
}

type Blob struct {
	Commitment []byte  `db:"commitment"`
	Proof      []byte  `db:"proof"`
	Size       uint32  `db:"size"`
	Blob       *[]byte `db:"blob"`
}

type BlobAssignment struct {
	Root       []byte `db:"root"`
	Commitment []byte `db:"commitment"`
	Slot       uint64 `db:"slot"`
}

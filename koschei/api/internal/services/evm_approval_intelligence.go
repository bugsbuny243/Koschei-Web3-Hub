package services

import (
	"errors"
	"math/big"
	"strings"
	"time"

	"koschei/api/internal/networktarget"
)

const (
	EVMApprovalMechanismApprove   = "erc20_approve"
	EVMApprovalMechanismPermit    = "erc2612_permit"
	EVMApprovalMechanismTemporary = "erc7674_temporary"

	EVMApprovalPersistencePersistent       = "persistent"
	EVMApprovalPersistenceTransactionScope = "transaction_scoped"
)

var maxUint256 = func() *big.Int {
	value := new(big.Int).Lsh(big.NewInt(1), 256)
	return value.Sub(value, big.NewInt(1))
}()

// EVMApprovalObservation is a bounded, read-only description of an allowance
// or approval authorization. It deliberately separates the mechanism used to
// create spending authority from how long that authority persists.
//
// In particular, an ERC-2612 permit deadline is the last time the signed permit
// may be submitted. It is not the expiration time of the allowance produced by
// a successful permit call. ERC-7674 approvals are transaction scoped.
type EVMApprovalObservation struct {
	Network string `json:"network"`
	Token   string `json:"token"`
	Owner   string `json:"owner"`
	Spender string `json:"spender"`

	Mechanism string `json:"mechanism"`
	Amount    string `json:"amount"`

	PermitNonce        string `json:"permit_nonce,omitempty"`
	PermitDeadlineUnix int64  `json:"permit_deadline_unix,omitempty"`
	DomainSeparator    string `json:"domain_separator,omitempty"`
	TransactionHash    string `json:"transaction_hash,omitempty"`

	SpenderCodeHash         string `json:"spender_code_sha256,omitempty"`
	SpenderDelegationTarget string `json:"spender_delegation_target,omitempty"`
	ProxyImplementation     string `json:"proxy_implementation,omitempty"`
	ProxyAdmin              string `json:"proxy_admin,omitempty"`
	ProxyBeacon             string `json:"proxy_beacon,omitempty"`

	ObservedAt time.Time `json:"observed_at"`
}

type EVMApprovalIntelligence struct {
	Network     string          `json:"network"`
	Token       string          `json:"token"`
	Owner       string          `json:"owner"`
	Spender     string          `json:"spender"`
	Mechanism   string          `json:"mechanism"`
	Persistence string          `json:"persistence"`
	Amount      string          `json:"amount"`
	Unlimited   bool            `json:"unlimited"`
	Trust       Web3TrustVector `json:"trust"`
	Reasons     []string        `json:"reasons"`

	PermitNonce        string `json:"permit_nonce,omitempty"`
	PermitDeadlineUnix int64  `json:"permit_deadline_unix,omitempty"`
	DomainSeparator    string `json:"domain_separator,omitempty"`
	TransactionHash    string `json:"transaction_hash,omitempty"`

	SpenderCodeHash         string `json:"spender_code_sha256,omitempty"`
	SpenderDelegationTarget string `json:"spender_delegation_target,omitempty"`
	ProxyImplementation     string `json:"proxy_implementation,omitempty"`
	ProxyAdmin              string `json:"proxy_admin,omitempty"`
	ProxyBeacon             string `json:"proxy_beacon,omitempty"`

	ObservedAt time.Time `json:"observed_at"`
}

func BuildEVMApprovalIntelligence(observation EVMApprovalObservation) (EVMApprovalIntelligence, error) {
	if observation.ObservedAt.IsZero() {
		return EVMApprovalIntelligence{}, errors.New("approval observation time is required")
	}
	if _, ok := networktarget.ExpectedEVMChainID(strings.TrimSpace(observation.Network)); !ok {
		return EVMApprovalIntelligence{}, errors.New("unsupported EVM approval network")
	}

	token, err := normalizeEVMApprovalAddress(observation.Token)
	if err != nil {
		return EVMApprovalIntelligence{}, errors.New("invalid approval token address")
	}
	owner, err := normalizeEVMApprovalAddress(observation.Owner)
	if err != nil {
		return EVMApprovalIntelligence{}, errors.New("invalid approval owner address")
	}
	spender, err := normalizeEVMApprovalAddress(observation.Spender)
	if err != nil {
		return EVMApprovalIntelligence{}, errors.New("invalid approval spender address")
	}

	amount := new(big.Int)
	if strings.TrimSpace(observation.Amount) == "" {
		return EVMApprovalIntelligence{}, errors.New("approval amount is required")
	}
	if _, ok := amount.SetString(strings.TrimSpace(observation.Amount), 10); !ok || amount.Sign() < 0 || amount.Cmp(maxUint256) > 0 {
		return EVMApprovalIntelligence{}, errors.New("invalid approval amount")
	}

	mechanism := strings.TrimSpace(observation.Mechanism)
	persistence := EVMApprovalPersistencePersistent
	reasons := []string{"READ_ONLY_EVM_APPROVAL_OBSERVATION"}

	permitNonce := strings.TrimSpace(observation.PermitNonce)
	domainSeparator := strings.ToLower(strings.TrimSpace(observation.DomainSeparator))
	transactionHash := strings.ToLower(strings.TrimSpace(observation.TransactionHash))

	switch mechanism {
	case EVMApprovalMechanismApprove:
		if permitNonce != "" || observation.PermitDeadlineUnix != 0 || domainSeparator != "" || transactionHash != "" {
			return EVMApprovalIntelligence{}, errors.New("ERC-20 approve observation cannot carry permit or temporary fields")
		}
		reasons = append(reasons, "PERSISTENT_ALLOWANCE_OBSERVED")
	case EVMApprovalMechanismPermit:
		if !validUnsignedDecimal(permitNonce) || observation.PermitDeadlineUnix <= 0 || !validHexWord(domainSeparator) || transactionHash != "" {
			return EVMApprovalIntelligence{}, errors.New("incomplete ERC-2612 permit observation")
		}
		reasons = append(reasons, "ERC2612_PERMIT_AUTHORIZATION_OBSERVED", "PERSISTENT_ALLOWANCE_SEMANTICS")
		if observation.ObservedAt.Unix() > observation.PermitDeadlineUnix {
			// This closes the permit submission window only. It does not imply that
			// an allowance already created by permit has expired.
			reasons = append(reasons, "PERMIT_SUBMISSION_WINDOW_CLOSED")
		}
	case EVMApprovalMechanismTemporary:
		if permitNonce != "" || observation.PermitDeadlineUnix != 0 || domainSeparator != "" || !validTransactionHash(transactionHash) {
			return EVMApprovalIntelligence{}, errors.New("incomplete ERC-7674 temporary approval observation")
		}
		persistence = EVMApprovalPersistenceTransactionScope
		reasons = append(reasons, "ERC7674_TRANSACTION_SCOPED_APPROVAL_OBSERVED")
	default:
		return EVMApprovalIntelligence{}, errors.New("unsupported approval mechanism")
	}

	unlimited := amount.Cmp(maxUint256) == 0
	if unlimited {
		reasons = append(reasons, "UNLIMITED_ALLOWANCE_OBSERVED")
	}

	spenderCodeHash := strings.ToLower(strings.TrimSpace(observation.SpenderCodeHash))
	if spenderCodeHash != "" {
		if len(spenderCodeHash) != 64 || !isLowerHex(spenderCodeHash) {
			return EVMApprovalIntelligence{}, errors.New("invalid spender code hash")
		}
		reasons = append(reasons, "SPENDER_CODE_OBSERVED")
	}

	delegationTarget, err := normalizeOptionalEVMApprovalAddress(observation.SpenderDelegationTarget)
	if err != nil {
		return EVMApprovalIntelligence{}, errors.New("invalid spender delegation target")
	}
	implementation, err := normalizeOptionalEVMApprovalAddress(observation.ProxyImplementation)
	if err != nil {
		return EVMApprovalIntelligence{}, errors.New("invalid proxy implementation")
	}
	admin, err := normalizeOptionalEVMApprovalAddress(observation.ProxyAdmin)
	if err != nil {
		return EVMApprovalIntelligence{}, errors.New("invalid proxy admin")
	}
	beacon, err := normalizeOptionalEVMApprovalAddress(observation.ProxyBeacon)
	if err != nil {
		return EVMApprovalIntelligence{}, errors.New("invalid proxy beacon")
	}
	if delegationTarget != "" {
		reasons = append(reasons, "EIP7702_SPENDER_DELEGATION_OBSERVED")
	}
	if implementation != "" || admin != "" || beacon != "" {
		reasons = append(reasons, "SPENDER_UPGRADE_AUTHORITY_OBSERVED")
	}

	trust := Web3TrustVector{Observed: true, Reasons: append([]string(nil), reasons...)}
	return EVMApprovalIntelligence{
		Network:                 strings.TrimSpace(observation.Network),
		Token:                   token,
		Owner:                   owner,
		Spender:                 spender,
		Mechanism:               mechanism,
		Persistence:             persistence,
		Amount:                  amount.String(),
		Unlimited:               unlimited,
		Trust:                   trust,
		Reasons:                 reasons,
		PermitNonce:             permitNonce,
		PermitDeadlineUnix:      observation.PermitDeadlineUnix,
		DomainSeparator:         domainSeparator,
		TransactionHash:         transactionHash,
		SpenderCodeHash:         spenderCodeHash,
		SpenderDelegationTarget: delegationTarget,
		ProxyImplementation:     implementation,
		ProxyAdmin:              admin,
		ProxyBeacon:             beacon,
		ObservedAt:              observation.ObservedAt.UTC(),
	}, nil
}

func normalizeEVMApprovalAddress(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if len(value) != 42 || !strings.HasPrefix(value, "0x") || !isLowerHex(value[2:]) {
		return "", errors.New("invalid EVM address")
	}
	return value, nil
}

func normalizeOptionalEVMApprovalAddress(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", nil
	}
	return normalizeEVMApprovalAddress(value)
}

func validUnsignedDecimal(value string) bool {
	parsed := new(big.Int)
	if value == "" {
		return false
	}
	_, ok := parsed.SetString(value, 10)
	return ok && parsed.Sign() >= 0 && parsed.Cmp(maxUint256) <= 0
}

func validHexWord(value string) bool {
	return len(value) == 66 && strings.HasPrefix(value, "0x") && isLowerHex(value[2:])
}

func validTransactionHash(value string) bool {
	return validHexWord(value)
}

func isLowerHex(value string) bool {
	if value == "" {
		return false
	}
	for _, ch := range value {
		if (ch < '0' || ch > '9') && (ch < 'a' || ch > 'f') {
			return false
		}
	}
	return true
}

package v1

import (
	"github.com/xssnick/tonutils-go/ton/wallet"
)

type BagInfo struct {
	BagID       string `json:"bag_id"`
	Description string `json:"description"`
	Size        uint64 `json:"size"`
	FilesCount  uint64 `json:"files_count"`
	BagSize     uint64 `json:"bag_size"`
	Peers       int    `json:"peers"`
}

type LoginInfo struct {
	Address   string                 `json:"address"`
	Proof     wallet.TonConnectProof `json:"proof"`
	StateInit []byte                 `json:"state_init"`
}

type ProviderShort struct {
	Address       string `json:"address"`
	PricePerMBDay uint64 `json:"price_per_mb_day"`
	MaxSpan       uint64 `json:"max_span"`
}

type OffersRequest struct {
	BagID     string   `json:"bag_id"`
	Providers []string `json:"providers"`
}

type ProviderContractData struct {
	Key          string `json:"key"`
	MinBounty    string `json:"min_bounty"`
	MinSpan      uint64 `json:"min_span"`
	MaxSpan      uint64 `json:"max_span"`
	RatePerMBDay uint64 `json:"price_per_mb_day"`
}

type InitStorageContractRequest struct {
	BagID        string            `json:"bag_id"`
	OwnerAddress string            `json:"owner_address"`
	Amount       uint64            `json:"amount"`
	Providers    []ProviderAddress `json:"providers"`
}

type ProviderOffer struct {
	OfferSpan     uint64 `json:"offer_span"`
	PricePerDay   uint64 `json:"price_per_day"`
	PricePerProof uint64 `json:"price_per_proof"`
	PricePerMB    uint64 `json:"price_per_mb"`

	Provider ProviderContractData `json:"provider"`
}

type ProviderRatesResponse struct {
	Offers   []ProviderOffer   `json:"offers"`
	Declines []ProviderDecline `json:"declines,omitempty"`
}

type ProviderAddress struct {
	PublicKey string `json:"public_key"`
	Address   string `json:"address"`
}

type ProviderDecline struct {
	ProviderKey string `json:"provider_key"`
	Reason      string `json:"reason"`
}

type Transaction struct {
	Body      string `json:"body"`
	StateInit string `json:"state_init"`
	Address   string `json:"address"`
	Amount    uint64 `json:"amount"`
}

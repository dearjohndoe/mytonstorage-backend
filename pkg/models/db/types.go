package db

type BagInfo struct {
	BagID       string `json:"bagid"`
	Description string `json:"description"`
	Size        uint64 `json:"size"`
	CreatedAt   int64  `json:"created_at"`
}

type BagDescription struct {
	ContractAddress string `json:"contract_address"`
	BagID           string `json:"bagid"`
	Description     string `json:"description"`
	Size            uint64 `json:"size"`
}

type UserBagInfo struct {
	BagID       string `json:"bagid"`
	UserAddress string `json:"user_address"`
	CreatedAt   int64  `json:"created_at"`
}

type BagStorageContract struct {
	BagID           string `json:"bagid"`
	StorageContract string `json:"storage_contract"`
	Size            uint64 `json:"size"`
}

type ProviderNotification struct {
	BagID           string `json:"bagid"`
	StorageContract string `json:"storage_contract"`
	ProviderPubkey  string `json:"provider_pubkey"`
	Size            uint64 `json:"size"`
}

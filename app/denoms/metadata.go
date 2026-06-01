package denoms

import banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

const (
	DisplayDenom  = "tac"
	BaseDenom     = "utac"
	BaseDenomUnit = 18

	// EVMCoinName is the full name of the native EVM coin.
	EVMCoinName = "TAC"
	// EVMCoinSymbol is the ticker symbol of the native EVM coin.
	EVMCoinSymbol = "TAC"
	// EVMCoinDescription is the description stored in bank denom metadata.
	EVMCoinDescription = "Native 18-decimal denom metadata for TacChain EVM"
)

// DefaultBankDenomMetadata returns the bank denom metadata for the native EVM coin.
// This metadata is required by the EVM module's InitEvmCoinInfo to locate the coin denom.
func DefaultBankDenomMetadata() []banktypes.Metadata {
	return []banktypes.Metadata{
		DefaultNativeDenomMetadata(),
	}
}

// DefaultNativeDenomMetadata returns the bank denom metadata for the native EVM coin.
// This metadata is required by the EVM module's InitEvmCoinInfo to locate the coin denom.
func DefaultNativeDenomMetadata() banktypes.Metadata {
	return banktypes.Metadata{
		Description: EVMCoinDescription,
		Base:        BaseDenom,
		DenomUnits: []*banktypes.DenomUnit{
			{Denom: BaseDenom, Exponent: 0},
			{Denom: DisplayDenom, Exponent: uint32(BaseDenomUnit)},
		},
		Name:    EVMCoinName,
		Symbol:  EVMCoinSymbol,
		Display: DisplayDenom,
	}
}

func DefaultBankDenomMetadataFor(baseDenom string) (banktypes.Metadata, bool) {
	for _, metadata := range DefaultBankDenomMetadata() {
		if metadata.Base == baseDenom {
			return metadata, true
		}
	}

	return banktypes.Metadata{}, false
}

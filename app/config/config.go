package config

import (
	"math/big"
	"os"
	"sync"
	"time"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	evmdconfig "github.com/cosmos/evm/config"
)

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

	// Bech32PrefixAccAddr defines the Bech32 prefix of an account's address.
	Bech32PrefixAccAddr = "tac"

	NodeDir        = ".tacchaind"
	AppName        = "TacChainApp"
	DefaultChainID = "tacchain_2391337-1"

	// TimeoutCommit defines the default consensus commit timeout.
	TimeoutCommit = 1 * time.Second
)

var (
	// Bech32PrefixAccPub defines the Bech32 prefix of an account's public key.
	Bech32PrefixAccPub = Bech32PrefixAccAddr + "pub"
	// Bech32PrefixValAddr defines the Bech32 prefix of a validator's operator address.
	Bech32PrefixValAddr = Bech32PrefixAccAddr + "valoper"
	// Bech32PrefixValPub defines the Bech32 prefix of a validator's operator public key.
	Bech32PrefixValPub = Bech32PrefixAccAddr + "valoperpub"
	// Bech32PrefixConsAddr defines the Bech32 prefix of a consensus node address.
	Bech32PrefixConsAddr = Bech32PrefixAccAddr + "valcons"
	// Bech32PrefixConsPub defines the Bech32 prefix of a consensus node public key.
	Bech32PrefixConsPub = Bech32PrefixAccAddr + "valconspub"

	DefaultNodeHome = os.ExpandEnv("$HOME/") + NodeDir

	// PowerReduction defines the default power reduction value for staking.
	PowerReduction = sdkmath.NewIntFromBigInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(BaseDenomUnit), nil))

	setupSDKConfigOnce sync.Once
)

// SetupSDKConfig configures the Cosmos SDK globals required by TacChain.
func SetupSDKConfig() {
	setupSDKConfigOnce.Do(func() {
		registerDenoms()
		setAddressPrefixes()
	})
}

// registerDenoms registers token denoms.
func registerDenoms() {
	sdk.DefaultBondDenom = BaseDenom
	sdk.DefaultPowerReduction = PowerReduction

	config := sdk.GetConfig()
	evmdconfig.SetBip44CoinType(config)

	if err := sdk.RegisterDenom(DisplayDenom, sdkmath.LegacyOneDec()); err != nil {
		panic(err)
	}

	if err := sdk.RegisterDenom(BaseDenom, sdkmath.LegacyNewDecWithPrec(1, BaseDenomUnit)); err != nil {
		panic(err)
	}
}

// setAddressPrefixes configures Bech32 prefixes for accounts, validators, and consensus nodes.
func setAddressPrefixes() {
	config := sdk.GetConfig()
	config.SetBech32PrefixForAccount(Bech32PrefixAccAddr, Bech32PrefixAccPub)
	config.SetBech32PrefixForValidator(Bech32PrefixValAddr, Bech32PrefixValPub)
	config.SetBech32PrefixForConsensusNode(Bech32PrefixConsAddr, Bech32PrefixConsPub)
}

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

// DefaultBankDenomMetadataFor returns default metadata for the provided base denom.
func DefaultBankDenomMetadataFor(baseDenom string) (banktypes.Metadata, bool) {
	for _, metadata := range DefaultBankDenomMetadata() {
		if metadata.Base == baseDenom {
			return metadata, true
		}
	}

	return banktypes.Metadata{}, false
}

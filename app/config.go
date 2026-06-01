package app

import (
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/TacBuild/tacchain/app/denoms"
	"github.com/cosmos/cosmos-sdk/client/flags"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	evmdconfig "github.com/cosmos/evm/config"
	"github.com/spf13/cast"
)

const (
	DisplayDenom  = denoms.DisplayDenom
	BaseDenom     = denoms.BaseDenom
	BaseDenomUnit = denoms.BaseDenomUnit

	// EVMCoinName is the full name of the native EVM coin.
	EVMCoinName = denoms.EVMCoinName
	// EVMCoinSymbol is the ticker symbol of the native EVM coin.
	EVMCoinSymbol = denoms.EVMCoinSymbol
	// EVMCoinDescription is the description stored in bank denom metadata.
	EVMCoinDescription = denoms.EVMCoinDescription

	// Bech32PrefixAccAddr defines the Bech32 prefix of an account's address.
	Bech32PrefixAccAddr = "tac"

	NodeDir        = ".tacchaind"
	AppName        = "TacChainApp"
	DefaultChainID = "tacchain_2391337-1"

	// Custom timeout commit to ensure faster block times
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

	// PowerReduction defines the default power reduction value for staking
	PowerReduction = sdkmath.NewIntFromBigInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(BaseDenomUnit), nil))
)

func init() {
	registerDenoms()
	setAddressPrefixes()
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

// setAddressPrefixes builds the Config with Bech32 addressPrefix and publKeyPrefix for accounts, validators, and consensus nodes and verifies that addreeses have correct format.
func setAddressPrefixes() {
	config := sdk.GetConfig()
	config.SetBech32PrefixForAccount(Bech32PrefixAccAddr, Bech32PrefixAccPub)
	config.SetBech32PrefixForValidator(Bech32PrefixValAddr, Bech32PrefixValPub)
	config.SetBech32PrefixForConsensusNode(Bech32PrefixConsAddr, Bech32PrefixConsPub)
}

// DefaultBankDenomMetadata returns the bank denom metadata for the native EVM coin.
// This metadata is required by the EVM module's InitEvmCoinInfo to locate the coin denom.
func DefaultBankDenomMetadata() []banktypes.Metadata {
	return denoms.DefaultBankDenomMetadata()
}

// ParseEVMChainID extracts the EVM chain ID (uint64) from a cosmos chain ID string.
// Format: "tacchain_2391337-1" → 2391337.
func ParseEVMChainID(cosmosChainID string) (uint64, error) {
	re := regexp.MustCompile(`_(\d+)-`)
	m := re.FindStringSubmatch(cosmosChainID)
	if len(m) != 2 {
		return 0, fmt.Errorf("invalid cosmos chain ID format: %q", cosmosChainID)
	}
	n, ok := new(big.Int).SetString(m[1], 10)
	if !ok || !n.IsUint64() {
		return 0, fmt.Errorf("invalid EVM chain ID in %q", cosmosChainID)
	}
	return n.Uint64(), nil
}

// resolveChainID returns the cosmos chain ID, falling back to genesis file if flag is empty.
// flags.FlagChainID is only populated during `init`, not during `start`.
func resolveChainID(appOpts servertypes.AppOptions) string {
	if chainID := cast.ToString(appOpts.Get(flags.FlagChainID)); chainID != "" {
		return chainID
	}
	homeDir := cast.ToString(appOpts.Get(flags.FlagHome))
	genesisPath := filepath.Join(homeDir, "config", "genesis.json")
	f, err := os.Open(genesisPath)
	if err != nil {
		// genesis not available (e.g. tempApp during encoding config init).
		// Return empty string so caller falls back to DefaultEVMChainID (262144),
		// which allows SetChainConfig to be called again by the real app instance.
		return ""
	}
	defer f.Close()
	chainID, err := genutiltypes.ParseChainIDFromGenesis(f)
	if err != nil {
		panic(fmt.Errorf("cannot parse chain ID from genesis %q: %w", genesisPath, err))
	}
	return chainID
}

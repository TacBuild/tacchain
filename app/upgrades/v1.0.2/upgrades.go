package v102

// Upgrade for implementing liquid stake module

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"

	"github.com/TacBuild/tacchain/app/upgrades"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	evmerc20types "github.com/cosmos/evm/x/erc20/types"

	"github.com/ethereum/go-ethereum/common"
	"golang.org/x/crypto/sha3"
)

var WhitelistAdminAddressNotFound = errors.New("failed to find whitelist admin address")

// UpgradeName defines the on-chain upgrade name
const UpgradeName = "v1.0.2"

var Upgrade = upgrades.Upgrade{
	UpgradeName:          UpgradeName,
	CreateUpgradeHandler: CreateUpgradeHandler,
	StoreUpgrades: storetypes.StoreUpgrades{
		Added:   []string{"utacliquidstake", "epochs"},
		Deleted: []string{},
	},
}

func CreateUpgradeHandler(
	mm upgrades.ModuleManager,
	configurator module.Configurator,
	ak *upgrades.AppKeepers,
) upgradetypes.UpgradeHandler {
	return func(ctx context.Context, plan upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
		newVM, err := mm.RunMigrations(ctx, configurator, fromVM)
		if err != nil {
			return newVM, err
		}

		sdkCtx := sdk.UnwrapSDKContext(ctx)

		// Register gTAC token pair
		lsmBondDenom := ak.LiquidStakeKeeper.LiquidBondDenom(sdkCtx)
		lsmBondCommonAddress, err := generateAddressFromDenom(lsmBondDenom)
		if err != nil {
			return newVM, err
		}
		ak.BankKeeper.SetDenomMetaData(ctx, GTACMetadata)

		erc20Params := ak.Erc20Keeper.GetParams(sdkCtx)
		if err := ak.Erc20Keeper.SetParams(sdkCtx, erc20Params); err != nil {
			return newVM, err
		}
		if err := ak.Erc20Keeper.EnableNativePrecompile(sdkCtx, lsmBondCommonAddress); err != nil {
			return newVM, err
		}

		lsmTokenPair := evmerc20types.NewTokenPair(lsmBondCommonAddress, lsmBondDenom, evmerc20types.OWNER_MODULE)

		ak.Erc20Keeper.SetToken(sdkCtx, lsmTokenPair)

		logger := sdkCtx.Logger()
		params := ak.LiquidStakeKeeper.GetParams(sdkCtx)
		adminAddress, err := getAdminAddressFromPlanInfo(plan.Info)
		switch err {
		case WhitelistAdminAddressNotFound, nil:
		default:
			logger.Error("invalid whitelist admin address in plan info", "error", err)
		}
		params.WhitelistAdminAddress = adminAddress
		if err := ak.LiquidStakeKeeper.SetParams(sdkCtx, params); err != nil {
			return newVM, fmt.Errorf("failed to set params for liquidstake module: %w", err)
		}

		stakingParams, err := ak.StakingKeeper.GetParams(ctx)
		if err != nil {
			return newVM, err
		}

		stakingParams.ValidatorBondFactor = stakingtypes.DefaultValidatorBondFactor
		stakingParams.GlobalLiquidStakingCap = stakingtypes.DefaultGlobalLiquidStakingCap
		stakingParams.ValidatorLiquidStakingCap = stakingtypes.DefaultValidatorLiquidStakingCap

		if err := ak.StakingKeeper.SetParams(ctx, stakingParams); err != nil {
			return newVM, fmt.Errorf("failed to set params for staking module: %w", err)
		}

		if err := initializeNewValidatorFields(ctx, ak); err != nil {
			return newVM, fmt.Errorf("failed to initialize new validtor fields: %w", err)
		}

		return newVM, nil
	}
}

func generateAddressFromDenom(denom string) (common.Address, error) {
	hash := sha3.NewLegacyKeccak256()
	if _, err := hash.Write([]byte(denom)); err != nil {
		return common.Address{}, err
	}
	return common.BytesToAddress(hash.Sum(nil)), nil
}

func initializeNewValidatorFields(ctx context.Context, ak *upgrades.AppKeepers) error {
	params, err := ak.StakingKeeper.GetParams(ctx)
	if err != nil {
		return fmt.Errorf("failed to get staking params: %w", err)
	}

	validators, err := ak.StakingKeeper.GetValidators(ctx, params.MaxValidators)
	if err != nil {
		return fmt.Errorf("failed to get validators: %w", err)
	}

	for _, validator := range validators {
		newValidator := validator
		newValidator.ValidatorBondShares = math.LegacyZeroDec()
		newValidator.LiquidShares = math.LegacyZeroDec()

		err = ak.StakingKeeper.RemoveValidator(ctx, sdk.ValAddress(validator.OperatorAddress))
		if err != nil {
			return fmt.Errorf("failed to remove validator: %w", err)
		}
		err = ak.StakingKeeper.SetValidator(ctx, newValidator)
		if err != nil {
			return fmt.Errorf("failed to set validator: %w", err)
		}
		err = ak.StakingKeeper.SetValidatorByConsAddr(ctx, newValidator)
		if err != nil {
			return fmt.Errorf("failed to set validator by consensus address: %w", err)
		}
		err = ak.StakingKeeper.SetValidatorByPowerIndex(ctx, newValidator)
		if err != nil {
			return fmt.Errorf("failed to set validator by power index: %w", err)
		}
	}

	return nil
}

func getAdminAddressFromPlanInfo(info string) (string, error) {
	key := "whitelist_admin_address"
	addressPrefix := sdk.GetConfig().GetBech32AccountAddrPrefix()
	re := regexp.MustCompile(key + `:\s*(` + addressPrefix + `[a-zA-Z0-9]+)`)
	matches := re.FindStringSubmatch(info)

	var addr string
	if len(matches) > 1 {
		addr = matches[1]
	} else {
		return "", WhitelistAdminAddressNotFound
	}
	if _, err := sdk.AccAddressFromBech32(addr); err != nil {
		return "", fmt.Errorf("failed to validate whitelist admin address %s: %w", addr, err)
	}
	return addr, nil
}

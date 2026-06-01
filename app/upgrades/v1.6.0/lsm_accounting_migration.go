package v160

import (
	"fmt"

	"cosmossdk.io/math"

	"github.com/TacBuild/tacchain/app/upgrades"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// rebuildStakingLSMAccounting fixes staking's LSM counters from delegation state
// instead of asserting a narrow pre-upgrade shape. This keeps the upgrade
// resilient to valid pre-upgrade transactions such as tokenizing shares or
// marking a delegation as validator-bond.
func rebuildStakingLSMAccounting(ctx sdk.Context, ak *upgrades.AppKeepers) error {
	validators, err := ak.StakingKeeper.GetAllValidators(ctx)
	if err != nil {
		return fmt.Errorf("get all validators: %w", err)
	}

	validatorsByOperator := make(map[string]stakingtypes.Validator, len(validators))
	for _, validator := range validators {
		validator.LiquidShares = math.LegacyZeroDec()
		validator.ValidatorBondShares = math.LegacyZeroDec()
		validatorsByOperator[validator.OperatorAddress] = validator
	}

	delegations, err := ak.StakingKeeper.GetAllDelegations(ctx)
	if err != nil {
		return fmt.Errorf("get all delegations: %w", err)
	}

	oldTotalLiquidTokens := ak.StakingKeeper.GetTotalLiquidStakedTokens(ctx)
	newTotalLiquidTokens := math.ZeroInt()
	liquidDelegations := 0
	validatorBondDelegations := 0

	for _, delegation := range delegations {
		validator, ok := validatorsByOperator[delegation.ValidatorAddress]
		if !ok {
			return fmt.Errorf("delegation %s -> %s references missing validator", delegation.DelegatorAddress, delegation.ValidatorAddress)
		}

		delegatorAddr, err := sdk.AccAddressFromBech32(delegation.DelegatorAddress)
		if err != nil {
			return fmt.Errorf("invalid delegation delegator address %q: %w", delegation.DelegatorAddress, err)
		}

		if delegation.ValidatorBond {
			validatorBondDelegations++
			validator.ValidatorBondShares = validator.ValidatorBondShares.Add(delegation.Shares)
		}

		if ak.StakingKeeper.DelegatorIsLiquidStaker(delegatorAddr) {
			liquidDelegations++
			validator.LiquidShares = validator.LiquidShares.Add(delegation.Shares)
			newTotalLiquidTokens = newTotalLiquidTokens.Add(validator.TokensFromShares(delegation.Shares).TruncateInt())
		}

		validatorsByOperator[delegation.ValidatorAddress] = validator
	}

	updatedValidators := 0
	for _, validator := range validatorsByOperator {
		updatedValidators++
		if err := ak.StakingKeeper.SetValidator(ctx, validator); err != nil {
			return fmt.Errorf("set validator %s: %w", validator.OperatorAddress, err)
		}
	}

	ak.StakingKeeper.SetTotalLiquidStakedTokens(ctx, newTotalLiquidTokens)

	tokenizeShareRecords := ak.StakingKeeper.GetAllTokenizeShareRecords(ctx)
	ctx.Logger().Info(
		"Rebuilt staking LSM accounting",
		"tokenize_share_records", len(tokenizeShareRecords),
		"liquid_delegations", liquidDelegations,
		"validator_bond_delegations", validatorBondDelegations,
		"updated_validators", updatedValidators,
		"old_total_liquid_tokens", oldTotalLiquidTokens.String(),
		"new_total_liquid_tokens", newTotalLiquidTokens.String(),
	)

	return nil
}

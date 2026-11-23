package app

import (
	"bytes"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	sdkmath "cosmossdk.io/math"
	upgradetypes "cosmossdk.io/x/upgrade/types"

	appconfig "github.com/TacBuild/tacchain/app/config"
	v166 "github.com/TacBuild/tacchain/app/upgrades/v1.6.6"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

func TestV166VestingUnlock(t *testing.T) {
	app, ctx, source, _ := newV166StakingTestApp(t)
	const denom = appconfig.BaseDenom
	fundV166TestAccount(t, app, ctx, source, 100, sdkmath.ZeroInt(), sdkmath.ZeroInt())

	amount := sdkmath.NewInt(100)
	unrelated := sdk.AccAddress(bytes.Repeat([]byte{9}, 20))
	fundV166TestAccount(t, app, ctx, unrelated, 110, amount, amount)
	unrelatedAccount := app.AccountKeeper.GetAccount(ctx, unrelated)

	validatorAddress := sdk.ValAddress(bytes.Repeat([]byte{1}, 20))
	newV166TestValidator(t, app, ctx, validatorAddress)
	sk := app.StakingKeeper
	contractCode := []byte{0x60, 0x00, 0x60, 0x00, 0xf3}
	codeHash := crypto.Keccak256Hash(contractCode)
	storageKey := common.HexToHash("0x01")
	storageValue := common.HexToHash("0x02")
	app.EVMKeeper.SetCode(ctx, codeHash.Bytes(), contractCode)

	addresses := []string{
		"tac14gurzf75agujcznmv7k5xnz5z9jgcctza5gynh",
		"tac1td8q7adv0x6ltxs8630tves5t9y49fzfs8cj9d",
		"tac18f83zsqe8up5vge8l8gpr3tnnp2sfvheqxn83f",
		v166TestDestination,
	}
	type accountState struct {
		address      sdk.AccAddress
		base         *authtypes.BaseAccount
		balance      sdk.Coins
		delegation   stakingtypes.Delegation
		startingInfo distrtypes.DelegatorStartingInfo
	}
	snapshots := make([]accountState, 0, len(addresses))
	var completionTime time.Time
	var pending stakingtypes.UnbondingDelegation
	for i, address := range addresses {
		addr, err := sdk.AccAddressFromBech32(address)
		require.NoError(t, err)
		fundV166TestAccount(t, app, ctx, addr, 101+uint64(i), amount, amount)
		delegateV166TestCoins(t, app, ctx, addr, validatorAddress, sdkmath.NewInt(40))
		if i == 0 {
			completionTime, _, err = sk.Undelegate(ctx, addr, validatorAddress, sdkmath.LegacyNewDec(10))
			require.NoError(t, err)
			pending, err = sk.GetUnbondingDelegation(ctx, addr, validatorAddress)
			require.NoError(t, err)
		}
		ethAddress := common.BytesToAddress(addr)
		app.EVMKeeper.SetCodeHash(ctx, ethAddress.Bytes(), codeHash.Bytes())
		app.EVMKeeper.SetState(ctx, ethAddress, storageKey, storageValue.Bytes())

		account := app.AccountKeeper.GetAccount(ctx, addr).(*vestingtypes.PeriodicVestingAccount)
		delegation, err := sk.GetDelegation(ctx, addr, validatorAddress)
		require.NoError(t, err)
		startingInfo, err := app.DistrKeeper.GetDelegatorStartingInfo(ctx, validatorAddress, addr)
		require.NoError(t, err)
		require.Empty(t, app.BankKeeper.SpendableCoins(ctx, addr))
		snapshots = append(snapshots, accountState{
			address:      addr,
			base:         account.BaseAccount,
			balance:      app.BankKeeper.GetAllBalances(ctx, addr),
			delegation:   delegation,
			startingInfo: startingInfo,
		})
	}

	ctx = withV166TestBlock(ctx, ctx.BlockHeight()+1, ctx.BlockTime().Add(time.Second))
	validator, err := sk.GetValidator(ctx, validatorAddress)
	require.NoError(t, err)
	coins := sdk.NewCoins(sdk.NewCoin(denom, validator.Tokens))
	require.NoError(t, app.BankKeeper.MintCoins(ctx, minttypes.ModuleName, coins))
	require.NoError(t, app.BankKeeper.SendCoinsFromModuleToModule(ctx, minttypes.ModuleName, distrtypes.ModuleName, coins))
	require.NoError(t, app.DistrKeeper.AllocateTokensToValidator(ctx, validator, sdk.NewDecCoinsFromCoins(coins...)))
	outstanding, err := app.DistrKeeper.GetValidatorOutstandingRewards(ctx, validatorAddress)
	require.NoError(t, err)
	supply := app.BankKeeper.GetSupply(ctx, denom)

	require.NoError(t, app.UpgradeKeeper.ApplyUpgrade(ctx, upgradetypes.Plan{Name: v166.UpgradeName, Height: ctx.BlockHeight()}))
	for _, state := range snapshots {
		require.Equal(t, state.base, app.AccountKeeper.GetAccount(ctx, state.address))
		require.Equal(t, state.balance, app.BankKeeper.GetAllBalances(ctx, state.address))
		require.Equal(t, state.balance, app.BankKeeper.SpendableCoins(ctx, state.address))
		delegation, err := sk.GetDelegation(ctx, state.address, validatorAddress)
		require.NoError(t, err)
		require.Equal(t, state.delegation, delegation)
		startingInfo, err := app.DistrKeeper.GetDelegatorStartingInfo(ctx, validatorAddress, state.address)
		require.NoError(t, err)
		require.Equal(t, state.startingInfo, startingInfo)

		ethAddress := common.BytesToAddress(state.address)
		code, err := app.EVMKeeper.Code(ctx, &evmtypes.QueryCodeRequest{Address: ethAddress.Hex()})
		require.NoError(t, err)
		require.Equal(t, contractCode, code.Code)
		require.Equal(t, codeHash, app.EVMKeeper.GetCodeHash(ctx, ethAddress))
		require.Equal(t, state.base.GetSequence(), app.EVMKeeper.GetNonce(ctx, ethAddress))
		require.Equal(t, storageValue, app.EVMKeeper.GetState(ctx, ethAddress, storageKey))
	}
	require.Equal(t, unrelatedAccount, app.AccountKeeper.GetAccount(ctx, unrelated))
	require.Empty(t, app.BankKeeper.SpendableCoins(ctx, unrelated))
	require.Equal(t, supply, app.BankKeeper.GetSupply(ctx, denom))
	updatedOutstanding, err := app.DistrKeeper.GetValidatorOutstandingRewards(ctx, validatorAddress)
	require.NoError(t, err)
	require.Equal(t, outstanding, updatedOutstanding)

	owner := snapshots[0]
	record, err := sk.GetUnbondingDelegation(ctx, owner.address, validatorAddress)
	require.NoError(t, err)
	require.Equal(t, pending, record)
	ctx = withV166TestBlock(ctx, ctx.BlockHeight()+1, completionTime)
	returned, err := sk.CompleteUnbonding(ctx, owner.address, validatorAddress)
	require.NoError(t, err)
	require.Equal(t, sdkmath.NewInt(10), returned.AmountOf(denom))
	require.Equal(t, owner.balance.Add(returned...), app.BankKeeper.SpendableCoins(ctx, owner.address))
}

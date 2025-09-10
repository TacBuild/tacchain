package e2e

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func TestTacchainTestSuite(t *testing.T) {
	suite.Run(t, new(TacchainTestSuite))
}

func (s *TacchainTestSuite) TestChainInitialization() {
	genesisPath := filepath.Join(s.homeDir, "config", "genesis.json")
	_, err := os.Stat(genesisPath)
	require.NoError(s.T(), err, "Genesis file should exist")

	configFiles := []string{
		"config.toml",
		"app.toml",
		"client.toml",
	}

	for _, file := range configFiles {
		path := filepath.Join(s.homeDir, "config", file)
		_, err := os.Stat(path)
		require.NoError(s.T(), err, "Config file %s should exist", file)
	}
}

func (s *TacchainTestSuite) TestBankBalances() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	params := s.CommandParamsHomeDir()
	output, err := ExecuteCommand(ctx, params, "status")
	require.NoError(s.T(), err, "Failed to get status: %s", output)

	validatorAddr, err := GetAddress(ctx, s, "validator")
	require.NoError(s.T(), err, "Failed to get validator address")

	balance, err := QueryBankBalances(ctx, s, validatorAddr)
	require.NoError(s.T(), err, "Failed to query balances: %s", balance)
	require.Contains(s.T(), balance, DefaultDenom, "Balance should contain utac denomination")
}

func (s *TacchainTestSuite) TestBankSend() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	params := s.DefaultCommandParams()
	_, err := ExecuteCommand(ctx, params, "keys", "add", "recipient")
	require.NoError(s.T(), err, "Failed to add recipient account")

	recipientAddr, err := GetAddress(ctx, s, "recipient")
	require.NoError(s.T(), err, "Failed to get recipient address")

	validatorAddr, err := GetAddress(ctx, s, "validator")
	require.NoError(s.T(), err, "Failed to get validator address")

	initialValidatorBalance, err := QueryBankBalances(ctx, s, validatorAddr)
	require.NoError(s.T(), err, "Failed to query validator balance")

	initialRecipientBalance, err := QueryBankBalances(ctx, s, recipientAddr)
	require.NoError(s.T(), err, "Failed to query recipient balance")

	amount := UTacAmount("1000000")
	_, err = TxBankSend(ctx, s, "validator", recipientAddr, amount)
	require.NoError(s.T(), err, "Failed to send tokens")

	waitForNewBlock(s, nil)

	finalValidatorBalance, err := QueryBankBalances(ctx, s, validatorAddr)
	require.NoError(s.T(), err, "Failed to query validator balance after tx")

	finalRecipientBalance, err := QueryBankBalances(ctx, s, recipientAddr)
	require.NoError(s.T(), err, "Failed to query recipient balance after tx")

	require.NotEqual(s.T(), initialValidatorBalance, finalValidatorBalance, "Validator balance should have changed")
	require.NotEqual(s.T(), initialRecipientBalance, finalRecipientBalance, "Recipient balance should have changed")
	require.Contains(s.T(), finalRecipientBalance, amount, "Recipient should have received the sent amount")
}

func (s *TacchainTestSuite) TestInflationRate() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	params := s.CommandParamsHomeDir()
	output, err := ExecuteCommand(ctx, params, "q", "mint", "params")
	require.NoError(s.T(), err, "Failed to query mint params: %s", output)

	inflationRateStr := parseField(output, "inflation_rate_change")
	require.NotEmpty(s.T(), inflationRateStr, "Inflation rate not found in mint params")

	inflationRate, err := strconv.ParseFloat(inflationRateStr, 64)
	require.NoError(s.T(), err, "Failed to parse inflation rate: %s", inflationRateStr)

	// Divide by 10^18 to convert from base units to percentage
	inflationRate = inflationRate / 1e18

	require.Greater(s.T(), inflationRate, 0.0, "Inflation rate should be positive")
	require.Less(s.T(), inflationRate, 0.20, "Inflation rate should be less than 20%")
}

func (s *TacchainTestSuite) TestStaking() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	validatorAddr, err := GetValidatorAddress(ctx, s)
	require.NoError(s.T(), err, "Failed to get validator address")

	params := s.CommandParamsHomeDir()
	output, err := ExecuteCommand(ctx, params, "q", "staking", "validator", validatorAddr)
	require.NoError(s.T(), err, "Failed to query validator info")

	delegatorShares := parseField(output, "delegator_shares")
	require.NotEmpty(s.T(), delegatorShares, "Delegator shares should not be empty")
}

func (s *TacchainTestSuite) TestDelegation() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	params := s.DefaultCommandParams()
	_, err := ExecuteCommand(ctx, params, "keys", "add", "delegator")
	require.NoError(s.T(), err, "Failed to add delegator account")

	delegatorAddr, err := GetAddress(ctx, s, "delegator")
	require.NoError(s.T(), err, "Failed to get delegator address")

	validatorAddr, err := GetValidatorAddress(ctx, s)
	require.NoError(s.T(), err, "Failed to get validator address")

	amount := UTacAmount("10000000000000000000")
	_, err = TxBankSend(ctx, s, "validator", delegatorAddr, amount)
	require.NoError(s.T(), err, "Failed to send tokens to delegator")

	waitForNewBlock(s, nil)

	delegationAmount := UTacAmount("500000")
	require.NoError(s.T(), err, "Failed to parse delegation amount")

	_, err = ExecuteCommand(ctx, params, "tx", "staking", "delegate", validatorAddr,
		delegationAmount, "--from", "delegator", "--gas-prices", "400000000000utac", "-y")
	require.NoError(s.T(), err, "Failed to delegate tokens")

	waitForNewBlock(s, nil)

	output, err := ExecuteCommand(ctx, params, "q", "staking", "delegation", delegatorAddr, validatorAddr)
	require.NoError(s.T(), err, "Failed to query delegation")

	delegatedAmount := parseBalanceAmount(output)
	require.Contains(s.T(), delegatedAmount, delegationAmount, "Delegated amount should match")
}

func (s *TacchainTestSuite) TestStakingAPR() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	params := s.DefaultCommandParams()

	_, err := ExecuteCommand(ctx, params, "keys", "add", "apr_delegator")
	require.NoError(s.T(), err, "Failed to add delegator account")

	delegatorAddr, err := GetAddress(ctx, s, "apr_delegator")
	require.NoError(s.T(), err, "Failed to get delegator address")

	validatorAddr, err := GetValidatorAddress(ctx, s)
	require.NoError(s.T(), err, "Failed to get validator address")

	initialAmount := UTacAmount("10000000000000000000")
	_, err = TxBankSend(ctx, s, "validator", delegatorAddr, initialAmount)
	require.NoError(s.T(), err, "Failed to send tokens to delegator")

	waitForNewBlock(s, nil)

	balance, err := QueryBankBalances(ctx, s, delegatorAddr)
	require.NoError(s.T(), err, "Failed to query delegator balance")
	require.Contains(s.T(), balance, initialAmount, "Delegator should have received the tokens")

	delegationAmount := UTacAmount("10000000000000000")
	output, err := ExecuteCommand(ctx, params, "tx", "staking", "delegate", validatorAddr,
		delegationAmount, "--from", "apr_delegator", "--gas", "200000", "--gas-prices", "400000000000utac", "-y")
	require.NoError(s.T(), err, "Failed to delegate tokens: %s", output)

	waitForNewBlock(s, nil)

	output, err = ExecuteCommand(ctx, params, "q", "staking", "delegation", delegatorAddr, validatorAddr)
	delegatedAmount := parseBalanceAmount(output)
	require.NoError(s.T(), err, "Failed to query delegation")
	require.Contains(s.T(), delegatedAmount, delegationAmount, "Delegation amount should match")

	// Wait for a few blocks to accumulate rewards
	blocksWaited := int(3)
	for i := 0; i < blocksWaited; i++ {
		waitForNewBlock(s, nil)
	}

	output, err = ExecuteCommand(ctx, params, "q", "distribution", "rewards", delegatorAddr)
	require.NoError(s.T(), err, "Failed to query rewards")

	rewardsAmount := parseBalanceAmount(output)
	rewardsAmount = rewardsAmount[:len(rewardsAmount)-len(DefaultDenom)]

	rewards, err := strconv.ParseInt(rewardsAmount, 10, 64)
	require.NoError(s.T(), err, "Failed to parse rewards amount")
	fmt.Print("Rewards: ", rewards, "\n")

	// blocksPerYear := int(10512000)
	// rewardsPerBlock := rewards / int64(blocksWaited)
	// rewardsForAYear := rewardsPerBlock * int64(blocksPerYear)
	//TODO: check if this formula is correct
	// apr := float64(rewardsForAYear) / float64(initialAmount) * 100
	// fmt.Print("APR: ", apr, "%\n")

	// TODO: uncomment this and tweak the values of expected APR
	// require.Greater(s.T(), apr, 5.0, "APR should be greater than 5%")
	// require.Less(s.T(), apr, 20.0, "APR should be less than 20%")
}

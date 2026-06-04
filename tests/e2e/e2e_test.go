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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	params := s.DefaultCommandParams()
	mintParams := s.CommandParamsHomeDir()

	// Setup delegator and delegation
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

	delegationAmountNum := float64(10000000000000000) // 1e16 utac
	delegationAmount := UTacAmount("10000000000000000")
	output, err := ExecuteCommand(ctx, params, "tx", "staking", "delegate", validatorAddr,
		delegationAmount, "--from", "apr_delegator", "--gas", "200000", "--gas-prices", "400000000000utac", "-y")
	require.NoError(s.T(), err, "Failed to delegate tokens: %s", output)

	waitForNewBlock(s, nil)

	output, err = ExecuteCommand(ctx, params, "q", "staking", "delegation", delegatorAddr, validatorAddr)
	require.NoError(s.T(), err, "Failed to query delegation")
	delegatedAmount := parseBalanceAmount(output)
	require.Contains(s.T(), delegatedAmount, delegationAmount, "Delegation amount should match")

	// Verify delegation rewards via dedicated query and dump rewards raw.
	delegRewardsOut, err := ExecuteCommand(ctx, params, "q", "distribution", "rewards-by-validator", delegatorAddr, validatorAddr)
	require.NoError(s.T(), err, "Failed to query rewards for specific validator: %s", delegRewardsOut)
	fmt.Printf("Rewards for specific validator: %s\n", delegRewardsOut)

	// Wait for a few blocks to accumulate rewards before measurement
	for i := 0; i < 5; i++ {
		waitForNewBlock(s, nil)
	}

	// Measure reward rate over 100 blocks for statistical accuracy.
	// Query rewards at t1, wait 100 blocks, query again at t2.
	rewardsBefore, err := func() (float64, error) {
		out, e := ExecuteCommand(ctx, params, "q", "distribution", "rewards", delegatorAddr)
		if e != nil {
			return 0, e
		}
		return parseRewardsFloat(out, DefaultDenom)
	}()
	require.NoError(s.T(), err, "Failed to query rewards (before)")
	heightBefore := getCurrentBlockHeight(s)
	t1 := time.Now()
	fmt.Printf("Measurement start — height: %d, rewards: %.2f utac\n", heightBefore, rewardsBefore)

	for i := 0; i < 100; i++ {
		waitForNewBlock(s, nil)
	}

	rewardsAfter, err := func() (float64, error) {
		out, e := ExecuteCommand(ctx, params, "q", "distribution", "rewards", delegatorAddr)
		if e != nil {
			return 0, e
		}
		return parseRewardsFloat(out, DefaultDenom)
	}()
	require.NoError(s.T(), err, "Failed to query rewards (after)")
	heightAfter := getCurrentBlockHeight(s)
	t2 := time.Now()
	fmt.Printf("Measurement end   — height: %d, rewards: %.2f utac\n", heightAfter, rewardsAfter)

	// Delta rewards over 100-block interval
	rewards := rewardsAfter - rewardsBefore
	blockDuration := t2.Sub(t1).Seconds()
	blocksElapsed := heightAfter - heightBefore
	require.Greater(s.T(), rewards, 0.0, "Rewards delta should be greater than 0")
	fmt.Printf("Rewards delta: %.2f utac over %d blocks in %.2fs\n", rewards, blocksElapsed, blockDuration)
	fmt.Printf("Per-block reward (measured): %.2f utac\n", rewards/float64(blocksElapsed))

	// Annualize using real block time
	secondsPerYear := float64(365.25 * 24 * 3600)
	rewardsAnnualized := rewards / blockDuration * secondsPerYear
	measuredAPR := rewardsAnnualized / delegationAmountNum * 100
	fmt.Printf("Measured APR: %.4f%%\n", measuredAPR)

	// Query current inflation rate
	inflationOutput, err := ExecuteCommand(ctx, mintParams, "q", "mint", "inflation")
	require.NoError(s.T(), err, "Failed to query inflation")
	inflationStr := parseField(inflationOutput, "inflation")
	currentInflation, err := strconv.ParseFloat(inflationStr, 64)
	require.NoError(s.T(), err, "Failed to parse inflation: %s", inflationStr)
	fmt.Printf("Current inflation: %.4f%%\n", currentInflation*100)

	// Query bonded tokens and total supply to compute bonded ratio
	poolOutput, err := ExecuteCommand(ctx, mintParams, "q", "staking", "pool")
	require.NoError(s.T(), err, "Failed to query staking pool")
	bondedStr := parseField(poolOutput, "bonded_tokens")
	bonded, err := strconv.ParseFloat(bondedStr, 64)
	require.NoError(s.T(), err, "Failed to parse bonded tokens")

	supplyOutput, err := ExecuteCommand(ctx, mintParams, "q", "bank", "total-supply-of", DefaultDenom)
	require.NoError(s.T(), err, "Failed to query total supply")
	totalSupply, err := parseTotalSupply(supplyOutput)
	require.NoError(s.T(), err, "Failed to parse total supply: %s", supplyOutput)

	bondedRatio := bonded / totalSupply
	fmt.Printf("Bonded: %.0f, Total supply: %.0f, Bonded ratio: %.4f%%\n", bonded, totalSupply, bondedRatio*100)

	// Theoretical APR using on-chain annual_provisions directly
	annualProvisionsOut, err := ExecuteCommand(ctx, mintParams, "q", "mint", "annual-provisions")
	require.NoError(s.T(), err, "Failed to query annual provisions")
	annualProvisionsStr := parseField(annualProvisionsOut, "annual_provisions")
	annualProvisions, err := strconv.ParseFloat(annualProvisionsStr, 64)
	require.NoError(s.T(), err, "Failed to parse annual provisions")
	fmt.Printf("Annual provisions (chain): %.2f\n", annualProvisions)

	// commission from validator query
	validatorOut2, err := ExecuteCommand(ctx, params, "q", "staking", "validator", validatorAddr)
	require.NoError(s.T(), err, "Failed to query validator for commission")
	commissionRateStr := parseField(validatorOut2, "rate")
	commissionRateF, parseErr := strconv.ParseFloat(commissionRateStr, 64)
	if parseErr != nil {
		commissionRateF = 0
	}
	fmt.Printf("Validator commission rate: %.4f\n", commissionRateF)

	// The chain mints annualProvisions/blocksPerYear per block.
	// annualProvisions assumes a specific expected block time (secondsPerYear/blocksPerYear).
	// If actual block time differs, we must scale accordingly.
	blocksPerYear := float64(15768000) // from genesis
	expectedBlockTime := secondsPerYear / blocksPerYear
	actualBlockTime := blockDuration / float64(blocksElapsed)
	// theoreticalAPR = (annualProvisions / blocksPerYear) * (1/actualBlockTime) * secondsPerYear
	//                  / bonded * (1 - commission) * 100
	//                = annualProvisions * (expectedBlockTime/actualBlockTime) / bonded * (1-commission) * 100
	theoreticalAPR := annualProvisions / bonded * (1 - commissionRateF) * (expectedBlockTime / actualBlockTime) * 100
	fmt.Printf("Expected block time: %.4fs, Actual block time: %.4fs, Scale: %.4f\n",
		expectedBlockTime, actualBlockTime, expectedBlockTime/actualBlockTime)
	fmt.Printf("Theoretical APR (adjusted for actual block time): %.4f%%\n", theoreticalAPR)

	// Tolerance of 10% (relative) — should be tight since we measured over 100+ blocks.
	require.InDelta(s.T(), theoreticalAPR, measuredAPR, theoreticalAPR*0.10,
		"Measured APR %.4f%% should be within 10%% of theoretical APR %.4f%%", measuredAPR, theoreticalAPR)
}

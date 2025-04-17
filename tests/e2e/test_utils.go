package e2e

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/stretchr/testify/suite"
)

const (
	DefaultChainID        = "tacchain_2390-1"
	DefaultDenom          = "utac"
	DefaultKeyringBackend = "test"
)

type TacchainTestSuite struct {
	suite.Suite
	homeDir string
	cmd     *exec.Cmd
}

type CommandParams struct {
	ChainID        string
	HomeDir        string
	KeyringBackend string
}

func (s *TacchainTestSuite) CommandParamsHomeDir() CommandParams {
	return CommandParams{
		HomeDir: s.homeDir,
	}
}

func (s *TacchainTestSuite) CommandParamsChainIDHomeDir() CommandParams {
	return CommandParams{
		ChainID: DefaultChainID,
		HomeDir: s.homeDir,
	}
}

func (s *TacchainTestSuite) DefaultCommandParams() CommandParams {
	return CommandParams{
		ChainID:        DefaultChainID,
		HomeDir:        s.homeDir,
		KeyringBackend: DefaultKeyringBackend,
	}
}

func DefaultExecuteCommand(ctx context.Context, params CommandParams, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "tacchaind", args...)

	cmd.Args = append(cmd.Args, "--home", params.HomeDir)

	if params.ChainID != "" {
		cmd.Args = append(cmd.Args, "--chain-id", params.ChainID)
	}

	if params.KeyringBackend != "" {
		cmd.Args = append(cmd.Args, "--keyring-backend", params.KeyringBackend)
	}

	output, err := cmd.CombinedOutput()
	return string(output), err
}

func GetAddress(ctx context.Context, s *TacchainTestSuite, keyName string) (string, error) {
	params := s.DefaultCommandParams()
	output, err := DefaultExecuteCommand(ctx, params, "keys", "show", keyName, "-a")
	if err != nil {
		return "", fmt.Errorf("failed to get %s address: %v", keyName, err)
	}
	return strings.TrimSpace(output), nil
}

func QueryBankBalances(ctx context.Context, s *TacchainTestSuite, address string) (string, error) {
	params := s.CommandParamsHomeDir()
	output, err := DefaultExecuteCommand(ctx, params, "q", "bank", "balances", address)
	if err != nil {
		return "", fmt.Errorf("failed to query balance: %v", err)
	}
	return parseBalanceAmount(output), nil
}

func TxBankSend(ctx context.Context, s *TacchainTestSuite, from, to string, amount int64) (string, error) {
	params := s.DefaultCommandParams()
	amountWithDenom := fmt.Sprintf("%d%s", amount, DefaultDenom)
	output, err := DefaultExecuteCommand(ctx, params, "tx", "bank", "send", from, to, amountWithDenom, "--gas", "200000", "-y")
	return output, err
}

func parseField(output string, fieldName string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		if strings.Contains(line, fieldName+":") {
			parts := strings.Split(strings.TrimSpace(line), ":")
			if len(parts) == 2 {
				return strings.Trim(strings.TrimSpace(parts[1]), "\"")
			}
		}
	}
	return ""
}

func parseBalanceAmount(balanceOutput string) string {
	lines := strings.Split(balanceOutput, "\n")
	var amount, denom string
	for _, line := range lines {
		if strings.Contains(line, "amount:") {
			parts := strings.Split(strings.TrimSpace(line), ":")
			if len(parts) == 2 {
				amount = strings.Trim(strings.TrimSpace(parts[1]), "\"")
			}
		}
		if strings.Contains(line, "denom:") {
			parts := strings.Split(strings.TrimSpace(line), ":")
			if len(parts) == 2 {
				denom = strings.Trim(strings.TrimSpace(parts[1]), "\"")
			}
		}
	}
	if amount == "" {
		return "0" + DefaultDenom
	}
	return amount + denom
}

func killProcessOnPort(port int) error {
	cmd := exec.Command("lsof", "-i", fmt.Sprintf(":%d", port), "-t")
	output, err := cmd.Output()
	if err != nil {
		cmd = exec.Command("netstat", "-ano", "-p", "tcp")
		output, err = cmd.Output()
		if err != nil {
			fmt.Printf("Warning: could not check for processes on port %d: %v\n", port, err)
			return nil
		}

		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.Contains(line, fmt.Sprintf(":%d", port)) {
				fields := strings.Fields(line)
				if len(fields) > 0 {
					pid := fields[len(fields)-1]
					killCmd := exec.Command("kill", "-9", pid)
					if err := killCmd.Run(); err != nil {
						fmt.Printf("Warning: failed to kill process %s: %v\n", pid, err)
					}
					fmt.Printf("Killed process %s on port %d\n", pid, port)
				}
			}
		}
		return nil
	}

	pids := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, pid := range pids {
		if pid == "" {
			continue
		}
		killCmd := exec.Command("kill", "-9", pid)
		if err := killCmd.Run(); err != nil {
			return fmt.Errorf("failed to kill process %s: %v", pid, err)
		}
	}
	return nil
}

func getCurrentBlockHeight(s *TacchainTestSuite) int64 {
	ctx := context.Background()
	params := s.CommandParamsHomeDir()
	output, err := DefaultExecuteCommand(ctx, params, "q", "block")
	if err != nil {
		return -1
	}

	return parseBlockHeight(string(output))
}

func parseBlockHeight(output string) int64 {
	lines := strings.Split(output, "\n  ")
	for i := 1; i < len(lines); i++ {
		line := lines[i]
		if strings.Contains(line, "height:") {
			parts := strings.Split(line, ":")
			if len(parts) == 2 {
				heightStr := strings.TrimSpace(parts[1])
				height, err := strconv.ParseInt(strings.Trim(heightStr, `"`), 10, 64)
				if err == nil {
					return height
				}
			}
		}
	}

	return -1
}

func waitForNewBlock(s *TacchainTestSuite, stderr io.ReadCloser) {
	maxAttempts := 30
	attempt := 0

	initialHeight := getCurrentBlockHeight(s)

	for attempt < maxAttempts {
		currentHeight := getCurrentBlockHeight(s)
		if currentHeight > initialHeight {
			s.T().Logf("New block minted at height %d", currentHeight)
			return
		}

		attempt++
		if attempt == maxAttempts {
			if s.cmd.ProcessState != nil && s.cmd.ProcessState.Exited() {
				errOutput, _ := io.ReadAll(stderr)
				s.T().Fatalf("Chain process exited unexpectedly: %s", string(errOutput))
			}
			s.T().Fatalf("Chain failed to produce new block after %d attempts", maxAttempts)
		}

		time.Sleep(2 * time.Second)
		s.T().Logf("Waiting for new block (attempt %d/%d)", attempt, maxAttempts)
	}
}

func UTacAmount(amount int64) string {
	return fmt.Sprintf("%d%s", amount, DefaultDenom)
}

func GetValidatorAddress(ctx context.Context, s *TacchainTestSuite) (string, error) {
	params := s.CommandParamsHomeDir()
	output, err := DefaultExecuteCommand(ctx, params, "q", "staking", "historical-info", "1")
	if err != nil {
		return "", fmt.Errorf("failed to query validator info: %v", err)
	}

	validatorAddr := parseField(output, "operator_address")
	if validatorAddr == "" {
		return "", fmt.Errorf("validator operator address is empty")
	}

	return validatorAddr, nil
}

func CreateFeemarketProposalFile(s *TacchainTestSuite, newBaseFee string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	params := s.CommandParamsHomeDir()
	output, err := DefaultExecuteCommand(ctx, params, "q", "auth", "module-account", "gov")
	if err != nil {
		return "", fmt.Errorf("failed to get governance module address: %v", err)
	}

	governanceAddr := parseField(output, "address")
	if governanceAddr == "" {
		return "", fmt.Errorf("failed to extract governance module address")
	}

	proposalContent := fmt.Sprintf(`{ 
	"messages": [
		{
			"@type": "/ethermint.feemarket.v1.MsgUpdateParams",
			"authority": "%s",
			"params": {
				"no_base_fee": false,
				"base_fee_change_denominator": 8,
				"elasticity_multiplier": 2,
				"enable_height": "0",
				"base_fee": "%s", 
				"min_gas_price": "0.000000000000000000",
				"min_gas_multiplier": "0.500000000000000000"
			}
		}
	],
	"metadata": "ipfs://CID",
	"deposit": "20000000utac",
	"title": "test",
	"summary": "test",
	"expedited": false
}`, governanceAddr, newBaseFee)

	proposalFile := filepath.Join(s.homeDir, "draft_proposal.json")
	err = os.WriteFile(proposalFile, []byte(proposalContent), 0644)
	if err != nil {
		return "", fmt.Errorf("failed to write proposal file: %v", err)
	}

	return proposalFile, nil
}

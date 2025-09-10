package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/stretchr/testify/suite"
)

const (
	DefaultChainID        = "tacchain_2391-1"
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

func ExecuteCommand(ctx context.Context, params CommandParams, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "tacchaind", args...)
	cmd.Args = append(cmd.Args, "--home", params.HomeDir)

	if params.ChainID != "" {
		cmd.Args = append(cmd.Args, "--chain-id", params.ChainID)
	}

	if params.KeyringBackend != "" {
		cmd.Args = append(cmd.Args, "--keyring-backend", params.KeyringBackend)
	}

	output, err := cmd.CombinedOutput()
	strOutput := string(output)

	// NOTE: This Warning gets thrown on go 1.24 and gets applied to the output
	sonicWarning := "WARNING:(ast) sonic only supports go1.17~1.23, but your environment is not suitable\n"
	strOutput = strings.Replace(strOutput, sonicWarning, "", 1)

	// Check for errors in the output in case of tx commands
	// TODO: parseField(output, "code") doesn't return the code correctly
	// TODO: additionally tx can fail after a txHash is returned. ideally we want to q tx <txHash> and also check it
	var txCode, rawLog string
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(strOutput), &m); err == nil {
		if codeVal, ok := m["code"]; ok {
			txCode = fmt.Sprintf("%v", codeVal)
			rawLog = fmt.Sprintf("%v", m["raw_log"])
		}
	}
	if txCode != "" && txCode != "0" {
		return strOutput, fmt.Errorf("command failed with code %s, err: %s", txCode, rawLog)
	}

	return strOutput, err
}

func GetAddress(ctx context.Context, s *TacchainTestSuite, keyName string) (string, error) {
	params := s.DefaultCommandParams()
	output, err := ExecuteCommand(ctx, params, "keys", "show", keyName, "-a")
	if err != nil {
		return "", fmt.Errorf("failed to get %s address: %v", keyName, err)
	}
	return strings.TrimSpace(output), nil
}

func QueryBankBalances(ctx context.Context, s *TacchainTestSuite, address string) (string, error) {
	params := s.CommandParamsHomeDir()
	output, err := ExecuteCommand(ctx, params, "q", "bank", "balances", address)
	if err != nil {
		return "", fmt.Errorf("failed to query balance: %v", err)
	}
	return parseBalanceAmount(output), nil
}

func TxBankSend(ctx context.Context, s *TacchainTestSuite, from, to string, utacAmount string) (string, error) {
	params := s.DefaultCommandParams()
	output, err := ExecuteCommand(ctx, params, "tx", "bank", "send", from, to, utacAmount, "--gas", "200000", "--gas-prices", "400000000000utac", "-y")
	return output, err
}

func parseBlockHeight(output string) int64 {
	lines := strings.Split(output, "\n")
	if len(lines) < 2 {
		return -1
	}

	secondLine := lines[1]

	heightIdx := strings.Index(secondLine, "\"height\":\"")
	if heightIdx >= 0 {
		startIdx := heightIdx + 10
		endIdx := strings.Index(secondLine[startIdx:], "\"")
		if endIdx >= 0 {
			heightStr := secondLine[startIdx : startIdx+endIdx]
			height, err := strconv.ParseInt(heightStr, 10, 64)
			if err == nil {
				return height
			}
		}
	}

	return -1
}

func parseField(output string, fieldName string) string {
	if strings.Count(output, "\n") <= 1 {
		idx := strings.Index(output, "\""+fieldName+"\":\"")
		if idx >= 0 {
			startIdx := idx + len("\""+fieldName+"\":\"")
			endIdx := strings.Index(output[startIdx:], "\"")
			if endIdx >= 0 {
				return output[startIdx : startIdx+endIdx]
			}
		}
	}

	lines := strings.Split(output, "\n")
	quotedField := "\"" + fieldName + "\":"

	for _, line := range lines {
		trimLine := strings.TrimSpace(line)
		if strings.Contains(trimLine, quotedField) {
			parts := strings.Split(trimLine, ":")
			if len(parts) == 2 {
				return strings.Trim(strings.TrimSpace(parts[1]), "\",")
			}
		}
	}
	return ""
}

func parseBalanceAmount(balanceOutput string) string {
	amount := parseField(balanceOutput, "amount")
	if amount == "" {
		return UTacAmount("0")
	}

	return UTacAmount(amount)
}

func killProcessOnPort(port int) error {
	cmd := exec.Command("lsof", "-i", fmt.Sprintf(":%d", port), "-t")
	output, err := cmd.Output()
	if err != nil {
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
	output, err := ExecuteCommand(ctx, params, "q", "block")
	if err != nil {
		return -1
	}

	return parseBlockHeight(string(output))
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

func UTacAmount(amount string) string {
	return fmt.Sprintf("%s%s", amount, DefaultDenom)
}

func GetValidatorAddress(ctx context.Context, s *TacchainTestSuite) (string, error) {
	params := s.DefaultCommandParams()
	validatorAddr, err := ExecuteCommand(ctx, params, "keys", "show", "validator", "--bech", "val", "-a")
	if err != nil {
		return "", fmt.Errorf("failed to query validator info: %v", err)
	}
	return strings.TrimSpace(validatorAddr), nil
}

func ParseBoolField(output string, fieldName string) (bool, bool) {
	truePattern := "\"" + fieldName + "\":true"
	falsePattern := "\"" + fieldName + "\":false"

	isTrue := strings.Contains(output, truePattern)
	isFalse := strings.Contains(output, falsePattern)

	found := isTrue || isFalse

	return isTrue, found
}

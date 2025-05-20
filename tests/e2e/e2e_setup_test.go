package e2e

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func (s *TacchainTestSuite) SetupSuite() {
	s.T().Log("Setting up test suite...")

	if err := killProcessOnPort(26657); err != nil {
		s.T().Logf("Warning: Failed to kill process on port 26657: %v", err)
	}

	dir, err := os.MkdirTemp("", "tacchain-test")
	if err != nil {
		s.T().Fatalf("Failed to create temporary directory: %v", err)
	}
	s.homeDir = dir

	if err := s.initChain(); err != nil {
		s.T().Fatalf("Failed to initialize chain: %v", err)
	}
	if err := s.startChain(); err != nil {
		s.T().Fatalf("Failed to start chain: %v", err)
	}
}

func (s *TacchainTestSuite) initChain() error {
	s.T().Log("Initializing chain...")

	nodeDir := s.homeDir
	pwd, _ := os.Getwd()
	initScript := filepath.Join(pwd, "../../contrib/localnet/init.sh")
	cmd := exec.Command("bash", "-c", fmt.Sprintf("echo y | HOMEDIR=%s %s", nodeDir, initScript))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to initialize chain: %v", err)
	}

	if err := ModifyInitialChainConfig(s.homeDir); err != nil {
		return fmt.Errorf("failed to modify chain config: %v", err)
	}

	return nil
}

func (s *TacchainTestSuite) startChain() error {
	s.T().Log("Starting chain process...")

	s.cmd = exec.Command("tacchaind", "start", "--chain-id", DefaultChainID, "--home", s.homeDir)

	stderr, err := s.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %v", err)
	}

	err = s.cmd.Start()
	if err != nil {
		return fmt.Errorf("failed to start chain: %v", err)
	}

	s.T().Log("Waiting 2 seconds for chain to initialize...")
	time.Sleep(2 * time.Second)

	s.T().Log("Waiting for chain to start producing blocks...")
	waitForNewBlock(s, stderr)

	if s.cmd.ProcessState != nil && s.cmd.ProcessState.Exited() {
		errOutput, _ := io.ReadAll(stderr)
		return fmt.Errorf("chain process exited unexpectedly: %s", string(errOutput))
	}

	return nil
}

func ModifyInitialChainConfig(homeDir string) error {
	genesisPath := filepath.Join(homeDir, "config", "genesis.json")
	genesisData, err := os.ReadFile(genesisPath)
	if err != nil {
		return fmt.Errorf("failed to read genesis file: %v", err)
	}

	var genesis map[string]any
	if err := json.Unmarshal(genesisData, &genesis); err != nil {
		return fmt.Errorf("failed to unmarshal genesis: %v", err)
	}

	if appState, ok := genesis["app_state"].(map[string]any); ok {
		if gov, ok := appState["gov"].(map[string]any); ok {
			// Modify voting period
			if params, ok := gov["params"].(map[string]any); ok {
				params["voting_period"] = "3s"
				params["expedited_voting_period"] = "3s"
			}
		}
		if feemarket, ok := appState["feemarket"].(map[string]any); ok {
			// Modify no_base_fee
			if params, ok := feemarket["params"].(map[string]any); ok {
				params["no_base_fee"] = true
			}
		}
	}

	modifiedGenesis, err := json.MarshalIndent(genesis, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal modified genesis: %v", err)
	}

	if err := os.WriteFile(genesisPath, modifiedGenesis, 0644); err != nil {
		return fmt.Errorf("failed to write modified genesis: %v", err)
	}

	return nil
}

func (s *TacchainTestSuite) TearDownSuite() {
	s.T().Log("Tearing down Tacchain test suite...")

	if s.cmd != nil {
		s.T().Log("Stopping chain process...")
		if err := s.cmd.Process.Kill(); err != nil {
			s.T().Logf("Error stopping chain process: %v", err)
		}
		s.cmd.Wait()
	}

	if err := os.RemoveAll(s.homeDir); err != nil {
		s.T().Logf("Error cleaning up test directory: %v", err)
	}
}

// // SPDX-License-Identifier: BUSL-1.1-or-later
// // SPDX-FileCopyrightText: 2025 Web3 Technologies Inc. <https://asphere.xyz/>
// // Copyright (c) 2025 Web3 Technologies Inc. All rights reserved.
// // Use of this software is governed by the Business Source License included in the LICENSE file <https://github.com/Asphere-xyz/tacchain/blob/main/LICENSE>.
package main

// DONTCOVER

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/Asphere-xyz/tacchain/app"
	cmtconfig "github.com/cometbft/cometbft/config"
	"github.com/cometbft/cometbft/types"
	cmttime "github.com/cometbft/cometbft/types/time"
	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	"github.com/cosmos/cosmos-sdk/server"
	srvconfig "github.com/cosmos/cosmos-sdk/server/config"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/cosmos/cosmos-sdk/version"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	evmhd "github.com/cosmos/evm/crypto/hd"
	evmkeyring "github.com/cosmos/evm/crypto/keyring"
	"github.com/cosmos/evm/evmd"
	evmconfig "github.com/cosmos/evm/server/config"
	evmtestutil "github.com/cosmos/evm/testutil/network"
	evmerc20types "github.com/cosmos/evm/x/erc20/types"
	evmvmtypes "github.com/cosmos/evm/x/vm/types"
	gethparams "github.com/ethereum/go-ethereum/params"
)

var (
	flagNodeDirPrefix     = "node-dir-prefix"
	flagNumValidators     = "v"
	flagOutputDir         = "output-dir"
	flagNodeDaemonHome    = "node-daemon-home"
	flagStartingIPAddress = "starting-ip-address"
	flagEnableLogging     = "enable-logging"
	flagGRPCAddress       = "grpc.address"
	flagRPCAddress        = "rpc.address"
	flagAPIAddress        = "api.address"
	flagPrintMnemonic     = "print-mnemonic"
	// custom flags
	flagCommitTimeout   = "commit-timeout"
	flagSingleHost      = "single-host"
	flagBaseFee         = "base-fee"
	flagMinGasPrice     = "min-gas-price"
	flagMaxGas          = "max-gas"
	flagVotingPeriod    = "voting-period"
	flagInitialBalances = "initial-balances"
	flagInitialStaking  = "initial-staking"
)

type initArgs struct {
	algo              string
	chainID           string
	keyringBackend    string
	minGasPrices      string
	nodeDaemonHome    string
	nodeDirPrefix     string
	numValidators     int
	outputDir         string
	startingIPAddress string
	singleHost        bool
	baseFee           int
	minGasPrice       int
	maxGas            int
	votingPeriod      time.Duration
	timeoutCommit     time.Duration
	initialBalances   int
	initialStaking    int
}

// createValidatorMsgGasLimit is the gas limit used in the MsgCreateValidator included in genesis transactions.
// This transaction consumes approximately 220,000 gas when executed in the genesis block.
const createValidatorMsgGasLimit = 250_000

// by default erc20 TokenPair.ContractOwner enum is marshalled as string, but is expected as int
// temporary struct for marshaling TokenPairs with ContractOwner as an integer instead of a string
type TokenPairForMarshal struct {
	Erc20Address  string `json:"erc20_address"`
	Denom         string `json:"denom"`
	Enabled       bool   `json:"enabled"`
	ContractOwner int    `json:"contract_owner"`
}

// NewTestnetCmd creates a root testnet command with subcommands to run an in-process testnet or initialize
// validator configuration files for running a multi-validator testnet in a separate process
func NewTestnetCmd(mbm module.BasicManager, genBalIterator banktypes.GenesisBalancesIterator) *cobra.Command {
	testnetCmd := &cobra.Command{
		Use:                        "testnet",
		Short:                      "subcommands for starting or configuring local testnets",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	testnetCmd.AddCommand(testnetInitFilesCmd(mbm, genBalIterator))

	return testnetCmd
}

// testnetInitFilesCmd returns a cmd to initialize all files for CometBFT testnet and application
func testnetInitFilesCmd(mbm module.BasicManager, genBalIterator banktypes.GenesisBalancesIterator) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init-files",
		Short: "Initialize config directories & files for a multi-validator testnet running locally via separate processes (e.g. Docker Compose or similar)",
		Long: fmt.Sprintf(`init-files will setup "v" number of directories and populate each with
necessary files (private validator, genesis, config, etc.) for running "v" validator nodes.

Booting up a network with these validator folders is intended to be used with Docker Compose,
or a similar setup where each node has a manually configurable IP address.

Note, strict routability for addresses is turned off in the config file.

Example:
	%s testnet init-files --v 4 --output-dir ./.testnet --starting-ip-address 192.168.10.2
	`, version.AppName),
		RunE: func(cmd *cobra.Command, _ []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			serverCtx := server.GetServerContextFromCmd(cmd)
			config := serverCtx.Config

			args := initArgs{}
			args.outputDir, _ = cmd.Flags().GetString(flagOutputDir)
			args.keyringBackend, _ = cmd.Flags().GetString(flags.FlagKeyringBackend)
			args.chainID, _ = cmd.Flags().GetString(flags.FlagChainID)
			args.minGasPrices, _ = cmd.Flags().GetString(server.FlagMinGasPrices)
			args.nodeDirPrefix, _ = cmd.Flags().GetString(flagNodeDirPrefix)
			args.nodeDaemonHome, _ = cmd.Flags().GetString(flagNodeDaemonHome)
			args.startingIPAddress, _ = cmd.Flags().GetString(flagStartingIPAddress)
			args.numValidators, _ = cmd.Flags().GetInt(flagNumValidators)
			args.algo, _ = cmd.Flags().GetString(flags.FlagKeyType)
			args.baseFee, _ = cmd.Flags().GetInt(flagBaseFee)
			args.minGasPrice, _ = cmd.Flags().GetInt(flagMinGasPrice)
			args.maxGas, _ = cmd.Flags().GetInt(flagMaxGas)
			args.initialBalances, _ = cmd.Flags().GetInt(flagInitialBalances)
			args.initialStaking, _ = cmd.Flags().GetInt(flagInitialStaking)
			args.singleHost, _ = cmd.Flags().GetBool(flagSingleHost)
			args.votingPeriod, err = cmd.Flags().GetDuration(flagVotingPeriod)
			if err != nil {
				return err
			}
			config.Consensus.TimeoutCommit, err = cmd.Flags().GetDuration(flagCommitTimeout)
			if err != nil {
				return err
			}
			args.timeoutCommit = config.Consensus.TimeoutCommit

			return initTestnetFiles(clientCtx, cmd, config, mbm, genBalIterator, clientCtx.TxConfig.SigningContext().ValidatorAddressCodec(), args)
		},
	}

	cmd.Flags().Int(flagNumValidators, 4, "Number of validators to initialize the testnet with")
	cmd.Flags().StringP(flagOutputDir, "o", "./.testnet", "Directory to store initialization data for the testnet")
	cmd.Flags().String(flags.FlagChainID, app.DefaultChainID, "Chain ID for the network")
	cmd.Flags().String(server.FlagMinGasPrices, fmt.Sprintf("0%s", sdk.DefaultBondDenom), "Minimum gas prices to accept for transactions; All fees in a tx must meet this minimum (e.g. 0.01photino,0.001stake)")
	cmd.Flags().String(flags.FlagKeyType, string(evmhd.EthSecp256k1Type), "Key signing algorithm to generate keys for")
	cmd.Flags().String(flagBaseFee, strconv.Itoa(gethparams.InitialBaseFee), "The params base_fee in the feemarket module in geneis")
	cmd.Flags().String(flagMinGasPrice, "0", "The params min_gas_price in the feemarket module in geneis")
	cmd.Flags().Int(flagMaxGas, 20000000, "Max gas limit for a block")
	cmd.Flags().Duration(flagVotingPeriod, time.Minute*15, "Voting period for governance proposals")
	cmd.Flags().Int(flagInitialBalances, 10, "Initial balance for each validator account")
	cmd.Flags().Int(flagInitialStaking, 9, "Initial self-delegated amount for each validator")
	cmd.Flags().String(flagNodeDirPrefix, "node", "Prefix the directory name for each node with (node results in node0, node1, ...)")
	cmd.Flags().String(flagNodeDaemonHome, version.AppName, "Home directory of the node's daemon configuration")
	cmd.Flags().String(flagStartingIPAddress, "", "Starting IP address (192.168.0.1 results in persistent peers list ID0@192.168.0.1:46656, ID1@192.168.0.2:46656, ...)")
	cmd.Flags().String(flags.FlagKeyringBackend, "test", "Select keyring's backend (os|file|test)")
	cmd.Flags().Duration(flagCommitTimeout, 3*time.Second, "Time to wait after a block commit before starting on the new height")
	cmd.Flags().Bool(flagSingleHost, false, "Cluster runs on a single host machine with different ports")

	return cmd
}

const nodeDirPerm = 0o755

// initTestnetFiles initializes testnet files for a testnet to be run in a separate process
func initTestnetFiles(
	clientCtx client.Context,
	cmd *cobra.Command,
	nodeConfig *cmtconfig.Config,
	mbm module.BasicManager,
	genBalIterator banktypes.GenesisBalancesIterator,
	valAddrCodec runtime.ValidatorAddressCodec,
	args initArgs,
) error {
	if args.chainID == "" {
		args.chainID = app.DefaultChainID
	}
	nodeIDs := make([]string, args.numValidators)
	valPubKeys := make([]cryptotypes.PubKey, args.numValidators)

	appConfig := evmconfig.DefaultConfig()
	appConfig.MinGasPrices = args.minGasPrices
	appConfig.API.Enable = true
	appConfig.GRPC.Enable = true
	appConfig.JSONRPC.Enable = true
	appConfig.JSONRPC.AllowUnprotectedTxs = true
	appConfig.Telemetry.Enabled = true
	appConfig.Telemetry.PrometheusRetentionTime = 60
	appConfig.Telemetry.EnableHostnameLabel = false
	appConfig.Telemetry.GlobalLabels = [][]string{{"chain_id", args.chainID}}

	var (
		genAccounts []authtypes.GenesisAccount
		genBalances []banktypes.Balance
		genFiles    []string
	)
	const (
		rpcPort  = 26657
		apiPort  = 1317
		grpcPort = 9090
	)
	p2pPortStart := 26656

	inBuf := bufio.NewReader(cmd.InOrStdin())
	// generate private keys, node IDs, and initial transactions
	for i := 0; i < args.numValidators; i++ {
		var portOffset int
		if args.singleHost {
			portOffset = i
			p2pPortStart = 16656 // use different start point to not conflict with rpc port
			nodeConfig.P2P.AddrBookStrict = false
			nodeConfig.P2P.PexReactor = false
			nodeConfig.P2P.AllowDuplicateIP = true
		}

		nodeDirName := fmt.Sprintf("%s%d", args.nodeDirPrefix, i)
		nodeDir := filepath.Join(args.outputDir, nodeDirName, args.nodeDaemonHome)
		gentxsDir := filepath.Join(args.outputDir, "gentxs")

		nodeConfig.SetRoot(nodeDir)
		nodeConfig.Moniker = nodeDirName
		nodeConfig.RPC.ListenAddress = fmt.Sprintf("tcp://localhost:%d", rpcPort+portOffset)
		appConfig.API.Address = fmt.Sprintf("tcp://localhost:%d", apiPort+portOffset)
		appConfig.GRPC.Address = fmt.Sprintf("localhost:%d", grpcPort+portOffset)
		appConfig.GRPCWeb.Enable = true

		if err := os.MkdirAll(filepath.Join(nodeDir, "config"), nodeDirPerm); err != nil {
			_ = os.RemoveAll(args.outputDir)
			return err
		}

		ip, err := getIP(i, args.startingIPAddress)
		if err != nil {
			_ = os.RemoveAll(args.outputDir)
			return err
		}

		nodeIDs[i], valPubKeys[i], err = genutil.InitializeNodeValidatorFiles(nodeConfig)
		if err != nil {
			_ = os.RemoveAll(args.outputDir)
			return err
		}

		memo := fmt.Sprintf("%s@%s:%d", nodeIDs[i], ip, p2pPortStart+portOffset)
		genFiles = append(genFiles, nodeConfig.GenesisFile())

		kb, err := keyring.New(sdk.KeyringServiceName(), args.keyringBackend, nodeDir, inBuf, clientCtx.Codec, evmkeyring.Option())
		if err != nil {
			return err
		}

		keyringAlgos, _ := kb.SupportedAlgorithms()
		algo, err := keyring.NewSigningAlgoFromString(args.algo, keyringAlgos)
		if err != nil {
			return err
		}

		addr, secret, err := testutil.GenerateSaveCoinKey(kb, nodeDirName, "", true, algo)
		if err != nil {
			_ = os.RemoveAll(args.outputDir)
			return err
		}

		info := map[string]string{"secret": secret}

		cliPrint, err := json.Marshal(info)
		if err != nil {
			return err
		}

		// save private key seed words
		if err := evmtestutil.WriteFile(fmt.Sprintf("%v.json", "key_seed"), nodeDir, cliPrint); err != nil {
			return err
		}

		accStakingTokens := sdk.TokensFromConsensusPower(int64(args.initialBalances), sdk.DefaultPowerReduction)
		coins := sdk.Coins{
			sdk.NewCoin(sdk.DefaultBondDenom, accStakingTokens),
		}

		genBalances = append(genBalances, banktypes.Balance{Address: addr.String(), Coins: coins.Sort()})
		genAccounts = append(genAccounts, authtypes.NewBaseAccount(addr, nil, 0, 0))

		valStr, err := valAddrCodec.BytesToString(sdk.ValAddress(addr))
		if err != nil {
			return err
		}
		valTokens := sdk.TokensFromConsensusPower(int64(args.initialStaking), sdk.DefaultPowerReduction)
		rate, _ := sdkmath.LegacyNewDecFromStr("0.1")
		maxRate, _ := sdkmath.LegacyNewDecFromStr("0.2")
		maxChangeRate, _ := sdkmath.LegacyNewDecFromStr("0.01")
		createValMsg, err := stakingtypes.NewMsgCreateValidator(
			valStr,
			valPubKeys[i],
			sdk.NewCoin(sdk.DefaultBondDenom, valTokens),
			stakingtypes.NewDescription(nodeDirName, "", "", "", ""),
			stakingtypes.NewCommissionRates(rate, maxRate, maxChangeRate),
			sdkmath.OneInt(),
		)
		if err != nil {
			return err
		}

		txBuilder := clientCtx.TxConfig.NewTxBuilder()
		if err := txBuilder.SetMsgs(createValMsg); err != nil {
			return err
		}

		if args.baseFee > args.minGasPrice {
			args.minGasPrice = args.baseFee
		}
		txBuilder.SetMemo(memo)
		txBuilder.SetGasLimit(createValidatorMsgGasLimit)
		txBuilder.SetFeeAmount(sdk.NewCoins(sdk.NewCoin(app.BaseDenom, sdkmath.NewInt(int64(args.minGasPrice*createValidatorMsgGasLimit)))))
		txBuilder.SetTimeoutHeight(0)

		txFactory := tx.Factory{}
		txFactory = txFactory.
			WithChainID(args.chainID).
			WithMemo(memo).
			WithKeybase(kb).
			WithTxConfig(clientCtx.TxConfig)

		if err := tx.Sign(cmd.Context(), txFactory, nodeDirName, txBuilder, true); err != nil {
			return err
		}

		txBz, err := clientCtx.TxConfig.TxJSONEncoder()(txBuilder.GetTx())
		if err != nil {
			return err
		}

		if err := evmtestutil.WriteFile(fmt.Sprintf("%v.json", nodeDirName), gentxsDir, txBz); err != nil {
			return err
		}

		customAppTemplate, customAppConfig := initAppConfig()
		customCMTConfig := initCometBFTConfig()
		srvconfig.SetConfigTemplate(customAppTemplate)
		if err := server.InterceptConfigsPreRunHandler(cmd, customAppTemplate, customAppConfig, customCMTConfig); err != nil {
			return err
		}

		srvconfig.WriteConfigFile(filepath.Join(nodeDir, "config/app.toml"), appConfig)
	}

	if err := initGenFiles(clientCtx, mbm,
		args.chainID,
		genAccounts,
		genBalances,
		genFiles,
		args.numValidators,
		args.maxGas,
		args.votingPeriod,
		args.timeoutCommit,
	); err != nil {
		return err
	}

	err := collectGenFiles(
		clientCtx, nodeConfig, args.chainID, nodeIDs, valPubKeys, args.numValidators,
		args.outputDir, args.nodeDirPrefix, args.nodeDaemonHome, genBalIterator, valAddrCodec,
		rpcPort, p2pPortStart, args.singleHost, args.maxGas,
	)
	if err != nil {
		return err
	}

	cmd.PrintErrf("Successfully initialized %d node directories\n", args.numValidators)
	return nil
}

func initGenFiles(
	clientCtx client.Context,
	mbm module.BasicManager,
	chainID string,
	genAccounts []authtypes.GenesisAccount,
	genBalances []banktypes.Balance,
	genFiles []string,
	numValidators int,
	maxGas int,
	votingPeriod time.Duration,
	timeoutCommit time.Duration,
) error {
	appGenState := mbm.DefaultGenesis(clientCtx.Codec)

	// set the accounts in the genesis state
	var authGenState authtypes.GenesisState
	clientCtx.Codec.MustUnmarshalJSON(appGenState[authtypes.ModuleName], &authGenState)
	accounts, err := authtypes.PackAccounts(genAccounts)
	if err != nil {
		return err
	}
	authGenState.Accounts = accounts
	appGenState[authtypes.ModuleName] = clientCtx.Codec.MustMarshalJSON(&authGenState)

	// set the balances in the genesis state
	var bankGenState banktypes.GenesisState
	clientCtx.Codec.MustUnmarshalJSON(appGenState[banktypes.ModuleName], &bankGenState)
	bankGenState.Balances = banktypes.SanitizeGenesisBalances(genBalances)
	for _, bal := range bankGenState.Balances {
		bankGenState.Supply = bankGenState.Supply.Add(bal.Coins...)
	}
	appGenState[banktypes.ModuleName] = clientCtx.Codec.MustMarshalJSON(&bankGenState)

	// set evm state
	evmGenState := evmd.NewEVMGenesisState()
	evmGenState.Params.EvmDenom = sdk.DefaultBondDenom
	evmGenState.Params.ExtraEIPs = append(evmGenState.Params.ExtraEIPs, 3855)
	evmGenState.Params.AllowUnprotectedTxs = true
	evmGenState.Params.ChainConfig = *evmvmtypes.DefaultChainConfig(chainID)
	evmGenState.Params.ChainConfig.Denom = app.BaseDenom
	evmGenState.Params.ChainConfig.Decimals = app.BaseDenomUnit
	appGenState[evmvmtypes.ModuleName] = clientCtx.Codec.MustMarshalJSON(evmGenState)

	// set erc20 state
	erc20GenState := evmerc20types.DefaultGenesisState()
	utacERC20 := "0xD4949664cD82660AaE99bEdc034a0deA8A0bd517"
	erc20GenState.Params.NativePrecompiles = append(erc20GenState.Params.NativePrecompiles, utacERC20)
	erc20GenState.Params.DynamicPrecompiles = []string{}
	// by default erc20 TokenPair.ContractOwner enum is marshalled as string, but is expected as int
	// properly marshaling TokenPairs with ContractOwner as an integer instead of a string
	erc20GenStateJSON, err := json.Marshal(struct {
		TokenPairs []TokenPairForMarshal `json:"token_pairs"`
		Params     evmerc20types.Params  `json:"params"`
	}{
		TokenPairs: []TokenPairForMarshal{
			{
				Erc20Address:  utacERC20,
				Denom:         app.BaseDenom,
				Enabled:       true,
				ContractOwner: int(evmerc20types.OWNER_MODULE),
			},
		},
		Params: erc20GenState.Params,
	})
	if err != nil {
		return err
	}

	appGenState[evmerc20types.ModuleName] = erc20GenStateJSON

	// set voting period
	govGenState := govv1.DefaultGenesisState()
	govGenState.Params.MaxDepositPeriod = &votingPeriod
	govGenState.Params.VotingPeriod = &votingPeriod
	expeditedPeriod := votingPeriod / 2
	govGenState.Params.ExpeditedVotingPeriod = &expeditedPeriod
	appGenState[govtypes.ModuleName] = clientCtx.Codec.MustMarshalJSON(govGenState)

	// set blocks per year
	var mintGenState minttypes.GenesisState
	clientCtx.Codec.MustUnmarshalJSON(appGenState[minttypes.ModuleName], &mintGenState)
	mintGenState.Params.BlocksPerYear = uint64((365 * 24 * 60 * 60) / timeoutCommit.Seconds())
	appGenState[minttypes.ModuleName] = clientCtx.Codec.MustMarshalJSON(&mintGenState)

	appGenStateJSON, err := json.MarshalIndent(appGenState, "", "  ")
	if err != nil {
		return err
	}

	// set max gas
	consensusParams := types.DefaultConsensusParams()
	consensusParams.Block.MaxGas = int64(maxGas)

	// create app genesis
	appGenesis := genutiltypes.NewAppGenesisWithVersion(chainID, appGenStateJSON)
	appGenesis.Consensus.Params = consensusParams

	// generate empty genesis files for each validator and save
	for i := 0; i < numValidators; i++ {
		if err := appGenesis.SaveAs(genFiles[i]); err != nil {
			return err
		}
	}
	return nil
}

func collectGenFiles(
	clientCtx client.Context, nodeConfig *cmtconfig.Config, chainID string,
	nodeIDs []string, valPubKeys []cryptotypes.PubKey, numValidators int,
	outputDir, nodeDirPrefix, nodeDaemonHome string, genBalIterator banktypes.GenesisBalancesIterator, valAddrCodec runtime.ValidatorAddressCodec,
	rpcPortStart, p2pPortStart int,
	singleHost bool,
	maxGas int,
) error {
	var appState json.RawMessage
	genTime := cmttime.Now()

	for i := 0; i < numValidators; i++ {
		var portOffset int
		if singleHost {
			portOffset = i
		}

		nodeDirName := fmt.Sprintf("%s%d", nodeDirPrefix, i)
		nodeDir := filepath.Join(outputDir, nodeDirName, nodeDaemonHome)
		gentxsDir := filepath.Join(outputDir, "gentxs")
		nodeConfig.Moniker = nodeDirName
		nodeConfig.RPC.ListenAddress = fmt.Sprintf("tcp://localhost:%d", rpcPortStart+portOffset)
		nodeConfig.P2P.ListenAddress = fmt.Sprintf("tcp://localhost:%d", p2pPortStart+portOffset)

		nodeConfig.SetRoot(nodeDir)

		nodeID, valPubKey := nodeIDs[i], valPubKeys[i]
		initCfg := genutiltypes.NewInitConfig(chainID, gentxsDir, nodeID, valPubKey)

		appGenesis, err := genutiltypes.AppGenesisFromFile(nodeConfig.GenesisFile())
		if err != nil {
			return err
		}

		nodeAppState, err := genutil.GenAppStateFromConfig(clientCtx.Codec, clientCtx.TxConfig, nodeConfig, initCfg, appGenesis, genBalIterator, genutiltypes.DefaultMessageValidator,
			valAddrCodec)
		if err != nil {
			return err
		}

		if appState == nil {
			// set the canonical application state (they should not differ)
			appState = nodeAppState
		}

		// overwrite each validator's genesis file to have a canonical genesis time
		appGenesis = genutiltypes.NewAppGenesisWithVersion(chainID, appState)
		appGenesis.GenesisTime = genTime
		appGenesis.Consensus = &genutiltypes.ConsensusGenesis{
			Validators: nil,
			Params:     types.DefaultConsensusParams(),
		}
		appGenesis.Consensus.Params.Block.MaxGas = int64(maxGas)
		if err := appGenesis.ValidateAndComplete(); err != nil {
			return err
		}

		if err := appGenesis.SaveAs(nodeConfig.GenesisFile()); err != nil {
			return err
		}
	}

	return nil
}

func getIP(i int, startingIPAddr string) (ip string, err error) {
	if len(startingIPAddr) == 0 {
		ip, err = server.ExternalIP()
		if err != nil {
			return "", err
		}
		return ip, nil
	}
	return calculateIP(startingIPAddr, i)
}

func calculateIP(ip string, i int) (string, error) {
	ipv4 := net.ParseIP(ip).To4()
	if ipv4 == nil {
		return "", fmt.Errorf("%v: non ipv4 address", ip)
	}

	for j := 0; j < i; j++ {
		ipv4[3]++
	}

	return ipv4.String(), nil
}

package ledger_test

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"cosmossdk.io/log"
	"github.com/Asphere-xyz/tacchain/app"
	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/suite"

	//nolint:revive // dot imports are fine for Ginkgo
	. "github.com/onsi/ginkgo/v2"
	//nolint:revive // dot imports are fine for Ginkgo
	. "github.com/onsi/gomega"

	"github.com/cometbft/cometbft/crypto/tmhash"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	tmversion "github.com/cometbft/cometbft/proto/tendermint/version"
	rpcclientmock "github.com/cometbft/cometbft/rpc/client/mock"
	"github.com/cometbft/cometbft/version"

	evmclientkeys "github.com/cosmos/evm/client/keys"
	evmhd "github.com/cosmos/evm/crypto/hd"
	evmcosmoskeyring "github.com/cosmos/evm/crypto/keyring"

	evmledgermocks "github.com/cosmos/evm/tests/integration/ledger/mocks"
	evmconstants "github.com/cosmos/evm/testutil/constants"
	evmutiltx "github.com/cosmos/evm/testutil/tx"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/keys"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	cosmosledger "github.com/cosmos/cosmos-sdk/crypto/ledger"
	"github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/server"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdktestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
)

var s *LedgerTestSuite

type LedgerTestSuite struct {
	suite.Suite

	app *app.TacChainApp
	ctx sdk.Context

	ledger       *evmledgermocks.SECP256K1
	accRetriever *evmledgermocks.AccountRetriever

	accAddr sdk.AccAddress

	privKey types.PrivKey
	pubKey  types.PubKey
}

func TestLedger(t *testing.T) {
	s = new(LedgerTestSuite)
	suite.Run(t, s)

	RegisterFailHandler(Fail)
	RunSpecs(t, "Tacchaind Suite")
}

func (suite *LedgerTestSuite) SetupTest() {
	var (
		err     error
		ethAddr common.Address
	)

	suite.ledger = evmledgermocks.NewSECP256K1(s.T())

	ethAddr, s.privKey = evmutiltx.NewAddrKey()

	s.Require().NoError(err)
	suite.pubKey = s.privKey.PubKey()

	suite.accAddr = sdk.AccAddress(ethAddr.Bytes())
}

func (suite *LedgerTestSuite) SetupTacchainApp() {
	consAddress := sdk.ConsAddress(evmutiltx.GenerateAddress().Bytes())

	// init app
	chainID := app.DefaultChainID
	suite.app = app.NewTacChainAppWithCustomOptions(suite.T(), false, 0, app.SetupOptions{
		Logger:  log.NewTestLogger(suite.T()),
		DB:      dbm.NewMemDB(),
		AppOpts: simtestutil.NewAppOptionsWithFlagHome(suite.T().TempDir()),
	})
	suite.ctx = suite.app.BaseApp.NewContextLegacy(false, tmproto.Header{
		Height:          1,
		ChainID:         chainID,
		Time:            time.Now().UTC(),
		ProposerAddress: consAddress.Bytes(),

		Version: tmversion.Consensus{
			Block: version.BlockProtocol,
		},
		LastBlockId: tmproto.BlockID{
			Hash: tmhash.Sum([]byte("block_id")),
			PartSetHeader: tmproto.PartSetHeader{
				Total: 11,
				Hash:  tmhash.Sum([]byte("partset_header")),
			},
		},
		AppHash:            tmhash.Sum([]byte("app")),
		DataHash:           tmhash.Sum([]byte("data")),
		EvidenceHash:       tmhash.Sum([]byte("evidence")),
		ValidatorsHash:     tmhash.Sum([]byte("validators")),
		NextValidatorsHash: tmhash.Sum([]byte("next_validators")),
		ConsensusHash:      tmhash.Sum([]byte("consensus")),
		LastResultsHash:    tmhash.Sum([]byte("last_result")),
	})
}

func (suite *LedgerTestSuite) NewKeyringAndCtxs(krHome string, input io.Reader, encCfg sdktestutil.TestEncodingConfig) (keyring.Keyring, client.Context, context.Context) {
	kr, err := keyring.New(
		sdk.KeyringServiceName(),
		keyring.BackendTest,
		krHome,
		input,
		encCfg.Codec,
		s.MockKeyringOption(),
	)
	s.Require().NoError(err)
	s.accRetriever = evmledgermocks.NewAccountRetriever(s.T())

	initClientCtx := client.Context{}.
		WithCodec(encCfg.Codec).
		// NOTE: cmd.Execute() panics without account retriever
		WithAccountRetriever(s.accRetriever).
		WithTxConfig(encCfg.TxConfig).
		WithLedgerHasProtobuf(true).
		WithUseLedger(true).
		WithKeyring(kr).
		WithClient(evmledgermocks.MockCometRPC{Client: rpcclientmock.Client{}}).
		WithChainID(evmconstants.ExampleChainIDPrefix + "-13").
		WithSignModeStr(flags.SignModeLegacyAminoJSON)

	srvCtx := server.NewDefaultContext()
	ctx := context.Background()
	ctx = context.WithValue(ctx, client.ClientContextKey, &initClientCtx)
	ctx = context.WithValue(ctx, server.ServerContextKey, srvCtx)

	return kr, initClientCtx, ctx
}

func (suite *LedgerTestSuite) cosmosEVMAddKeyCmd() *cobra.Command {
	cmd := keys.AddKeyCommand()

	algoFlag := cmd.Flag(flags.FlagKeyType)
	algoFlag.DefValue = string(evmhd.EthSecp256k1Type)

	err := algoFlag.Value.Set(string(evmhd.EthSecp256k1Type))
	suite.Require().NoError(err)

	cmd.Flags().AddFlagSet(keys.Commands().PersistentFlags())

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		clientCtx := client.GetClientContextFromCmd(cmd).WithKeyringOptions(evmhd.EthSecp256k1Option())
		clientCtx, err := client.ReadPersistentCommandFlags(clientCtx, cmd.Flags())
		if err != nil {
			return err
		}
		buf := bufio.NewReader(clientCtx.Input)
		return evmclientkeys.RunAddCmd(clientCtx, cmd, args, buf)
	}
	return cmd
}

func (suite *LedgerTestSuite) MockKeyringOption() keyring.Option {
	return func(options *keyring.Options) {
		options.SupportedAlgos = evmcosmoskeyring.SupportedAlgorithms
		options.SupportedAlgosLedger = evmcosmoskeyring.SupportedAlgorithmsLedger
		options.LedgerDerivation = func() (cosmosledger.SECP256K1, error) { return suite.ledger, nil }
		options.LedgerCreateKey = evmcosmoskeyring.CreatePubkey
		options.LedgerAppName = evmcosmoskeyring.AppName
		options.LedgerSigSkipDERConv = evmcosmoskeyring.SkipDERConversion
	}
}

func (suite *LedgerTestSuite) FormatFlag(flag string) string {
	return fmt.Sprintf("--%s", flag)
}

package ledger_test

import (
	"bytes"
	"context"

	"github.com/spf13/cobra"

	//nolint:revive // dot imports are fine for Ginkgo
	. "github.com/onsi/ginkgo/v2"

	evmhd "github.com/cosmos/evm/crypto/hd"
	evmencoding "github.com/cosmos/evm/encoding"
	evmledgermocks "github.com/cosmos/evm/tests/integration/ledger/mocks"
	evmtestutil "github.com/cosmos/evm/testutil"
	evmutiltx "github.com/cosmos/evm/testutil/tx"

	"cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdktestutil "github.com/cosmos/cosmos-sdk/testutil"
	sdktestutilcli "github.com/cosmos/cosmos-sdk/testutil/cli"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdktestutilmod "github.com/cosmos/cosmos-sdk/types/module/testutil"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	bankcli "github.com/cosmos/cosmos-sdk/x/bank/client/cli"
)

var (
	signOkMock = func(_ []uint32, msg []byte) ([]byte, error) {
		return s.privKey.Sign(msg)
	}

	signErrMock = func([]uint32, []byte) ([]byte, error) {
		return nil, evmledgermocks.ErrMockedSigning
	}
)

var _ = Describe("Ledger CLI and keyring functionality: ", func() {
	var (
		receiverAccAddr sdk.AccAddress
		encCfg          sdktestutilmod.TestEncodingConfig
		kr              keyring.Keyring
		mockedIn        sdktestutil.BufferReader
		clientCtx       client.Context
		ctx             context.Context
		cmd             *cobra.Command
		krHome          string
		keyRecord       *keyring.Record
		baseDenom       string
	)

	ledgerKey := "ledger_key"

	s.SetupTest()
	s.SetupTacchainApp()

	Describe("Adding a key from ledger using the CLI", func() {
		BeforeEach(func() {
			krHome = s.T().TempDir()
			encCfg = evmencoding.MakeConfig()

			cmd = s.cosmosEVMAddKeyCmd()

			mockedIn = sdktestutil.ApplyMockIODiscardOutErr(cmd)

			kr, clientCtx, ctx = s.NewKeyringAndCtxs(krHome, mockedIn, encCfg)

			evmledgermocks.MClose(s.ledger)
			evmledgermocks.MGetAddressPubKeySECP256K1(s.ledger, s.accAddr, s.pubKey)

			var err error
			baseDenom, err = sdk.GetBaseDenom()
			s.Require().NoError(err)
		})
		Context("with default algo", func() {
			It("should use eth_secp256k1 by default and pass", func() {
				out, err := sdktestutilcli.ExecTestCLICmd(clientCtx, cmd, []string{
					ledgerKey,
					s.FormatFlag(flags.FlagUseLedger),
				})

				s.Require().NoError(err)
				s.Require().Contains(out.String(), "name: ledger_key")

				_, err = kr.Key(ledgerKey)
				s.Require().NoError(err, "can't find ledger key")
			})
		})
		Context("with eth_secp256k1 algo", func() {
			It("should add the ledger key ", func() {
				out, err := sdktestutilcli.ExecTestCLICmd(clientCtx, cmd, []string{
					ledgerKey,
					s.FormatFlag(flags.FlagUseLedger),
					s.FormatFlag(flags.FlagKeyType),
					string(evmhd.EthSecp256k1Type),
				})

				s.Require().NoError(err)
				s.Require().Contains(out.String(), "name: ledger_key")

				_, err = kr.Key(ledgerKey)
				s.Require().NoError(err, "can't find ledger key")
			})
		})
	})
	Describe("Singing a transactions", func() {
		BeforeEach(func() {
			krHome = s.T().TempDir()
			encCfg = evmencoding.MakeConfig()

			var err error

			// create add key command
			cmd = s.cosmosEVMAddKeyCmd()

			mockedIn = sdktestutil.ApplyMockIODiscardOutErr(cmd)
			evmledgermocks.MGetAddressPubKeySECP256K1(s.ledger, s.accAddr, s.pubKey)

			kr, clientCtx, ctx = s.NewKeyringAndCtxs(krHome, mockedIn, encCfg)

			b := bytes.NewBufferString("")
			cmd.SetOut(b)

			cmd.SetArgs([]string{
				ledgerKey,
				s.FormatFlag(flags.FlagUseLedger),
				s.FormatFlag(flags.FlagKeyType),
				"eth_secp256k1",
			})
			// add ledger key for following tests
			s.Require().NoError(cmd.ExecuteContext(ctx))
			keyRecord, err = kr.Key(ledgerKey)
			s.Require().NoError(err, "can't find ledger key")
		})
		Context("perform bank send", func() {
			Context("with keyring functions calling", func() {
				BeforeEach(func() {
					s.ledger = evmledgermocks.NewSECP256K1(s.T())

					evmledgermocks.MClose(s.ledger)
					evmledgermocks.MGetPublicKeySECP256K1(s.ledger, s.pubKey)
				})
				It("should return valid signature", func() {
					evmledgermocks.MSignSECP256K1(s.ledger, signOkMock, nil)

					ledgerAddr, err := keyRecord.GetAddress()
					s.Require().NoError(err, "can't retirieve ledger addr from a keyring")

					msg := []byte("test message")

					signed, _, err := kr.SignByAddress(ledgerAddr, msg, signingtypes.SignMode_SIGN_MODE_TEXTUAL)
					s.Require().NoError(err, "failed to sign message")

					valid := s.pubKey.VerifySignature(msg, signed)
					s.Require().True(valid, "invalid signature returned")
				})
				It("should raise error from ledger sign function to the top", func() {
					evmledgermocks.MSignSECP256K1(s.ledger, signErrMock, evmledgermocks.ErrMockedSigning)

					ledgerAddr, err := keyRecord.GetAddress()
					s.Require().NoError(err, "can't retirieve ledger addr from a keyring")

					msg := []byte("test message")

					_, _, err = kr.SignByAddress(ledgerAddr, msg, signingtypes.SignMode_SIGN_MODE_TEXTUAL)

					s.Require().Error(err, "false positive result, error expected")

					s.Require().Equal(evmledgermocks.ErrMockedSigning.Error(), err.Error(), "original and returned errors are not equal")
				})
			})
			Context("with cli command", func() {
				BeforeEach(func() {
					s.ledger = evmledgermocks.NewSECP256K1(s.T())

					err := evmtestutil.FundAccount(
						s.ctx,
						s.app.BankKeeper,
						s.accAddr,
						sdk.NewCoins(
							sdk.NewCoin(baseDenom, math.NewInt(100000000000000)),
						),
					)
					s.Require().NoError(err)

					receiverAccAddr = sdk.AccAddress(evmutiltx.GenerateAddress().Bytes())

					cmd = bankcli.NewSendTxCmd(s.app.AccountKeeper.AddressCodec())
					mockedIn = sdktestutil.ApplyMockIODiscardOutErr(cmd)

					kr, clientCtx, ctx = s.NewKeyringAndCtxs(krHome, mockedIn, encCfg)

					// register mocked funcs
					evmledgermocks.MClose(s.ledger)
					evmledgermocks.MGetPublicKeySECP256K1(s.ledger, s.pubKey)
					evmledgermocks.MEnsureExist(s.accRetriever, nil)
					evmledgermocks.MGetAccountNumberSequence(s.accRetriever, 0, 0, nil)
				})
				It("should execute bank tx cmd", func() {
					evmledgermocks.MSignSECP256K1(s.ledger, signOkMock, nil)

					cmd.SetContext(ctx)
					cmd.SetArgs([]string{
						ledgerKey,
						receiverAccAddr.String(),
						sdk.NewCoin(baseDenom, math.NewInt(1000)).String(),
						s.FormatFlag(flags.FlagUseLedger),
						s.FormatFlag(flags.FlagSkipConfirmation),
					})
					out := bytes.NewBufferString("")
					cmd.SetOutput(out)

					err := cmd.Execute()

					s.Require().NoError(err, "can't execute cli tx command")
				})
				It("should return error from ledger device", func() {
					evmledgermocks.MSignSECP256K1(s.ledger, signErrMock, evmledgermocks.ErrMockedSigning)

					cmd.SetContext(ctx)
					cmd.SetArgs([]string{
						ledgerKey,
						receiverAccAddr.String(),
						sdk.NewCoin(baseDenom, math.NewInt(1000)).String(),
						s.FormatFlag(flags.FlagUseLedger),
						s.FormatFlag(flags.FlagSkipConfirmation),
					})
					out := bytes.NewBufferString("")
					cmd.SetOutput(out)

					err := cmd.Execute()

					s.Require().Error(err, "false positive, error expected")
					s.Require().Equal(evmledgermocks.ErrMockedSigning.Error(), err.Error())
				})
			})
		})
	})
})

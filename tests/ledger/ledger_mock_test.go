//go:build ledger && test_ledger_mock

package ledger_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	appconfig "github.com/TacBuild/tacchain/app/config"

	"github.com/cosmos/evm/crypto/ethsecp256k1"
	evmhd "github.com/cosmos/evm/crypto/hd"
	evmkeyring "github.com/cosmos/evm/crypto/keyring"
	evmencoding "github.com/cosmos/evm/encoding"

	sdkhd "github.com/cosmos/cosmos-sdk/crypto/hd"
	sdkkeyring "github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdkledger "github.com/cosmos/cosmos-sdk/crypto/ledger"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/testutil/testdata"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
)

func TestEVMKeyringCreatesLedgerKeyWithTACConfig(t *testing.T) {
	kr := newLedgerKeyring(t)

	record, err := kr.SaveLedgerKey("ledger", evmhd.EthSecp256k1, appconfig.Bech32PrefixAccAddr, evmhd.Bip44CoinType, 0, 0)
	require.NoError(t, err)
	require.Equal(t, sdkkeyring.TypeLedger, record.GetType())

	pubKey, err := record.GetPubKey()
	require.NoError(t, err)
	require.IsType(t, &ethsecp256k1.PubKey{}, pubKey)
	require.Equal(t, ethsecp256k1.KeyType, pubKey.Type())

	address, err := record.GetAddress()
	require.NoError(t, err)
	require.Len(t, address.Bytes(), 20)
	require.True(t, strings.HasPrefix(address.String(), appconfig.Bech32PrefixAccAddr+"1"))

	path := record.GetLedger().GetPath()
	require.Equal(t, "m/44'/60'/0'/0/0", path.String())
	require.Equal(t, sdk.GetConfig().GetCoinType(), path.CoinType)

	restored, err := kr.Key("ledger")
	require.NoError(t, err)
	restoredPubKey, err := restored.GetPubKey()
	require.NoError(t, err)
	require.True(t, pubKey.Equals(restoredPubKey))
	require.Equal(t, path.String(), restored.GetLedger().GetPath().String())
}

func TestEVMKeyringSignsWithLedgerMock(t *testing.T) {
	kr := newLedgerKeyring(t)
	record, err := kr.SaveLedgerKey("ledger", evmhd.EthSecp256k1, appconfig.Bech32PrefixAccAddr, evmhd.Bip44CoinType, 0, 0)
	require.NoError(t, err)

	msg := []byte("ledger mock signs TAC EVM transactions")
	sig, pubKey, err := kr.Sign("ledger", msg, signing.SignMode_SIGN_MODE_LEGACY_AMINO_JSON)
	require.NoError(t, err)
	require.Len(t, sig, crypto.SignatureLength)
	require.True(t, pubKey.VerifySignature(msg, sig))

	recordPubKey, err := record.GetPubKey()
	require.NoError(t, err)
	require.True(t, recordPubKey.Equals(pubKey))

	_, _, err = sdkkeyring.SignWithLedger(record, msg, signing.SignMode_SIGN_MODE_DIRECT)
	require.ErrorIs(t, err, sdkkeyring.ErrInvalidSignMode)
}

func TestEVMKeyringLedgerMockRejectsWrongCoinType(t *testing.T) {
	kr := newLedgerKeyring(t)

	_, err := kr.SaveLedgerKey("wrong-coin-type", evmhd.EthSecp256k1, appconfig.Bech32PrefixAccAddr, sdk.CoinType, 0, 0)
	require.Error(t, err)
	require.ErrorIs(t, err, sdkkeyring.ErrLedgerGenerateKey)
	require.Contains(t, fmt.Sprintf("%+v", err), "invalid derivation path")
}

func newLedgerKeyring(t *testing.T) sdkkeyring.Keyring {
	t.Helper()

	appconfig.SetupSDKConfig()
	encodingConfig := evmencoding.MakeConfig(0)

	kr := sdkkeyring.NewInMemory(encodingConfig.Codec, evmkeyring.Option(), useEVMFriendlyLedgerMock)
	t.Cleanup(func() {
		// Keep later tests in this process on the same public EVM keyring settings,
		// without leaving this package's mock device installed globally.
		sdkkeyring.NewInMemory(encodingConfig.Codec, evmkeyring.Option())
	})

	return kr
}

func useEVMFriendlyLedgerMock(options *sdkkeyring.Options) {
	options.LedgerDerivation = func() (sdkledger.SECP256K1, error) {
		return evmFriendlyLedgerMock{}, nil
	}
}

type evmFriendlyLedgerMock struct{}

func (evmFriendlyLedgerMock) Close() error {
	return nil
}

func (evmFriendlyLedgerMock) GetPublicKeySECP256K1(derivationPath []uint32) ([]byte, error) {
	privKey, err := deriveEVMPrivKey(derivationPath)
	if err != nil {
		return nil, err
	}

	ecdsaPrivKey, err := privKey.ToECDSA()
	if err != nil {
		return nil, err
	}

	return crypto.FromECDSAPub(&ecdsaPrivKey.PublicKey), nil
}

func (mock evmFriendlyLedgerMock) GetAddressPubKeySECP256K1(derivationPath []uint32, hrp string) ([]byte, string, error) {
	publicKey, err := mock.GetPublicKeySECP256K1(derivationPath)
	if err != nil {
		return nil, "", err
	}

	privKey, err := deriveEVMPrivKey(derivationPath)
	if err != nil {
		return nil, "", err
	}

	address, err := sdk.Bech32ifyAddressBytes(hrp, []byte(privKey.PubKey().Address()))
	if err != nil {
		return nil, "", err
	}

	return publicKey, address, nil
}

func (evmFriendlyLedgerMock) SignSECP256K1(derivationPath []uint32, message []byte, _ byte) ([]byte, error) {
	privKey, err := deriveEVMPrivKey(derivationPath)
	if err != nil {
		return nil, err
	}

	ecdsaPrivKey, err := privKey.ToECDSA()
	if err != nil {
		return nil, err
	}

	return crypto.Sign(crypto.Keccak256Hash(message).Bytes(), ecdsaPrivKey)
}

func (evmFriendlyLedgerMock) ShowAddressSECP256K1([]uint32, string) error {
	return nil
}

func deriveEVMPrivKey(derivationPath []uint32) (*ethsecp256k1.PrivKey, error) {
	if len(derivationPath) != 5 {
		return nil, fmt.Errorf("invalid derivation path length: %d", len(derivationPath))
	}

	if derivationPath[0] != 44 || derivationPath[1] != sdk.GetConfig().GetCoinType() {
		return nil, errors.New("invalid derivation path")
	}

	path := sdkhd.NewParams(derivationPath[0], derivationPath[1], derivationPath[2], derivationPath[3] != 0, derivationPath[4])
	keyBytes, err := evmhd.EthSecp256k1.Derive()(testdata.TestMnemonic, "", path.String())
	if err != nil {
		return nil, err
	}

	privKey, ok := evmhd.EthSecp256k1.Generate()(keyBytes).(*ethsecp256k1.PrivKey)
	if !ok {
		return nil, fmt.Errorf("unexpected EVM private key type")
	}

	return privKey, nil
}

var _ sdkledger.SECP256K1 = evmFriendlyLedgerMock{}
var _ cryptotypes.PrivKey = (*ethsecp256k1.PrivKey)(nil)

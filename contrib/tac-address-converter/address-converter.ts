import { Buffer } from 'buffer';

import { bech32 } from 'bech32';
import {
  isValidChecksumAddress,
  stripHexPrefix,
  toChecksumAddress,
  // eslint-disable-next-line @typescript-eslint/ban-ts-comment
  // @ts-ignore
} from 'crypto-addr-codec';

function makeChecksummedHexDecoder(chainId?: number) {
  return (data: string) => {
    const stripped = stripHexPrefix(data);
    if (
      !isValidChecksumAddress(data, chainId || null) &&
      stripped !== stripped.toLowerCase() &&
      stripped !== stripped.toUpperCase()
    ) {
      throw Error('Invalid address checksum');
    }
    return Buffer.from(stripHexPrefix(data), 'hex');
  };
}

function makeChecksummedHexEncoder(chainId?: number) {
  return (data: Buffer) =>
    toChecksumAddress(data.toString('hex'), chainId || null);
}

const hexChecksumChain = (name: string, chainId?: number) => ({
  decoder: makeChecksummedHexDecoder(chainId),
  encoder: makeChecksummedHexEncoder(chainId),
  name,
});

export const ETH = hexChecksumChain('ETH');

function makeBech32Encoder(prefix: string) {
  return (data: Buffer) => bech32.encode(prefix, bech32.toWords(data));
}

function makeBech32Decoder(currentPrefix: string) {
  return (data: string) => {
    const { prefix, words } = bech32.decode(data);
    if (prefix !== currentPrefix) {
      throw Error('Unrecognised address format');
    }
    return Buffer.from(bech32.fromWords(words));
  };
}

const bech32Chain = (name: string, prefix: string) => ({
  decoder: makeBech32Decoder(prefix),
  encoder: makeBech32Encoder(prefix),
  name,
});

export const TAC = bech32Chain('TAC', 'tac');

export const ethToTac = (ethAddress: string) => {
  const data = ETH.decoder(ethAddress.toLowerCase());
  return TAC.encoder(data).toLowerCase();
};

export const tacToEth = (tacAddress: string) => {
  const data = TAC.decoder(tacAddress.toLowerCase());
  return ETH.encoder(data).toLowerCase();
};

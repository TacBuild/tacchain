import { ethToTac, tacToEth } from './address-converter';

describe('address-converter', () => {
    const ethAddress = '0x123456789abcdef0123456789abcdef012345678';
    const ethAddressMixed = '0x123456789aBcDeF0123456789ABcDef012345678';
    const tacAddress = 'tac1zg69v7y6hn00qy352euf40x77qfrg4nchk34lw';
    const tacAddressMixed = 'tac1zG69v7y6hN00qY352euF40x77qfRg4nchK34lw';

    it('should convert a valid ETH address to TAC', () => {
        const tacResult = ethToTac(ethAddress);
        expect(tacResult).toBe(tacAddress);
    });

    it('should convert TAC address to ETH', () => {
        const ethResult = tacToEth(tacAddress);
        expect(ethResult).toBe(ethAddress);
    });

    it('should convert mixed-case ETH address to TAC', () => {
        const tacResult = ethToTac(ethAddressMixed);
        expect(tacResult).toBe(tacAddress);
    });

    it('should convert mixed-case TAC address to ETH', () => {
        const ethResult = tacToEth(tacAddressMixed);
        expect(ethResult).toBe(ethAddress);
    });
});
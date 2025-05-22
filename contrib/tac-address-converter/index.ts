import { ethToTac, tacToEth } from "./address-converter";

// replace with the address you want to convert
const ADDRESS = '0x123456789aBcDeF0123456789ABcDef012345678';

// alternatively you can convert address from TAC to EVM:
// const ADDRESS = 'tac1zg69v7y6hn00qy352euf40x77qfrg4nchk34lw';

function main() {
    console.log(ethToTac(ADDRESS))

    // alternatively you can convert address from TAC to EVM:
    // console.log(tacToEth(ADDRESS))
}

main();
import { ethToTac, tacToEth } from "./address-converter";

// replace with the address you want to convert
const ADDRESS = '0x123456789aBcDeF0123456789ABcDef012345678';

// alternatively you can can convert tac address to eth:
// const ADDRESS = 'tac1zg69v7y6hn00qy352euf40x77qfrg4sl8phzz';

function main() {
    console.log(ethToTac(ADDRESS))

    // alternatively you can convert from tac to eth:
    // console.log(tacToEth(ADDRESS))
}

main();
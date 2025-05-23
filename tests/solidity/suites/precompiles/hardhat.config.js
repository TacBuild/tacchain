require("@nomicfoundation/hardhat-toolbox");

/** @type import('hardhat/config').HardhatUserConfig */
module.exports = {
  solidity: {
    compilers: [
      {
        version: "0.8.18",
      },
      // This version is required to compile the werc9 contract.
      {
        version: "0.4.22",
      },
    ],
  },
  networks: {
    cosmos: {
      url: "http://127.0.0.1:8545",
      chainId: 2391,
    },
  },
};

module.exports = {
  networks: {
    // Development network is just left as truffle's default settings
    cosmos: {
      host: '127.0.0.1', // Localhost (default: none)
      port: 8545, // Standard Ethereum port (default: none)
      network_id: '*', // Any network (default: none)
      gas: 5000000, // Gas sent with each transaction
      gasPrice: 400000000000 // 30 gwei (in wei)
    }
  },
  compilers: {
    solc: {
      version: '0.5.17'
    }
  }
}

const { expect } = require('chai')
const hre = require('hardhat')
const { execSync } = require('child_process')
const path = require('path')

describe('Staking', function () {
  it('should stake ATOM to a validator', async function () {
    const valAddr = execSync(`tacchaind keys show validator -a --bech val --home ${path.resolve(__dirname, '../../../.test-solidity')}`).toString().trim()

    const stakeAmount = hre.ethers.parseEther('0.001')

    const staking = await hre.ethers.getContractAt(
      'StakingI',
      '0x0000000000000000000000000000000000000800'
    )

    const [signer] = await hre.ethers.getSigners()
    const delegationBefore = await staking.delegation(signer, valAddr)

    const tx = await staking
      .connect(signer)
      .delegate(signer, valAddr, stakeAmount)
    await tx.wait(1)

    // Query delegation
    const delegation = await staking.delegation(signer, valAddr)
    expect(delegation.balance.amount).to.equal(
      delegationBefore.balance.amount + stakeAmount,
      'Stake amount does not match'
    )
  })
})

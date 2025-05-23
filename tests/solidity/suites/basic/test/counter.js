/* eslint-disable no-undef */

const Counter = artifacts.require('Counter')

async function expectRevert(promise) {
  try {
    await promise
  } catch (error) {
    if (error.message.indexOf('revert') === -1) {
      expect('revert').to.equal(
        error.message,
        'Wrong kind of exception received'
      )
    }
    return
  }
  expect.fail('Expected an exception but none was received')
}

contract('Counter', (accounts) => {
  const [acc] = accounts
  let counter

  beforeEach(async () => {
    counter = await Counter.new()
  })

  it('should add', async () => {
    const balance = await web3.eth.getBalance(acc)

    let count

    await counter.add({ from: acc })
    count = await counter.getCounter()
    assert.equal(count, '1', 'Counter should be 1')
    assert.notEqual(
      balance,
      await web3.eth.getBalance(acc),
      `${acc}'s balance should be different`
    )
  })

  it('should subtract', async () => {
    let count

    await counter.add()
    count = await counter.getCounter()
    assert.equal(count, '1', 'Counter should be 1')

    // Use receipt to ensure logs are emitted
    const receipt = await counter.subtract()
    count = await counter.getCounter()

    assert.equal(count, '0', 'Counter should be 0')
    assert.equal(
      receipt.logs[0].event,
      'Changed',
      "Should have emitted 'Changed' event"
    )
    assert.equal(
      receipt.logs[0].args.counter,
      '0',
      "Should have emitted 'Changed' event with counter being 0"
    )

    // Check lifecycle of events
    const contract = new web3.eth.Contract(counter.abi, counter.address)
    const allEvents = await contract.getPastEvents('allEvents', {
      fromBlock: 1,
      toBlock: 'latest'
    })
    const changedEvents = await contract.getPastEvents('Changed', {
      fromBlock: 1,
      toBlock: 'latest'
    })
    assert.equal(allEvents.length, 3)
    assert.equal(changedEvents.length, 2)

    await expectRevert(counter.subtract())
  })
})

#!/bin/bash -e

export GENESIS_ACC_ADDRESS=tac1zg69v7ys40x77y352eufp27daufrg4nckcjrx2
export HOMEDIR=./.test-localnet-params
export CHAIN_ID=tacchain_239-1

# start new multi-validator network
echo "Starting new multi-validator network with 4 nodes"
echo y | make localnet-init-multi-node > /dev/null 2>&1
make HOMEDIR=${HOMEDIR}/node0 localnet-start > /dev/null 2>&1 &
make HOMEDIR=${HOMEDIR}/node1 localnet-start > /dev/null 2>&1 &
make HOMEDIR=${HOMEDIR}/node2 localnet-start > /dev/null 2>&1 &
make HOMEDIR=${HOMEDIR}/node3 localnet-start > /dev/null 2>&1 &

# wait for network to start
echo "Waiting for network to start"
timeout=120
elapsed=0
interval=2
while ! tacchaind query block --type=height 3 --node http://localhost:45111 > /dev/null 2>&1; do
  sleep $interval
  elapsed=$((elapsed + interval))
  if [ $elapsed -ge $timeout ]; then
    echo "Failed to start network. Timeout waiting for block height 3"
    killall tacchaind
    exit 1
  fi
done
echo "Network started successfully"

# verify network has 4 active validators
echo "Verifying network has 4 active validators"
expected_active_validators="4"
active_validators=$(tacchaind q comet-validator-set --node http://localhost:45111 --output json | jq -r '.pagination .total')
if [[ "$active_validators" != "$expected_active_validators" ]]; then
  echo "Failed to verify 4 active validators"
  echo "Expected: $expected_active_validators"
  echo "Got:      $active_validators"
  
  killall tacchaind
  exit 1
else
  echo "Verified 4 active validators successfully"
fi

# verify token distribution
echo "Verifying token distribution"

# verify genesis account balance
echo "Verifying genesis account balance"
expected_genesis_acc_balance="9980000000000000000000000000"
genesis_acc_balance=$(tacchaind q bank balances $GENESIS_ACC_ADDRESS --node http://localhost:45111 --output json | jq -r '.balances[0].amount')
if [[ "$genesis_acc_balance" != "$expected_genesis_acc_balance" ]]; then
  echo "Failed to verify genesis account balance"
  echo "Expected: $expected_genesis_acc_balance"
  echo "Got:      $genesis_acc_balance"
  
  killall tacchaind
  exit 1
else
  echo "Verified genesis account balance successfully"
fi

# verify validators emergency balances, self delegations and description
echo "Verifying validators emergency balances and self delegations"
expected_validator_emergency_balance="99995000000000000000"
expected_validator_self_delegation="4999900000000000000000000"
for i in $(seq 0 3); do
  echo "Verifying validator $i emergency balance"
  tac_addr=$(tacchaind keys show validator --home ./.test-localnet-params/node$i -a)
  balance=$(tacchaind q bank balances $tac_addr --node http://localhost:45111 --output json | jq -r '.balances[0].amount')
  if [[ "$balance" != "$expected_validator_emergency_balance" ]]; then
    echo "Failed to verify validator $i emergency balance"
    echo "Expected: $expected_validator_emergency_balance"
    echo "Got:      $balance"
    killall tacchaind
    exit 1
  else
    echo "Verified validator $i emergency balance successfully"
  fi

  echo "Verifying validator $i self delegation"
  valoper_addr=$(tacchaind keys show validator --home ./.test-localnet-params/node$i -a --bech val)
  self_delegation=$(tacchaind q staking validator $valoper_addr --node http://localhost:45111 --output json | jq -r '.validator .tokens')
  if [[ "$self_delegation" != "$expected_validator_self_delegation" ]]; then
    echo "Failed to verify validator $i self delegation"
    echo "Expected: $expected_validator_self_delegation"
    echo "Got:      $self_delegation"
    killall tacchaind
    exit 1
  else
    echo "Verified validator $i self delegation successfully"
  fi

  echo "Verifying validator $i description"
  expected_description="{
  \"moniker\": \"TAC Validator $((i + 1))\",
  \"identity\": \"TAC\",
  \"website\": \"https://tac.build/\"
}"
  description=$(tacchaind q staking validator $valoper_addr --node http://localhost:45111 --output json | jq -r '.validator .description')
  if [[ "$description" != "$expected_description" ]]; then
    echo "Failed to verify validator $i description"
    echo "Expected: $expected_description"
    echo "Got:      $description"
    killall tacchaind
    exit 1
  else
    echo "Verified validator $i description successfully"
  fi
done

# verify predeployed contracts
echo "Verifying predeployed contracts"

# verify safe singleton contract
echo "Verifying safe singleton contract"
addr="0x914d7fec6aac8cd542e72bca78b30650d45643d7"
expected="0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe03601600081602082378035828234f58015156039578182fd5b8082525050506014600cf3"
echo -n "  Contract $addr: "
code=$(curl -s -X POST -H "Content-Type: application/json" --data "{\"jsonrpc\":\"2.0\",\"method\":\"eth_getCode\",\"params\":[\"$addr\",\"latest\"],\"id\":1}" http://localhost:45128 | jq -r '.result')
if [[ "$code" != "$expected" ]]; then
  echo "Failed to verify safe singleton contract"
  echo "Expected: $expected"
  echo "Got:      $code"
  
  killall tacchaind
  exit 1
else
  echo "Verified safe singleton contract successfully"
fi

# verify arachnid contract
echo "Verifying arachnid contract"
addr="0x4e59b44847b379578588920ca78fbf26c0b4956c"
expected="0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe03601600081602082378035828234f58015156039578182fd5b8082525050506014600cf3"
echo -n "  Contract $addr: "
code=$(curl -s -X POST -H "Content-Type: application/json" --data "{\"jsonrpc\":\"2.0\",\"method\":\"eth_getCode\",\"params\":[\"$addr\",\"latest\"],\"id\":1}" http://localhost:45128 | jq -r '.result')
if [[ "$code" != "$expected" ]]; then
  echo "Failed to verify arachnid contract"
  echo "Expected: $expected"
  echo "Got:      $code"
  
  killall tacchaind
  exit 1
else
  echo "Verified arachnid contract successfully"
fi

# verify multicall contract
echo "Verifying multicall contract"
addr="0xca11bde05977b3631167028862be2a173976ca11"
expected="0x6080604052600436106100f35760003560e01c80634d2301cc1161008a578063a8b0574e11610059578063a8b0574e1461025a578063bce38bd714610275578063c3077fa914610288578063ee82ac5e1461029b57600080fd5b80634d2301cc146101ec57806372425d9d1461022157806382ad56cb1461023457806386d516e81461024757600080fd5b80633408e470116100c65780633408e47014610191578063399542e9146101a45780633e64a696146101c657806342cbb15c146101d957600080fd5b80630f28c97d146100f8578063174dea711461011a578063252dba421461013a57806327e86d6e1461015b575b600080fd5b34801561010457600080fd5b50425b6040519081526020015b60405180910390f35b61012d610128366004610a85565b6102ba565b6040516101119190610bbe565b61014d610148366004610a85565b6104ef565b604051610111929190610bd8565b34801561016757600080fd5b50437fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff0140610107565b34801561019d57600080fd5b5046610107565b6101b76101b2366004610c60565b610690565b60405161011193929190610cba565b3480156101d257600080fd5b5048610107565b3480156101e557600080fd5b5043610107565b3480156101f857600080fd5b50610107610207366004610ce2565b73ffffffffffffffffffffffffffffffffffffffff163190565b34801561022d57600080fd5b5044610107565b61012d610242366004610a85565b6106ab565b34801561025357600080fd5b5045610107565b34801561026657600080fd5b50604051418152602001610111565b61012d610283366004610c60565b61085a565b6101b7610296366004610a85565b610a1a565b3480156102a757600080fd5b506101076102b6366004610d18565b4090565b60606000828067ffffffffffffffff8111156102d8576102d8610d31565b60405190808252806020026020018201604052801561031e57816020015b6040805180820190915260008152606060208201528152602001906001900390816102f65790505b5092503660005b8281101561047757600085828151811061034157610341610d60565b6020026020010151905087878381811061035d5761035d610d60565b905060200281019061036f9190610d8f565b6040810135958601959093506103886020850185610ce2565b73ffffffffffffffffffffffffffffffffffffffff16816103ac6060870187610dcd565b6040516103ba929190610e32565b60006040518083038185875af1925050503d80600081146103f7576040519150601f19603f3d011682016040523d82523d6000602084013e6103fc565b606091505b50602080850191909152901515808452908501351761046d577f08c379a000000000000000000000000000000000000000000000000000000000600052602060045260176024527f4d756c746963616c6c333a2063616c6c206661696c656400000000000000000060445260846000fd5b5050600101610325565b508234146104e6576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152601a60248201527f4d756c746963616c6c333a2076616c7565206d69736d6174636800000000000060448201526064015b60405180910390fd5b50505092915050565b436060828067ffffffffffffffff81111561050c5761050c610d31565b60405190808252806020026020018201604052801561053f57816020015b606081526020019060019003908161052a5790505b5091503660005b8281101561068657600087878381811061056257610562610d60565b90506020028101906105749190610e42565b92506105836020840184610ce2565b73ffffffffffffffffffffffffffffffffffffffff166105a66020850185610dcd565b6040516105b4929190610e32565b6000604051808303816000865af19150503d80600081146105f1576040519150601f19603f3d011682016040523d82523d6000602084013e6105f6565b606091505b5086848151811061060957610609610d60565b602090810291909101015290508061067d576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152601760248201527f4d756c746963616c6c333a2063616c6c206661696c656400000000000000000060448201526064016104dd565b50600101610546565b5050509250929050565b43804060606106a086868661085a565b905093509350939050565b6060818067ffffffffffffffff8111156106c7576106c7610d31565b60405190808252806020026020018201604052801561070d57816020015b6040805180820190915260008152606060208201528152602001906001900390816106e55790505b5091503660005b828110156104e657600084828151811061073057610730610d60565b6020026020010151905086868381811061074c5761074c610d60565b905060200281019061075e9190610e76565b925061076d6020840184610ce2565b73ffffffffffffffffffffffffffffffffffffffff166107906040850185610dcd565b60405161079e929190610e32565b6000604051808303816000865af19150503d80600081146107db576040519150601f19603f3d011682016040523d82523d6000602084013e6107e0565b606091505b506020808401919091529015158083529084013517610851577f08c379a000000000000000000000000000000000000000000000000000000000600052602060045260176024527f4d756c746963616c6c333a2063616c6c206661696c656400000000000000000060445260646000fd5b50600101610714565b6060818067ffffffffffffffff81111561087657610876610d31565b6040519080825280602002602001820160405280156108bc57816020015b6040805180820190915260008152606060208201528152602001906001900390816108945790505b5091503660005b82811015610a105760008482815181106108df576108df610d60565b602002602001015190508686838181106108fb576108fb610d60565b905060200281019061090d9190610e42565b925061091c6020840184610ce2565b73ffffffffffffffffffffffffffffffffffffffff1661093f6020850185610dcd565b60405161094d929190610e32565b6000604051808303816000865af19150503d806000811461098a576040519150601f19603f3d011682016040523d82523d6000602084013e61098f565b606091505b506020830152151581528715610a07578051610a07576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152601760248201527f4d756c746963616c6c333a2063616c6c206661696c656400000000000000000060448201526064016104dd565b506001016108c3565b5050509392505050565b6000806060610a2b60018686610690565b919790965090945092505050565b60008083601f840112610a4b57600080fd5b50813567ffffffffffffffff811115610a6357600080fd5b6020830191508360208260051b8501011115610a7e57600080fd5b9250929050565b60008060208385031215610a9857600080fd5b823567ffffffffffffffff811115610aaf57600080fd5b610abb85828601610a39565b90969095509350505050565b6000815180845260005b81811015610aed57602081850181015186830182015201610ad1565b81811115610aff576000602083870101525b50601f017fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe0169290920160200192915050565b600082825180855260208086019550808260051b84010181860160005b84811015610bb1578583037fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe001895281518051151584528401516040858501819052610b9d81860183610ac7565b9a86019a9450505090830190600101610b4f565b5090979650505050505050565b602081526000610bd16020830184610b32565b9392505050565b600060408201848352602060408185015281855180845260608601915060608160051b870101935082870160005b82811015610c52577fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffa0888703018452610c40868351610ac7565b95509284019290840190600101610c06565b509398975050505050505050565b600080600060408486031215610c7557600080fd5b83358015158114610c8557600080fd5b9250602084013567ffffffffffffffff811115610ca157600080fd5b610cad86828701610a39565b9497909650939450505050565b838152826020820152606060408201526000610cd96060830184610b32565b95945050505050565b600060208284031215610cf457600080fd5b813573ffffffffffffffffffffffffffffffffffffffff81168114610bd157600080fd5b600060208284031215610d2a57600080fd5b5035919050565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052604160045260246000fd5b7f4e487b7100000000000000000000000000000000000000000000000000000000600052603260045260246000fd5b600082357fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff81833603018112610dc357600080fd5b9190910192915050565b60008083357fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe1843603018112610e0257600080fd5b83018035915067ffffffffffffffff821115610e1d57600080fd5b602001915036819003821315610a7e57600080fd5b8183823760009101908152919050565b600082357fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffc1833603018112610dc357600080fd5b600082357fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffa1833603018112610dc357600080fdfea2646970667358221220bb2b5c71a328032f97c676ae39a1ec2148d3e5d6f73d95e9b17910152d61f16264736f6c634300080c0033"
echo -n "  Contract $addr: "
code=$(curl -s -X POST -H "Content-Type: application/json" --data "{\"jsonrpc\":\"2.0\",\"method\":\"eth_getCode\",\"params\":[\"$addr\",\"latest\"],\"id\":1}" http://localhost:45128 | jq -r '.result')
if [[ "$code" != "$expected" ]]; then
  echo "Failed to verify multicall contract"
  echo "Expected: $expected"
  echo "Got:      $code"
  
  killall tacchaind
  exit 1
else
  echo "Verified multicall contract successfully"
fi

# verify x/vm params
echo "Verifying x/vm params"
expected_evm_params='{
  "evm_denom": "utac",
  "extra_eips": [
    "3855"
  ],
  "chain_config": {
    "homestead_block": "0",
    "dao_fork_block": "0",
    "dao_fork_support": true,
    "eip150_block": "0",
    "eip155_block": "0",
    "eip158_block": "0",
    "byzantium_block": "0",
    "constantinople_block": "0",
    "petersburg_block": "0",
    "istanbul_block": "0",
    "muir_glacier_block": "0",
    "berlin_block": "0",
    "london_block": "0",
    "arrow_glacier_block": "0",
    "gray_glacier_block": "0",
    "merge_netsplit_block": "0",
    "chain_id": "239",
    "denom": "utac",
    "decimals": "18",
    "shanghai_time": "0",
    "cancun_time": "0",
    "prague_time": null,
    "verkle_time": null
  },
  "allow_unprotected_txs": true,
  "evm_channels": [],
  "access_control": {
    "create": {
      "access_type": "ACCESS_TYPE_PERMISSIONLESS",
      "access_control_list": []
    },
    "call": {
      "access_type": "ACCESS_TYPE_PERMISSIONLESS",
      "access_control_list": []
    }
  },
  "active_static_precompiles": [
    "0x0000000000000000000000000000000000000100",
    "0x0000000000000000000000000000000000000400",
    "0x0000000000000000000000000000000000000800",
    "0x0000000000000000000000000000000000000801",
    "0x0000000000000000000000000000000000000802",
    "0x0000000000000000000000000000000000000803",
    "0x0000000000000000000000000000000000000804",
    "0x0000000000000000000000000000000000000805",
    "0x0000000000000000000000000000000000000806",
    "0x0000000000000000000000000000000000000807"
  ]
}'
evm_params=$(tacchaind q evm params --node http://localhost:45111 --output json | jq -r '.params')
if [[ "$evm_params" != "$expected_evm_params" ]]; then
  echo "Failed to verify x/vm params"
  echo "Expected: $expected_evm_params"
  echo "Got:      $evm_params"
  
  killall tacchaind
  exit 1
else
  echo "Verified x/vm params successfully"
fi

# verify x/feemarket min gas price
echo "Verifying x/feemarket min gas price"
expected_feemarket_min_gas_price='25000000000.000000000000000000'
feemarket_min_gas_price=$(tacchaind q feemarket params --node http://localhost:45111 --output json | jq -r '.params .min_gas_price')
if [[ "$feemarket_min_gas_price" != "$expected_feemarket_min_gas_price" ]]; then
  echo "Failed to verify x/feemarket min gas price"
  echo "Expected: $expected_feemarket_min_gas_price"
  echo "Got:      $feemarket_min_gas_price"
  
  killall tacchaind
  exit 1
else
  echo "Verified feemarket min gas price successfully"
fi

# verify max gas
echo "Verifying max gas"
expected_max_gas="90000000"
max_gas=$(tacchaind q consensus params --node http://localhost:45111 --output json | jq -r '.params .block .max_gas')
if [[ "$max_gas" != "$expected_max_gas" ]]; then
  echo "Failed to verify max gas"
  echo "Expected: $expected_max_gas"
  echo "Got:      $max_gas"
  
  killall tacchaind
  exit 1
else
  echo "Verified max gas successfully"
fi


# verify timeout commit
echo "Verifying timeout_commit"
expected_timeout_commit="2s"
timeout_commit=$(grep '^timeout_commit' $HOMEDIR/node0/config/config.toml | cut -d '=' -f2 | tr -d ' "')
if [[ "$timeout_commit" != "$expected_timeout_commit" ]]; then
  echo "Failed to verify timeout_commit"
  echo "Expected: $expected_timeout_commit"
  echo "Got:      $timeout_commit"
  
  killall tacchaind
  exit 1
else
  echo "Verified timeout_commit successfully"
fi

# verify x/mint params
echo "Verifying x/mint params"
expected_mint_params='{
  "mint_denom": "utac",
  "inflation_rate_change": "0.130000000000000000",
  "inflation_max": "0.050000000000000000",
  "inflation_min": "0.000000000000000000",
  "goal_bonded": "0.600000000000000000",
  "blocks_per_year": "15768000"
}'
mint_params=$(tacchaind q mint params --node http://localhost:45111 --output json | jq -r '.params')
if [[ "$mint_params" != "$expected_mint_params" ]]; then
  echo "Failed to verify x/mint params"
  echo "Expected: $expected_mint_params"
  echo "Got:      $mint_params"
  
  killall tacchaind
  exit 1
else
  echo "Verified x/mint params successfully"
fi

# verify x/gov params
echo "Verifying x/gov params"
expected_gov_params='{
  "min_deposit": [
    {
      "denom": "utac",
      "amount": "10000000000000000"
    }
  ],
  "max_deposit_period": "48h0m0s",
  "voting_period": "12h0m0s",
  "quorum": "0.334000000000000000",
  "threshold": "0.500000000000000000",
  "veto_threshold": "0.334000000000000000",
  "min_initial_deposit_ratio": "0.000000000000000000",
  "proposal_cancel_ratio": "0.500000000000000000",
  "expedited_voting_period": "6h0m0s",
  "expedited_threshold": "0.667000000000000000",
  "expedited_min_deposit": [
    {
      "denom": "utac",
      "amount": "50000000000000000"
    }
  ],
  "burn_vote_veto": true,
  "min_deposit_ratio": "0.010000000000000000"
}'
gov_params=$(tacchaind q gov params --node http://localhost:45111 --output json | jq -r '.params')
if [[ "$gov_params" != "$expected_gov_params" ]]; then
  echo "Failed to verify x/gov params"
  echo "Expected: $expected_gov_params"
  echo "Got:      $gov_params"
  
  killall tacchaind
  exit 1
else
  echo "Verified x/gov params successfully"
fi

# verify api is enabled
echo "Verifying API is enabled"
expected_network="tacchain_239-1"
network=$(curl -s http://localhost:45122/cosmos/base/tendermint/v1beta1/node_info | jq -r '.default_node_info .network')
if [[ "$network" != "$expected_network" ]]; then
  echo "Failed to verify API is enabled"
  echo "Expected: $expected_network"
  echo "Got:      $network"
  
  killall tacchaind
  exit 1
else
  echo "Verified API is enabled successfully"
fi

# verify x/slashing params
echo "Verifying x/slashing params"
expected_slashing_params='{
  "signed_blocks_window": "21600",
  "min_signed_per_window": "0.500000000000000000",
  "downtime_jail_duration": "10m0s",
  "slash_fraction_double_sign": "0.050000000000000000",
  "slash_fraction_downtime": "0.001000000000000000"
}'
slashing_params=$(tacchaind q slashing params --node http://localhost:45111 --output json | jq -r '.params')
if [[ "$slashing_params" != "$expected_slashing_params" ]]; then
  echo "Failed to verify x/slashing params"
  echo "Expected: $expected_slashing_params"
  echo "Got:      $slashing_params"
  
  killall tacchaind
  exit 1
else
  echo "Verified x/slashing params successfully"
fi

# verify x/bank denom metadata
echo "Verifying x/bank denom metadata"
expected_denom_metadata='{
  "description": "The native staking token for tacchaind.",
  "denom_units": [
    {
      "denom": "utac"
    },
    {
      "denom": "tac",
      "exponent": 18
    }
  ],
  "base": "utac",
  "display": "tac",
  "name": "TAC Token",
  "symbol": "TAC"
}'
denom_metadata=$(tacchaind q bank denom-metadata utac --node http://localhost:45111 --output json | jq -r '.metadata')
if [[ "$denom_metadata" != "$expected_denom_metadata" ]]; then
  echo "Failed to verify x/bank denom metadata"
  echo "Expected: $expected_denom_metadata"
  echo "Got:      $denom_metadata"
  
  killall tacchaind
  exit 1
else
  echo "Verified x/bank denom metadata successfully"
fi

killall tacchaind
echo "All tests passed successfully"

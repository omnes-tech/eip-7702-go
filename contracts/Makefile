-include .env

ifeq ($(network),holesky)
	EXPLORER_API_KEY := --etherscan-api-key $(ETHERSCAN_API_KEY)
	NETWORK_ARGS := --rpc-url $(ETHEREUM_HOLESKY_RPC) $(EXPLORER_API_KEY) --broadcast --gas-estimate-multiplier 1000 --verify -vvvv
	CHAIN_ID := 17000
endif

ifeq ($(network),anvil)
	NETWORK_ARGS := --rpc-url http://localhost:8545 --broadcast --hardfork prague
endif

ifeq ($(network),ethereum)
	NETWORK_ARGS := --rpc-url $(ETHEREUM_RPC_URL) --broadcast --verify -vvvv
	CHAIN_ID := 1
endif

ifeq ($(network),amoy)
	EXPLORER_API_KEY := --etherscan-api-key $(POLYGONSCAN_API_KEY) --verifier-url https://api-amoy.polygonscan.com/api
	NETWORK_ARGS := --rpc-url $(AMOY_RPC_URL) $(EXPLORER_API_KEY) --broadcast --verify -vvvv
	CHAIN_ID := 80002
endif

ifeq ($(network),polygon)
	EXPLORER_API_KEY := --etherscan-api-key $(POLYGONSCAN_API_KEY) --verifier-url https://api.polygonscan.com/api
	NETWORK_ARGS := --rpc-url $(POLYGON_RPC_URL) $(EXPLORER_API_KEY) --broadcast --verify
	CHAIN_ID := 137
endif
### simulate  -- --dry-run

ifeq ($(network),moonbase)
	EXPLORER_API_KEY := --etherscan-api-key $(MOONSCAN_API_KEY)
	NETWORK_ARGS := --rpc-url $(MOONBASE_RPC_URL) $(EXPLORER_API_KEY) --broadcast --verify -vvvv
	CHAIN_ID := 1287
endif

ifeq ($(network),linea_sepolia)
	EXPLORER_API_KEY := --etherscan-api-key $(ETHERSCAN_API_KEY)
	NETWORK_ARGS := --rpc-url $(LINEA_SEPOLIA_RPC_URL) $(EXPLORER_API_KEY) --broadcast --gas-estimate-multiplier 800
	CHAIN_ID := 59141
endif

ifeq ($(network),optimism_sepolia)
	EXPLORER_API_KEY := --etherscan-api-key $(ETHERSCAN_API_KEY)
	NETWORK_ARGS := --rpc-url $(OPTIMISM_SEPOLIA_RPC_URL) $(EXPLORER_API_KEY) --broadcast --verify -vvvv
	CHAIN_ID := 11155420
endif

ifeq ($(network),base_sepolia)
	NETWORK_ARGS := --rpc-url $(BASE_RPC_URL) $(EXPLORER_API_KEY) --broadcast --gas-estimate-multiplier 300
	CHAIN_ID := 84532
endif

ifeq ($(network),chiliz_spicy)
	NETWORK_ARGS := --rpc-url $(CHILIZ_RPC_URL_TESTNET) --broadcast --priority-gas-price 5000gwei -- --max-fee-per-gas 8000gwei
	CHAIN_ID := 88882
endif

ifeq ($(network),arbitrum_sepolia)
	NETWORK_ARGS := --rpc-url $(ARBITRUM_RPC_URL_TESTNET) --broadcast
	CHAIN_ID := 421614
endif

ifneq ($(constructor_signature),)
	CONSTRUCTOR_COMMAND := $(shell cast abi-encode "$(constructor_signature)" $(input_parameters))
	VERIFY_COMMAND := $(contract_address) $(contract) --chain-id $(CHAIN_ID) --constructor-args $(CONSTRUCTOR_COMMAND) $(EXPLORER_API_KEY) --watch
else
	VERIFY_COMMAND := $(contract_address) $(contract) --chain-id $(CHAIN_ID) $(EXPLORER_API_KEY) --watch
endif

ifeq ($(contract),WalletAccount)
	SCRIPT := script/UpgradeContract.s.sol
endif

ifeq ($(contract),AccountFactory)
	SCRIPT := script/UpgradeContract.s.sol
endif

run_test:;
	@forge test --fork-url $(SEPOLIA_RPC_URL) -vvvv

run_coverage:;
	@forge coverage --mp "test/unit/*" --report lcov

coverage_report:;
	@genhtml lcov.info --branch-coverage --output-dir coverage

deploy_Eip7702:;
	@forge script script/Eip7702.s.sol $(NETWORK_ARGS)

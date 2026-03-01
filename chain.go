package main

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// ChainClient holds RPC connection and wallet for on-chain interaction.
type ChainClient struct {
	Client         *ethclient.Client
	PrivateKey     *ecdsa.PrivateKey
	OwnerAddress   common.Address
	FactoryAddress common.Address
	FactoryABI     abi.ABI
	VaultABI       abi.ABI
	ChainID        *big.Int
}

const factoryABIJSON = `[{"type":"constructor","name":"","stateMutability":"","constant":false,"inputs":[{"type":"address","name":"_owner","simpleType":"address"},{"type":"address","name":"_oracle","simpleType":"address"}],"id":"4f86d68f-0c37-4865-ab22-3bc559c8d290"},{"type":"function","name":"createMatch","stateMutability":"nonpayable","constant":false,"inputs":[{"type":"string","name":"_teamA","simpleType":"string"},{"type":"string","name":"_teamB","simpleType":"string"}],"outputs":[{"type":"uint256","name":"matchId","simpleType":"uint"},{"type":"address","name":"vault","simpleType":"address"}],"id":"0x57447733"},{"type":"function","name":"getVault","stateMutability":"view","constant":false,"inputs":[{"type":"uint256","name":"_matchId","simpleType":"uint"}],"outputs":[{"type":"address","name":"","simpleType":"address"}],"id":"0x9403b634"},{"type":"function","name":"owner","stateMutability":"view","constant":false,"outputs":[{"type":"address","name":"","simpleType":"address"}],"id":"0x8da5cb5b"},{"type":"function","name":"totalMatches","stateMutability":"view","constant":false,"outputs":[{"type":"uint256","name":"","simpleType":"uint"}],"id":"0x2a5b1451"},{"type":"function","name":"allVaults","stateMutability":"view","constant":false,"inputs":[{"type":"uint256","name":"","simpleType":"uint"}],"outputs":[{"type":"address","name":"","simpleType":"address"}],"id":"0x9094a91e"},{"type":"function","name":"oracle","stateMutability":"view","constant":false,"outputs":[{"type":"address","name":"","simpleType":"address"}],"id":"0x7dc0d1d0"},{"type":"function","name":"nextMatchId","stateMutability":"view","constant":false,"outputs":[{"type":"uint256","name":"","simpleType":"uint"}],"id":"0xc5adf7c9"},{"type":"function","name":"renounceOwnership","stateMutability":"nonpayable","constant":false,"id":"0x715018a6"},{"type":"function","name":"setOracle","stateMutability":"nonpayable","constant":false,"inputs":[{"type":"address","name":"_newOracle","simpleType":"address"}],"id":"0x7adbf973"},{"type":"function","name":"transferOwnership","stateMutability":"nonpayable","constant":false,"inputs":[{"type":"address","name":"newOwner","simpleType":"address"}],"id":"0xf2fde38b"},{"type":"function","name":"vaults","stateMutability":"view","constant":false,"inputs":[{"type":"uint256","name":"","simpleType":"uint"}],"outputs":[{"type":"address","name":"","simpleType":"address"}],"id":"0x8c64ea4a"},{"type":"event","name":"MatchCreated","stateMutability":"","constant":false,"inputs":[{"type":"uint256","name":"matchId","simpleType":"uint"},{"type":"address","name":"vault","simpleType":"address"},{"type":"string","name":"teamA","simpleType":"string"},{"type":"string","name":"teamB","simpleType":"string"}],"id":"0x8fcc2ddfd2c264d34bb98b27183fb6ef55ed843dac114ab6b200cc1e8bc64324"},{"type":"event","name":"OracleUpdated","stateMutability":"","constant":false,"inputs":[{"type":"address","name":"oldOracle","simpleType":"address"},{"type":"address","name":"newOracle","simpleType":"address"}],"id":"0x078c3b417dadf69374a59793b829c52001247130433427049317bde56607b1b7"},{"type":"event","name":"OwnershipTransferred","stateMutability":"","constant":false,"inputs":[{"type":"address","name":"previousOwner","simpleType":"address"},{"type":"address","name":"newOwner","simpleType":"address"}],"id":"0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0"}]`

const vaultABIJSON = `[{"type":"constructor","name":"","stateMutability":"","constant":false,"inputs":[{"type":"address","name":"_owner","simpleType":"address"},{"type":"address","name":"_oracle","simpleType":"address"},{"type":"uint256","name":"_matchId","simpleType":"uint"},{"type":"string","name":"_teamA","simpleType":"string"},{"type":"string","name":"_teamB","simpleType":"string"}],"id":"e64cf70e-dfc6-4834-9332-40a07259f14b"},{"type":"function","name":"emergencyWithdrawFromYield","stateMutability":"nonpayable","constant":false,"id":"0x0e070cb0"},{"type":"function","name":"MORPHO_VAULT","stateMutability":"view","constant":false,"outputs":[{"type":"address","name":"","simpleType":"address"}],"id":"0x5bca49d2"},{"type":"function","name":"setExpectedWorkflowId","stateMutability":"nonpayable","constant":false,"inputs":[{"type":"bytes32","name":"_id","simpleType":"bytes"}],"id":"0xc3c44ac2"},{"type":"function","name":"MATCH_ID","stateMutability":"view","constant":false,"outputs":[{"type":"uint256","name":"","simpleType":"uint"}],"id":"0x75e10829"},{"type":"function","name":"getYieldBalance","stateMutability":"view","constant":false,"outputs":[{"type":"uint256","name":"","simpleType":"uint"}],"id":"0x489cc20c"},{"type":"function","name":"claim","stateMutability":"nonpayable","constant":false,"id":"0x4e71d92d"},{"type":"function","name":"deposit","stateMutability":"nonpayable","constant":false,"inputs":[{"type":"uint8","name":"team","simpleType":"uint"},{"type":"uint256","name":"amount","simpleType":"uint"}],"id":"0xf4d4c9d7"},{"type":"function","name":"totalTeamB","stateMutability":"view","constant":false,"outputs":[{"type":"uint256","name":"","simpleType":"uint"}],"id":"0x2288fc2b"},{"type":"function","name":"paused","stateMutability":"view","constant":false,"outputs":[{"type":"bool","name":"","simpleType":"bool"}],"id":"0x5c975abb"},{"type":"function","name":"unpause","stateMutability":"nonpayable","constant":false,"id":"0x3f4ba83a"},{"type":"function","name":"getTotalDeposits","stateMutability":"view","constant":false,"outputs":[{"type":"uint256","name":"","simpleType":"uint"}],"id":"0x168a4822"},{"type":"function","name":"onReport","stateMutability":"nonpayable","constant":false,"inputs":[{"type":"bytes","name":"metadata","simpleType":"bytes"},{"type":"bytes","name":"report","simpleType":"bytes"}],"id":"0x805f2132"},{"type":"function","name":"oracle","stateMutability":"view","constant":false,"outputs":[{"type":"address","name":"","simpleType":"address"}],"id":"0x7dc0d1d0"},{"type":"function","name":"getExpectedAuthor","stateMutability":"view","constant":false,"outputs":[{"type":"address","name":"","simpleType":"address"}],"id":"0x3397cf67"},{"type":"function","name":"pause","stateMutability":"nonpayable","constant":false,"id":"0x8456cb59"},{"type":"function","name":"emergencyRefund","stateMutability":"nonpayable","constant":false,"inputs":[{"type":"address","name":"user","simpleType":"address"}],"id":"0x045f7019"},{"type":"function","name":"status","stateMutability":"view","constant":false,"outputs":[{"type":"uint8","name":"","simpleType":"uint"}],"id":"0x200d2ed2"},{"type":"function","name":"USDC","stateMutability":"view","constant":false,"outputs":[{"type":"address","name":"","simpleType":"address"}],"id":"0x89a30271"},{"type":"function","name":"owner","stateMutability":"view","constant":false,"outputs":[{"type":"address","name":"","simpleType":"address"}],"id":"0x8da5cb5b"},{"type":"function","name":"setOracle","stateMutability":"nonpayable","constant":false,"inputs":[{"type":"address","name":"_newOracle","simpleType":"address"}],"id":"0x7adbf973"},{"type":"function","name":"lockMatch","stateMutability":"nonpayable","constant":false,"id":"0xc49df529"},{"type":"function","name":"setExpectedAuthor","stateMutability":"nonpayable","constant":false,"inputs":[{"type":"address","name":"_author","simpleType":"address"}],"id":"0xd60c884b"},{"type":"function","name":"setForwarderAddress","stateMutability":"nonpayable","constant":false,"inputs":[{"type":"address","name":"_forwarder","simpleType":"address"}],"id":"0xd777cc6d"},{"type":"function","name":"supportsInterface","stateMutability":"view","constant":false,"inputs":[{"type":"bytes4","name":"interfaceId","simpleType":"bytes"}],"outputs":[{"type":"bool","name":"","simpleType":"bool"}],"id":"0x01ffc9a7"},{"type":"function","name":"getExpectedWorkflowName","stateMutability":"view","constant":false,"outputs":[{"type":"bytes10","name":"","simpleType":"bytes"}],"id":"0xa619d818"},{"type":"function","name":"getForwarderAddress","stateMutability":"view","constant":false,"outputs":[{"type":"address","name":"","simpleType":"address"}],"id":"0x3441856f"},{"type":"function","name":"getExpectedPayout","stateMutability":"view","constant":false,"inputs":[{"type":"address","name":"user","simpleType":"address"}],"outputs":[{"type":"uint256","name":"","simpleType":"uint"}],"id":"0x98eca55f"},{"type":"function","name":"hasClaimed","stateMutability":"view","constant":false,"inputs":[{"type":"address","name":"","simpleType":"address"}],"outputs":[{"type":"bool","name":"","simpleType":"bool"}],"id":"0x73b2e80e"},{"type":"function","name":"getUserTotalDeposit","stateMutability":"view","constant":false,"inputs":[{"type":"address","name":"user","simpleType":"address"}],"outputs":[{"type":"uint256","name":"","simpleType":"uint"}],"id":"0xa4c828dc"},{"type":"function","name":"winner","stateMutability":"view","constant":false,"outputs":[{"type":"uint8","name":"","simpleType":"uint"}],"id":"0xdfbf53ae"},{"type":"function","name":"renounceOwnership","stateMutability":"nonpayable","constant":false,"id":"0x715018a6"},{"type":"function","name":"totalTeamA","stateMutability":"view","constant":false,"outputs":[{"type":"uint256","name":"","simpleType":"uint"}],"id":"0x669b96e2"},{"type":"function","name":"totalYield","stateMutability":"view","constant":false,"outputs":[{"type":"uint256","name":"","simpleType":"uint"}],"id":"0x01418205"},{"type":"function","name":"userDeposits","stateMutability":"view","constant":false,"inputs":[{"type":"address","name":"","simpleType":"address"},{"type":"uint8","name":"","simpleType":"uint"}],"outputs":[{"type":"uint256","name":"","simpleType":"uint"}],"id":"0x42db5e35"},{"type":"function","name":"teamBName","stateMutability":"view","constant":false,"outputs":[{"type":"string","name":"","simpleType":"string"}],"id":"0x11aa09b1"},{"type":"function","name":"teamAName","stateMutability":"view","constant":false,"outputs":[{"type":"string","name":"","simpleType":"string"}],"id":"0x4e108c19"},{"type":"function","name":"resolveMatch","stateMutability":"nonpayable","constant":false,"inputs":[{"type":"uint8","name":"_winner","simpleType":"uint"}],"id":"0x7986ad49"},{"type":"function","name":"getExpectedWorkflowId","stateMutability":"view","constant":false,"outputs":[{"type":"bytes32","name":"","simpleType":"bytes"}],"id":"0xf5c793ef"},{"type":"function","name":"setExpectedWorkflowName","stateMutability":"nonpayable","constant":false,"inputs":[{"type":"string","name":"_name","simpleType":"string"}],"id":"0xbc1fc27a"},{"type":"function","name":"transferOwnership","stateMutability":"nonpayable","constant":false,"inputs":[{"type":"address","name":"newOwner","simpleType":"address"}],"id":"0xf2fde38b"},{"type":"event","name":"SecurityWarning","stateMutability":"","constant":false,"inputs":[{"type":"string","name":"message","simpleType":"string"}],"id":"0x704da7db165c79c1e33d542c079333bbde970a733032d2f95fec8fb7d770cbf7"},{"type":"event","name":"Deposited","stateMutability":"","constant":false,"inputs":[{"type":"address","name":"user","simpleType":"address"},{"type":"uint8","name":"team","simpleType":"uint"},{"type":"uint256","name":"amount","simpleType":"uint"}],"id":"0x1d0787aee899a49ef81d0b11da9aca5455b46aefed042a41bd398d74619cab00"},{"type":"event","name":"OwnershipTransferred","stateMutability":"","constant":false,"inputs":[{"type":"address","name":"previousOwner","simpleType":"address"},{"type":"address","name":"newOwner","simpleType":"address"}],"id":"0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0"},{"type":"event","name":"EmergencyRefund","stateMutability":"","constant":false,"inputs":[{"type":"address","name":"user","simpleType":"address"},{"type":"uint256","name":"amount","simpleType":"uint"}],"id":"0xdf36e221948da014ebe0f9f6bb96696776424780da298e7f05e2f362dcd4289a"},{"type":"event","name":"ForwarderAddressUpdated","stateMutability":"","constant":false,"inputs":[{"type":"address","name":"previousForwarder","simpleType":"address"},{"type":"address","name":"newForwarder","simpleType":"address"}],"id":"0x039ad854736757070884dd787ef1a7f58db33546639d1f3efddcf4a33fb8997e"},{"type":"event","name":"ExpectedWorkflowIdUpdated","stateMutability":"","constant":false,"inputs":[{"type":"bytes32","name":"previousId","simpleType":"bytes"},{"type":"bytes32","name":"newId","simpleType":"bytes"}],"id":"0x0dbedcdf21925e053b4c574eae180d7f2883235ab4976ecc0873598a2a999b03"},{"type":"event","name":"ExpectedWorkflowNameUpdated","stateMutability":"","constant":false,"inputs":[{"type":"bytes10","name":"previousName","simpleType":"bytes"},{"type":"bytes10","name":"newName","simpleType":"bytes"}],"id":"0x1e7ddd09d504c82dcfc784a464b167469f5aad967606ec4822d848ef9141dfa5"},{"type":"event","name":"MatchLocked","stateMutability":"","constant":false,"inputs":[{"type":"uint256","name":"matchId","simpleType":"uint"}],"id":"0x2f5f86d0163c8f8ac2745a053af228bf204225ec58d747c366151e05fc7a73a8"},{"type":"event","name":"OracleUpdated","stateMutability":"","constant":false,"inputs":[{"type":"address","name":"oldOracle","simpleType":"address"},{"type":"address","name":"newOracle","simpleType":"address"}],"id":"0x078c3b417dadf69374a59793b829c52001247130433427049317bde56607b1b7"},{"type":"event","name":"Paused","stateMutability":"","constant":false,"inputs":[{"type":"address","name":"account","simpleType":"address"}],"id":"0x62e78cea01bee320cd4e420270b5ea74000d11b0c9f74754ebdbfc544b05a258"},{"type":"event","name":"Unpaused","stateMutability":"","constant":false,"inputs":[{"type":"address","name":"account","simpleType":"address"}],"id":"0x5db9ee0a495bf2e6ff9c91a7834c1ba4fdd244a5e8aa4e537bd38aeae4b073aa"},{"type":"event","name":"Claimed","stateMutability":"","constant":false,"inputs":[{"type":"address","name":"user","simpleType":"address"},{"type":"uint256","name":"principal","simpleType":"uint"},{"type":"uint256","name":"yieldShare","simpleType":"uint"}],"id":"0x987d620f307ff6b94d58743cb7a7509f24071586a77759b77c2d4e29f75a2f9a"},{"type":"event","name":"EmergencyYieldWithdrawn","stateMutability":"","constant":false,"inputs":[{"type":"uint256","name":"amount","simpleType":"uint"}],"id":"0x32f0f1630e950fbc850f76becff89beb4eac3e75d09ee1abcf4a231fa783a65a"},{"type":"event","name":"ExpectedAuthorUpdated","stateMutability":"","constant":false,"inputs":[{"type":"address","name":"previousAuthor","simpleType":"address"},{"type":"address","name":"newAuthor","simpleType":"address"}],"id":"0x3321cda85c145617e47418aa14255e9dcbec53a753778e57591703b89a3cad31"},{"type":"event","name":"MatchResolved","stateMutability":"","constant":false,"inputs":[{"type":"uint256","name":"matchId","simpleType":"uint"},{"type":"uint8","name":"winner","simpleType":"uint"}],"id":"0xde1c752173fee6bd3976fb1f165361c647fffc7a33987556b7f54b6a2de08ece"}]`

// VaultStats holds on-chain data for a vault.
type VaultStats struct {
	TotalTeamA    *big.Int
	TotalTeamB    *big.Int
	TotalDeposits *big.Int
	YieldBalance  *big.Int
	Status        uint8 // 0=Open, 1=Locked, 2=Resolved
	Winner        uint8 // 0=None, 1=TeamA, 2=TeamB
}

// InitChain connects to the RPC and loads the wallet.
func InitChain() (*ChainClient, error) {
	rpcURL := os.Getenv("RPC_URL")
	if rpcURL == "" {
		return nil, fmt.Errorf("RPC_URL not set")
	}

	factoryAddr := os.Getenv("FACTORY_ADDRESS")
	if factoryAddr == "" {
		return nil, fmt.Errorf("FACTORY_ADDRESS not set")
	}

	pkHex := os.Getenv("OWNER_PRIVATE_KEY")
	if pkHex == "" || pkHex == "<put-owner-private-key-here>" {
		return nil, fmt.Errorf("OWNER_PRIVATE_KEY not set — please update panel/.env")
	}

	// Strip 0x prefix if present
	pkHex = strings.TrimPrefix(pkHex, "0x")

	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RPC: %w", err)
	}

	privateKey, err := crypto.HexToECDSA(pkHex)
	if err != nil {
		return nil, fmt.Errorf("invalid private key: %w", err)
	}

	publicKey := privateKey.Public().(*ecdsa.PublicKey)
	address := crypto.PubkeyToAddress(*publicKey)

	chainID, err := client.ChainID(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get chain ID: %w", err)
	}

	factoryABI, err := abi.JSON(strings.NewReader(factoryABIJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to parse factory ABI: %w", err)
	}

	vaultABI, err := abi.JSON(strings.NewReader(vaultABIJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to parse vault ABI: %w", err)
	}

	return &ChainClient{
		Client:         client,
		PrivateKey:     privateKey,
		OwnerAddress:   address,
		FactoryAddress: common.HexToAddress(factoryAddr),
		FactoryABI:     factoryABI,
		VaultABI:       vaultABI,
		ChainID:        chainID,
	}, nil
}

// CreateMatchOnChain calls FactoryMatch.createMatch() and returns (onChainMatchId, vaultAddress).
func (c *ChainClient) CreateMatchOnChain(teamA, teamB string) (uint64, common.Address, error) {
	data, err := c.FactoryABI.Pack("createMatch", teamA, teamB)
	if err != nil {
		return 0, common.Address{}, fmt.Errorf("failed to pack createMatch: %w", err)
	}

	auth, err := c.buildTxOpts()
	if err != nil {
		return 0, common.Address{}, err
	}

	tx, err := bind.NewBoundContract(c.FactoryAddress, c.FactoryABI, c.Client, c.Client, c.Client).Transact(auth, "createMatch", teamA, teamB)
	if err != nil {
		return 0, common.Address{}, fmt.Errorf("failed to send createMatch tx: %w", err)
	}

	// Wait for receipt
	receipt, err := bind.WaitMined(context.Background(), c.Client, tx)
	if err != nil {
		return 0, common.Address{}, fmt.Errorf("failed to wait for tx: %w", err)
	}

	if receipt.Status == 0 {
		return 0, common.Address{}, fmt.Errorf("createMatch tx reverted: %s", tx.Hash().Hex())
	}

	// Decode return values from output — call getVault to find the address
	// Since createMatch returns via tx, we parse the logs or read nextMatchId
	_ = data

	// Read nextMatchId (it was incremented, so current - 1 is what we just created)
	nextID, err := c.readUint256(c.FactoryAddress, c.FactoryABI, "nextMatchId")
	if err != nil {
		return 0, common.Address{}, fmt.Errorf("failed to read nextMatchId: %w", err)
	}
	matchID := nextID - 1

	// Read vault address
	vaultAddr, err := c.readAddress(c.FactoryAddress, c.FactoryABI, "getVault", new(big.Int).SetUint64(matchID))
	if err != nil {
		return 0, common.Address{}, fmt.Errorf("failed to read vault: %w", err)
	}

	return matchID, vaultAddr, nil
}

// ReadVaultStats reads on-chain stats for a VaultMatch.
func (c *ChainClient) ReadVaultStats(vaultAddr string) (*VaultStats, error) {
	addr := common.HexToAddress(vaultAddr)

	totalA, err := c.readUint256(addr, c.VaultABI, "totalTeamA")
	if err != nil {
		return nil, err
	}
	totalB, err := c.readUint256(addr, c.VaultABI, "totalTeamB")
	if err != nil {
		return nil, err
	}
	totalDeposits, err := c.readUint256(addr, c.VaultABI, "getTotalDeposits")
	if err != nil {
		return nil, err
	}
	// getYieldBalance calls Morpho convertToAssets which reverts when vault has 0 shares.
	// Treat failure as 0 yield — not a fatal error.
	yieldBalance, _ := c.readUint256(addr, c.VaultABI, "getYieldBalance")
	statusVal, err := c.readUint8(addr, c.VaultABI, "status")
	if err != nil {
		return nil, err
	}
	winnerVal, err := c.readUint8(addr, c.VaultABI, "winner")
	if err != nil {
		return nil, err
	}

	return &VaultStats{
		TotalTeamA:    new(big.Int).SetUint64(totalA),
		TotalTeamB:    new(big.Int).SetUint64(totalB),
		TotalDeposits: new(big.Int).SetUint64(totalDeposits),
		YieldBalance:  new(big.Int).SetUint64(yieldBalance),
		Status:        uint8(statusVal),
		Winner:        uint8(winnerVal),
	}, nil
}

// ── Internal helpers ────────────────────────────────────────────────────

func (c *ChainClient) buildTxOpts() (*bind.TransactOpts, error) {
	nonce, err := c.Client.PendingNonceAt(context.Background(), c.OwnerAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to get nonce: %w", err)
	}

	gasPrice, err := c.Client.SuggestGasPrice(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get gas price: %w", err)
	}

	auth, err := bind.NewKeyedTransactorWithChainID(c.PrivateKey, c.ChainID)
	if err != nil {
		return nil, fmt.Errorf("failed to create transactor: %w", err)
	}

	auth.Nonce = new(big.Int).SetUint64(nonce)
	auth.GasPrice = gasPrice
	auth.GasLimit = 5000000

	return auth, nil
}

func (c *ChainClient) readUint256(addr common.Address, contractABI abi.ABI, method string, args ...interface{}) (uint64, error) {
	data, err := contractABI.Pack(method, args...)
	if err != nil {
		return 0, fmt.Errorf("pack %s: %w", method, err)
	}

	result, err := c.Client.CallContract(context.Background(), ethereum.CallMsg{
		To:   &addr,
		Data: data,
	}, nil)
	if err != nil {
		return 0, fmt.Errorf("call %s: %w", method, err)
	}

	outputs, err := contractABI.Unpack(method, result)
	if err != nil {
		return 0, fmt.Errorf("unpack %s: %w", method, err)
	}

	if len(outputs) == 0 {
		return 0, fmt.Errorf("no output for %s", method)
	}

	if val, ok := outputs[0].(*big.Int); ok {
		return val.Uint64(), nil
	}

	return 0, fmt.Errorf("unexpected type for %s", method)
}

func (c *ChainClient) readUint8(addr common.Address, contractABI abi.ABI, method string) (uint64, error) {
	data, err := contractABI.Pack(method)
	if err != nil {
		return 0, fmt.Errorf("pack %s: %w", method, err)
	}

	result, err := c.Client.CallContract(context.Background(), ethereum.CallMsg{
		To:   &addr,
		Data: data,
	}, nil)
	if err != nil {
		return 0, fmt.Errorf("call %s: %w", method, err)
	}

	outputs, err := contractABI.Unpack(method, result)
	if err != nil {
		return 0, fmt.Errorf("unpack %s: %w", method, err)
	}

	if len(outputs) == 0 {
		return 0, fmt.Errorf("no output for %s", method)
	}

	if val, ok := outputs[0].(uint8); ok {
		return uint64(val), nil
	}

	return 0, fmt.Errorf("unexpected type for %s", method)
}

func (c *ChainClient) readAddress(addr common.Address, contractABI abi.ABI, method string, args ...interface{}) (common.Address, error) {
	data, err := contractABI.Pack(method, args...)
	if err != nil {
		return common.Address{}, fmt.Errorf("pack %s: %w", method, err)
	}

	result, err := c.Client.CallContract(context.Background(), ethereum.CallMsg{
		To:   &addr,
		Data: data,
	}, nil)
	if err != nil {
		return common.Address{}, fmt.Errorf("call %s: %w", method, err)
	}

	outputs, err := contractABI.Unpack(method, result)
	if err != nil {
		return common.Address{}, fmt.Errorf("unpack %s: %w", method, err)
	}

	if len(outputs) == 0 {
		return common.Address{}, fmt.Errorf("no output for %s", method)
	}

	if val, ok := outputs[0].(common.Address); ok {
		return val, nil
	}

	return common.Address{}, fmt.Errorf("unexpected type for %s", method)
}

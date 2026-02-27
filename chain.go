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

// FactoryMatch ABI (minimal — just what we need)
const factoryABIJSON = `[
	{
		"name": "createMatch",
		"type": "function",
		"stateMutability": "nonpayable",
		"inputs": [
			{"name": "_teamA", "type": "string"},
			{"name": "_teamB", "type": "string"}
		],
		"outputs": [
			{"name": "matchId", "type": "uint256"},
			{"name": "vault", "type": "address"}
		]
	},
	{
		"name": "getVault",
		"type": "function",
		"stateMutability": "view",
		"inputs": [{"name": "_matchId", "type": "uint256"}],
		"outputs": [{"name": "", "type": "address"}]
	},
	{
		"name": "nextMatchId",
		"type": "function",
		"stateMutability": "view",
		"inputs": [],
		"outputs": [{"name": "", "type": "uint256"}]
	},
	{
		"name": "totalMatches",
		"type": "function",
		"stateMutability": "view",
		"inputs": [],
		"outputs": [{"name": "", "type": "uint256"}]
	}
]`

// VaultMatch ABI (read functions + lockMatch + resolveMatch)
const vaultABIJSON = `[
	{
		"name": "totalTeamA",
		"type": "function",
		"stateMutability": "view",
		"inputs": [],
		"outputs": [{"name": "", "type": "uint256"}]
	},
	{
		"name": "totalTeamB",
		"type": "function",
		"stateMutability": "view",
		"inputs": [],
		"outputs": [{"name": "", "type": "uint256"}]
	},
	{
		"name": "getTotalDeposits",
		"type": "function",
		"stateMutability": "view",
		"inputs": [],
		"outputs": [{"name": "", "type": "uint256"}]
	},
	{
		"name": "getYieldBalance",
		"type": "function",
		"stateMutability": "view",
		"inputs": [],
		"outputs": [{"name": "", "type": "uint256"}]
	},
	{
		"name": "status",
		"type": "function",
		"stateMutability": "view",
		"inputs": [],
		"outputs": [{"name": "", "type": "uint8"}]
	},
	{
		"name": "winner",
		"type": "function",
		"stateMutability": "view",
		"inputs": [],
		"outputs": [{"name": "", "type": "uint8"}]
	},
	{
		"name": "lockMatch",
		"type": "function",
		"stateMutability": "nonpayable",
		"inputs": []
	},
	{
		"name": "resolveMatch",
		"type": "function",
		"stateMutability": "nonpayable",
		"inputs": [{"name": "_winner", "type": "uint8"}]
	},
	{
		"name": "teamAName",
		"type": "function",
		"stateMutability": "view",
		"inputs": [],
		"outputs": [{"name": "", "type": "string"}]
	},
	{
		"name": "teamBName",
		"type": "function",
		"stateMutability": "view",
		"inputs": [],
		"outputs": [{"name": "", "type": "string"}]
	}
]`

// VaultStats holds on-chain data for a vault.
type VaultStats struct {
	TotalTeamA    *big.Int
	TotalTeamB    *big.Int
	TotalDeposits *big.Int
	YieldBalance  *big.Int
	Status        uint8  // 0=Open, 1=Locked, 2=Resolved
	Winner        uint8  // 0=None, 1=TeamA, 2=TeamB
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
	yieldBalance, err := c.readUint256(addr, c.VaultABI, "getYieldBalance")
	if err != nil {
		return nil, err
	}
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

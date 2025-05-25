package eip7702

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

// EthRPCClient implementa EthClient usando ethclient
type EthRPCClient struct {
	client *ethclient.Client
	ctx    context.Context
}

func NewEthRPCClient(rpcURL string) (*EthRPCClient, error) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, err
	}

	return &EthRPCClient{
		client: client,
		ctx:    context.Background(),
	}, nil
}

func (e *EthRPCClient) NonceAt(from common.Address) (uint64, error) {
	return e.client.NonceAt(e.ctx, from, nil)
}

func (e *EthRPCClient) SuggestGasTipCap() (*big.Int, error) {
	return e.client.SuggestGasTipCap(e.ctx)
}

func (e *EthRPCClient) SendTransaction(tx *types.Transaction) error {
	return e.client.SendTransaction(e.ctx, tx)
}

func (e *EthRPCClient) ChainID() (*big.Int, error) {
	return e.client.ChainID(e.ctx)
}

package eip7702

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/holiman/uint256"
)

// Authorization com validações EIP-7702
type Authorization struct {
	ChainID   uint64         `json:"chain_id"`
	Address   common.Address `json:"address"` // Contrato delegado
	Nonce     uint64         `json:"nonce"`
	V         uint8          `json:"v"`
	R         [32]byte       `json:"r"`
	S         [32]byte       `json:"s"`
	Signer    common.Address `json:"signer"`     // EOA que delega
	CreatedAt int64          `json:"created_at"` // Timestamp para validação
}

// Call com validações de segurança
type Call struct {
	To       common.Address `json:"to"`
	Data     []byte         `json:"data"`
	Value    *big.Int       `json:"value"`
	GasLimit uint64         `json:"gas_limit,omitempty"` // Gas específico para esta call
}

// AuthorizeRequest é o payload para criar uma autorização
type AuthorizeRequest struct {
	ContractAddress string `json:"contract_address"` // Endereço do contrato para delegar
	SignerPK        string `json:"signer_pk"`        // Chave privada de quem autoriza
}

// SponsorRequest é o payload para execução patrocinada
type SponsorRequest struct {
	Authorization Authorization `json:"authorization"` // Autorização assinada
	Calls         []CallData    `json:"calls"`         // Chamadas a executar
	SponsorPK     string        `json:"sponsor_pk"`    // Chave privada do patrocinador
}

// CallData representa dados de chamada via JSON
type CallData struct {
	To    string `json:"to"`    // Endereço do contrato
	Data  string `json:"data"`  // Dados hexadecimais
	Value string `json:"value"` // Valor em wei (string para grandes números)
}

// SecureDelegationRequest com todas as validações EIP-7702
type SecureDelegationRequest struct {
	ContractAddress common.Address `json:"contract_address"`
	SignerPK        string         `json:"signer_pk"`

	// VALIDAÇÕES DE SEGURANÇA
	MaxGasPrice   *big.Int `json:"max_gas_price,omitempty"`  // Limite de gas price
	Deadline      int64    `json:"deadline,omitempty"`       // Timestamp limite
	ExpectedValue *big.Int `json:"expected_value,omitempty"` // Valor total esperado
}

// DelegationService com validações
type DelegationService struct {
	ChainID *big.Int
	RPC     EthClient
}

// EthClient interface para interação com a blockchain
type EthClient interface {
	NonceAt(from common.Address) (uint64, error)
	SuggestGasTipCap() (*big.Int, error)
	SendTransaction(tx *types.Transaction) error
	ChainID() (*big.Int, error)
}

// SignDelegation com validações completas EIP-7702
func (d *DelegationService) SignDelegation(contractAddr common.Address, signerPK *ecdsa.PrivateKey) (*Authorization, error) {
	// VALIDAÇÕES CRÍTICAS
	if d == nil || d.RPC == nil || d.ChainID == nil {
		return nil, errors.New("service not properly initialized")
	}
	if signerPK == nil {
		return nil, errors.New("signer private key is nil")
	}
	if (contractAddr == common.Address{}) {
		return nil, errors.New("contract address is zero")
	}

	signer := crypto.PubkeyToAddress(signerPK.PublicKey)

	// Verificar se é um contrato conhecido (segurança)
	if !d.isKnownContract(contractAddr) {
		return nil, fmt.Errorf("unknown/untrusted contract: %s", contractAddr.Hex())
	}

	// Obter nonce atual
	nonce, err := d.RPC.NonceAt(signer)
	if err != nil {
		return nil, fmt.Errorf("failed to get nonce for %s: %w", signer.Hex(), err)
	}

	// Criar mensagem EIP-7702: [chainId, address, nonce]
	chainID := d.ChainID.Uint64()
	authMessage := []interface{}{chainID, contractAddr, nonce}

	// RLP encode
	encoded, err := rlp.EncodeToBytes(authMessage)
	if err != nil {
		return nil, fmt.Errorf("failed to encode auth message: %w", err)
	}

	// Hash com magic byte 0x05
	hash := crypto.Keccak256Hash(append([]byte{0x05}, encoded...))

	// Assinar
	signature, err := crypto.Sign(hash.Bytes(), signerPK)
	if err != nil {
		return nil, fmt.Errorf("failed to sign authorization: %w", err)
	}

	// Extrair v, r, s
	v := signature[64]
	if v >= 27 {
		v -= 27
	}

	var r32, s32 [32]byte
	copy(r32[:], signature[:32])
	copy(s32[:], signature[32:64])

	return &Authorization{
		ChainID:   chainID,
		Address:   contractAddr,
		Nonce:     nonce,
		V:         v,
		R:         r32,
		S:         s32,
		Signer:    signer,
		CreatedAt: time.Now().Unix(),
	}, nil
}

// ExecuteSponsored com validações de segurança completas
func (d *DelegationService) ExecuteSponsored(auth *Authorization, calls []Call, sponsorPK *ecdsa.PrivateKey) (*types.Transaction, error) {
	// VALIDAÇÕES DE SEGURANÇA EIP-7702
	if err := d.validateAuthorization(auth); err != nil {
		return nil, fmt.Errorf("invalid authorization: %w", err)
	}
	if err := d.validateCalls(calls); err != nil {
		return nil, fmt.Errorf("invalid calls: %w", err)
	}

	sponsor := crypto.PubkeyToAddress(sponsorPK.PublicKey)
	sponsorNonce, err := d.RPC.NonceAt(sponsor)
	if err != nil {
		return nil, fmt.Errorf("failed to get sponsor nonce: %w", err)
	}

	// Gas configuration
	tip, err := d.RPC.SuggestGasTipCap()
	if err != nil {
		tip = big.NewInt(2_000_000_000) // fallback 2 Gwei
	}

	// Criar AuthList
	var r, s uint256.Int
	r.SetBytes(auth.R[:])
	s.SetBytes(auth.S[:])

	authList := []types.SetCodeAuthorization{{
		ChainID: *uint256.NewInt(auth.ChainID),
		Address: auth.Address,
		Nonce:   auth.Nonce,
		V:       auth.V,
		R:       r,
		S:       s,
	}}

	// Construir call data
	var txData []byte
	var gasLimit uint64 = 1_000_000 // default

	if len(calls) == 1 {
		txData = calls[0].Data
		if calls[0].GasLimit > 0 {
			gasLimit = calls[0].GasLimit
		}
	} else {
		// Usar execute para múltiplas calls
		builder := &CallDataBuilder{}
		executeData := builder.ExecuteCalls(calls)
		txData = common.Hex2Bytes(strings.TrimPrefix(executeData, "0x"))
		gasLimit = d.calculateMulticallGas(calls)
	}

	// Criar SetCodeTx
	setCodeTx := &types.SetCodeTx{
		ChainID:   uint256.MustFromBig(d.ChainID),
		Nonce:     sponsorNonce,
		GasTipCap: uint256.MustFromBig(tip),
		GasFeeCap: uint256.MustFromBig(new(big.Int).Mul(tip, big.NewInt(3))),
		Gas:       gasLimit,
		To:        auth.Signer,       // SEMPRE o signer (authority)
		Value:     uint256.NewInt(0), // ✅ SEMPRE 0 - sponsor só paga gas
		Data:      txData,
		AuthList:  authList,
	}

	// Assinar transação
	signer := types.NewPragueSigner(d.ChainID)
	return types.SignNewTx(sponsorPK, signer, setCodeTx)
}

// Validações de segurança conforme EIP-7702
func (d *DelegationService) validateAuthorization(auth *Authorization) error {
	if auth == nil {
		return errors.New("authorization is nil")
	}

	// Verificar idade da autorização (replay protection)
	if time.Now().Unix()-auth.CreatedAt > 300 { // 5 minutos
		return errors.New("authorization too old")
	}

	// Verificar chain ID
	if auth.ChainID != d.ChainID.Uint64() {
		return errors.New("chain ID mismatch")
	}

	// Verificar nonce atual
	currentNonce, err := d.RPC.NonceAt(auth.Signer)
	if err != nil {
		return fmt.Errorf("failed to check current nonce: %w", err)
	}
	if auth.Nonce != currentNonce {
		return fmt.Errorf("nonce mismatch: expected %d, got %d", currentNonce, auth.Nonce)
	}

	return nil
}

func (d *DelegationService) validateCalls(calls []Call) error {
	if len(calls) == 0 {
		return errors.New("no calls provided")
	}

	totalValue := big.NewInt(0)
	for i, call := range calls {
		if (call.To == common.Address{}) {
			return fmt.Errorf("call %d has zero address", i)
		}
		if call.Value != nil {
			totalValue.Add(totalValue, call.Value)
		}
	}

	// Verificar valor total (protection contra malicious sponsor)
	maxValue := new(big.Int).Mul(big.NewInt(10), big.NewInt(1e18)) // 10 ETH max
	if totalValue.Cmp(maxValue) > 0 {
		return fmt.Errorf("total value too high: %s", totalValue.String())
	}

	return nil
}

func (d *DelegationService) isKnownContract(addr common.Address) bool {
	knownContracts := map[common.Address]bool{
		common.HexToAddress(TokenContract):    true,
		common.HexToAddress(DelegateContract): true,
	}
	return knownContracts[addr]
}

func (d *DelegationService) calculateMulticallGas(calls []Call) uint64 {
	baseGas := uint64(100_000)   // base para execute
	perCallGas := uint64(50_000) // gas por call
	return baseGas + uint64(len(calls))*perCallGas
}

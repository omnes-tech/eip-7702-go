package eip7702

import (
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-chi/chi/v5"
)

// DelegationHandlers contém os handlers HTTP para EIP-7702
type DelegationHandlers struct {
	svc *DelegationService
}

func NewDelegationHandlers(service *DelegationService) *DelegationHandlers {
	return &DelegationHandlers{svc: service}
}

// Routes retorna as rotas HTTP para EIP-7702
func (h *DelegationHandlers) Routes() http.Handler {
	r := chi.NewRouter()

	// ===== ROTAS BÁSICAS =====
	r.Post("/authorize", h.handleAuthorize)
	r.Post("/sponsor", h.handleSponsor)

	// ===== ROTAS ESPECÍFICAS =====
	r.Post("/sponsor-eth", h.handleSponsorETH)
	r.Post("/sponsor-mint", h.handleSponsorMint)
	r.Post("/sponsor-transfer", h.handleSponsorTransfer)

	// ===== ROTA GENÉRICA (PRINCIPAL) =====
	r.Post("/sponsor-generic", h.handleSponsorGeneric)

	// ===== HELPERS PARA CALL DATA =====
	r.Post("/build-call/send-eth", h.handleBuildSendETH)
	r.Post("/build-call/mint", h.handleBuildMint)
	r.Post("/build-call/transfer", h.handleBuildTransfer)
	r.Post("/build-call/generic", h.handleBuildGeneric)

	// ===== ROTAS DE INFO =====
	r.Get("/contracts", h.handleGetContracts)

	return r
}

// parsePrivateKey - função helper padronizada
func parsePrivateKey(pkHex string) (*ecdsa.PrivateKey, error) {
	pkHex = strings.TrimPrefix(pkHex, "0x")
	return crypto.HexToECDSA(pkHex)
}

// Struct reutilizável para requests básicos
type BasicSponsorRequest struct {
	SignerPK  string `json:"signer_pk"`
	SponsorPK string `json:"sponsor_pk"`
	Recipient string `json:"recipient"`
	Amount    string `json:"amount"`
}

// Validação para a struct
func (req *BasicSponsorRequest) Validate() error {
	if req.SignerPK == "" || req.SponsorPK == "" || req.Recipient == "" || req.Amount == "" {
		return fmt.Errorf("missing required fields")
	}
	if !common.IsHexAddress(req.Recipient) {
		return fmt.Errorf("invalid recipient address")
	}
	return nil
}

// handleSponsorGeneric - ROTA PRINCIPAL GENÉRICA
func (h *DelegationHandlers) handleSponsorGeneric(w http.ResponseWriter, r *http.Request) {
	var in struct {
		SignerPK          string        `json:"signer_pk"`
		SponsorPK         string        `json:"sponsor_pk"`
		ContractAddress   string        `json:"contract_address"`   // SimpleDelegateContract
		FunctionSignature string        `json:"function_signature"` // Função DO SimpleDelegateContract
		Parameters        []interface{} `json:"parameters"`         // Parâmetros da função
	}
	json.NewDecoder(r.Body).Decode(&in)

	sk, err := parsePrivateKey(in.SignerPK)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid signer private key: %v", err), 400)
		return
	}

	sp, err := parsePrivateKey(in.SponsorPK)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid sponsor private key: %v", err), 400)
		return
	}

	if h.svc == nil {
		http.Error(w, "Service not initialized", 500)
		return
	}

	// Autorizar o contrato especificado
	auth, err := h.svc.SignDelegation(common.HexToAddress(in.ContractAddress), sk)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create authorization: %v", err), 500)
		return
	}

	// CORRIGIDO: Construir call data para função DO SimpleDelegateContract
	cd, err := (&CallDataBuilder{}).BuildGenericCall(in.FunctionSignature, in.Parameters)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to build call data: %v", err), 400)
		return
	}

	call := Call{
		To:   auth.Signer, // Authority que age como SimpleDelegateContract
		Data: common.Hex2Bytes(strings.TrimPrefix(cd, "0x")),
	}

	tx, err := h.svc.ExecuteSponsored(auth, []Call{call}, sp)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to execute sponsored transaction: %v", err), 500)
		return
	}

	if err := h.svc.RPC.SendTransaction(tx); err != nil {
		http.Error(w, fmt.Sprintf("Failed to send transaction: %v", err), 500)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"tx_hash":       tx.Hash().Hex(),
		"authorization": auth,
		"call_data":     cd,
		"function":      in.FunctionSignature,
	})
}

// handleSponsorMint - Rota específica para mint
func (h *DelegationHandlers) handleSponsorMint(w http.ResponseWriter, r *http.Request) {
	var req BasicSponsorRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), 400)
		return
	}

	if err := req.Validate(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	sk, err := parsePrivateKey(req.SignerPK)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid signer private key: %v", err), 400)
		return
	}

	sp, err := parsePrivateKey(req.SponsorPK)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid sponsor private key: %v", err), 400)
		return
	}

	if h.svc == nil {
		http.Error(w, "Service not initialized", 500)
		return
	}

	// Autorizar SimpleDelegateContract
	auth, err := h.svc.SignDelegation(common.HexToAddress(DelegateContract), sk)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create authorization: %v", err), 500)
		return
	}

	// Chamar SimpleDelegateContract.mint(token, to, amount)
	amtWei := TokenAmountToWei(req.Amount, 18)
	cd := (&CallDataBuilder{}).Mint(
		common.HexToAddress(TokenContract),
		common.HexToAddress(req.Recipient),
		amtWei,
	)
	call := Call{
		To:   auth.Signer, // Authority que age como SimpleDelegateContract
		Data: common.Hex2Bytes(strings.TrimPrefix(cd, "0x")),
	}

	tx, err := h.svc.ExecuteSponsored(auth, []Call{call}, sp)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to execute sponsored transaction: %v", err), 500)
		return
	}

	if err := h.svc.RPC.SendTransaction(tx); err != nil {
		http.Error(w, fmt.Sprintf("Failed to send transaction: %v", err), 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"tx_hash": tx.Hash().Hex()})
}

// handleSponsorTransfer - Rota específica para transfer
func (h *DelegationHandlers) handleSponsorTransfer(w http.ResponseWriter, r *http.Request) {
	var req BasicSponsorRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), 400)
		return
	}

	if err := req.Validate(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	sk, err := parsePrivateKey(req.SignerPK)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid signer private key: %v", err), 400)
		return
	}

	sp, err := parsePrivateKey(req.SponsorPK)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid sponsor private key: %v", err), 400)
		return
	}

	if h.svc == nil {
		http.Error(w, "Service not initialized", 500)
		return
	}

	// Autorizar SimpleDelegateContract
	auth, err := h.svc.SignDelegation(common.HexToAddress(DelegateContract), sk)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create authorization: %v", err), 500)
		return
	}

	// CORRIGIDO: Chamar SimpleDelegateContract.transfer(token, to, amount)
	amtWei := TokenAmountToWei(req.Amount, 18)
	cd := (&CallDataBuilder{}).Transfer(
		common.HexToAddress(TokenContract),
		common.HexToAddress(req.Recipient),
		amtWei,
	)
	call := Call{
		To:   auth.Signer, // Authority que age como SimpleDelegateContract
		Data: common.Hex2Bytes(strings.TrimPrefix(cd, "0x")),
	}

	tx, err := h.svc.ExecuteSponsored(auth, []Call{call}, sp)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to execute sponsored transaction: %v", err), 500)
		return
	}

	if err := h.svc.RPC.SendTransaction(tx); err != nil {
		http.Error(w, fmt.Sprintf("Failed to send transaction: %v", err), 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"tx_hash": tx.Hash().Hex()})
}

// handleBuildGeneric - Helper para construir call data genérico
func (h *DelegationHandlers) handleBuildGeneric(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FunctionSignature string        `json:"function_signature"` // ex: "transfer(address,uint256)"
		Parameters        []interface{} `json:"parameters"`         // ["0x123...", "1000"]
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	builder := &CallDataBuilder{}
	callData, err := builder.BuildGenericCall(req.FunctionSignature, req.Parameters)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to build call data: %v", err), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"call_data":          callData,
		"function_signature": req.FunctionSignature,
		"parameters":         req.Parameters,
	})
}

// handleGetContracts - Retorna endereços dos contratos deployados
func (h *DelegationHandlers) handleGetContracts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token_contract":           TokenContract,
		"simple_delegate_contract": DelegateContract,
		"network":                  "holesky",
		"chain_id":                 17000,
	})
}

// handleAuthorize cria uma autorização assinada para um contrato
func (h *DelegationHandlers) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	var req AuthorizeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validar endereço do contrato
	if !common.IsHexAddress(req.ContractAddress) {
		http.Error(w, "Invalid contract address", http.StatusBadRequest)
		return
	}
	contractAddr := common.HexToAddress(req.ContractAddress)

	// Validar e processar chave privada
	privateKey, err := parsePrivateKey(strings.TrimPrefix(req.SignerPK, "0x"))
	if err != nil {
		http.Error(w, "Invalid private key", http.StatusBadRequest)
		return
	}

	// Criar autorização
	auth, err := h.svc.SignDelegation(contractAddr, privateKey)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create authorization: %v", err), http.StatusInternalServerError)
		return
	}

	// Retornar autorização
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"authorization":  auth,
		"signer_address": crypto.PubkeyToAddress(privateKey.PublicKey).Hex(),
	})
}

// handleSponsor executa uma transação patrocinada
func (h *DelegationHandlers) handleSponsor(w http.ResponseWriter, r *http.Request) {
	var req SponsorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validar chave privada do sponsor
	sponsorPK, err := parsePrivateKey(strings.TrimPrefix(req.SponsorPK, "0x"))
	if err != nil {
		http.Error(w, "Invalid sponsor private key", http.StatusBadRequest)
		return
	}

	sponsorAddr := crypto.PubkeyToAddress(sponsorPK.PublicKey)
	fmt.Printf("Sponsor address: %s\n", sponsorAddr.Hex())
	fmt.Printf("Signer address: %s\n", req.Authorization.Signer.Hex())

	// Converter CallData para Call
	calls := make([]Call, len(req.Calls))
	for i, callData := range req.Calls {
		// Validar endereço
		if !common.IsHexAddress(callData.To) {
			http.Error(w, fmt.Sprintf("Invalid address in call %d", i), http.StatusBadRequest)
			return
		}

		// Converter dados hex
		data, err := hex.DecodeString(strings.TrimPrefix(callData.Data, "0x"))
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid data in call %d", i), http.StatusBadRequest)
			return
		}

		// Converter valor
		value, ok := new(big.Int).SetString(callData.Value, 10)
		if !ok {
			value = big.NewInt(0)
		}

		calls[i] = Call{
			To:    common.HexToAddress(callData.To),
			Data:  data,
			Value: value,
		}

		fmt.Printf("Call %d: to=%s, dataLen=%d\n", i, calls[i].To.Hex(), len(calls[i].Data))
	}

	// Executar transação patrocinada
	tx, err := h.svc.ExecuteSponsored(&req.Authorization, calls, sponsorPK)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to execute sponsored transaction: %v", err), http.StatusInternalServerError)
		return
	}

	fmt.Printf("Transaction created: %s\n", tx.Hash().Hex())

	// Enviar transação para a rede
	if err := h.svc.RPC.SendTransaction(tx); err != nil {
		http.Error(w, fmt.Sprintf("Failed to send transaction: %v", err), http.StatusInternalServerError)
		return
	}

	fmt.Printf("Transaction sent successfully: %s\n", tx.Hash().Hex())

	// Retornar hash da transação
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tx_hash": tx.Hash().Hex(),
		"sponsor": sponsorAddr.Hex(),
	})
}

// handleBuildSendETH - Helper para construir call data de sendETH
func (h *DelegationHandlers) handleBuildSendETH(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Recipient string `json:"recipient"`
		Amount    string `json:"amount"` // em ETH, ex: "1.5"
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if !common.IsHexAddress(req.Recipient) {
		http.Error(w, "Invalid recipient address", http.StatusBadRequest)
		return
	}

	builder := &CallDataBuilder{}
	amount := EtherToWei(req.Amount)
	callData := builder.SendETHDirectly(common.HexToAddress(req.Recipient), amount)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"call_data":  callData,
		"function":   "sendETH",
		"recipient":  req.Recipient,
		"amount":     req.Amount,
		"amount_wei": amount.String(),
	})
}

// handleBuildMint - Helper para construir call data de mint
func (h *DelegationHandlers) handleBuildMint(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Recipient string `json:"recipient"` // Quem recebe os tokens
		Amount    string `json:"amount"`    // Quantidade (ex: "100")
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if !common.IsHexAddress(req.Recipient) {
		http.Error(w, "Invalid recipient address", http.StatusBadRequest)
		return
	}

	builder := &CallDataBuilder{}
	amount := TokenAmountToWei(req.Amount, 18)
	tokenAddr := common.HexToAddress(TokenContract)
	callData := builder.Mint(tokenAddr, common.HexToAddress(req.Recipient), amount)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"call_data":     callData,
		"function":      "mint",
		"token_address": TokenContract,
		"recipient":     req.Recipient,
		"amount":        req.Amount,
		"amount_wei":    amount.String(),
	})
}

// handleBuildTransfer - Helper para construir call data de transfer
func (h *DelegationHandlers) handleBuildTransfer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Recipient string `json:"recipient"` // Quem recebe os tokens
		Amount    string `json:"amount"`    // Quantidade (ex: "100")
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if !common.IsHexAddress(req.Recipient) {
		http.Error(w, "Invalid recipient address", http.StatusBadRequest)
		return
	}

	builder := &CallDataBuilder{}
	amount := TokenAmountToWei(req.Amount, 18)
	tokenAddr := common.HexToAddress(TokenContract)
	callData := builder.Transfer(tokenAddr, common.HexToAddress(req.Recipient), amount)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"call_data":     callData,
		"function":      "transfer",
		"token_address": TokenContract,
		"recipient":     req.Recipient,
		"amount":        req.Amount,
		"amount_wei":    amount.String(),
	})
}

// handleSponsorETH - USANDO STRUCT REUTILIZÁVEL
func (h *DelegationHandlers) handleSponsorETH(w http.ResponseWriter, r *http.Request) {
	var req BasicSponsorRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), 400)
		return
	}

	if err := req.Validate(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	sk, err := parsePrivateKey(req.SignerPK)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid signer private key: %v", err), 400)
		return
	}

	sp, err := parsePrivateKey(req.SponsorPK)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid sponsor private key: %v", err), 400)
		return
	}

	if h.svc == nil {
		http.Error(w, "Service not initialized", 500)
		return
	}

	auth, err := h.svc.SignDelegation(common.HexToAddress(DelegateContract), sk)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create authorization: %v", err), 500)
		return
	}

	val := EtherToWei(req.Amount)
	cd := (&CallDataBuilder{}).SendETHDirectly(common.HexToAddress(req.Recipient), val)

	call := Call{
		To:    auth.Signer,
		Data:  common.Hex2Bytes(strings.TrimPrefix(cd, "0x")),
		Value: val,
	}

	tx, err := h.svc.ExecuteSponsored(auth, []Call{call}, sp)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to execute sponsored transaction: %v", err), 500)
		return
	}

	if err := h.svc.RPC.SendTransaction(tx); err != nil {
		http.Error(w, fmt.Sprintf("Failed to send transaction: %v", err), 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tx_hash":    tx.Hash().Hex(),
		"operation":  "sendETH",
		"amount":     req.Amount,
		"amount_wei": val.String(),
		"recipient":  req.Recipient,
	})
}

// handleSponsorToken - fluxo completo para enviar tokens ERC20 patrocinados
func (h *DelegationHandlers) handleSponsorToken(w http.ResponseWriter, r *http.Request) {
	// Implemente o handler para enviar tokens ERC20 patrocinados
	// Este é um espaço reservado e deve ser implementado
	http.Error(w, "Sponsor token not implemented", http.StatusNotImplemented)
}

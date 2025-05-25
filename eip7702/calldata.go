package eip7702

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"reflect"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// ===== CONSTANTES PADRONIZADAS =====
const (
	TokenContract    = "0x93d77bE58A977350B924C0694242b075eB26AEdE"
	DelegateContract = "0x1f0F9d7e19991e7E296630DC0073610f23CF066a"
)

// ===== ABI DO SIMPLE DELEGATE CONTRACT =====
var simpleDelegateABI abi.ABI

func init() {
	const abiJSON = `[
		{
			"name": "execute",
			"type": "function",
			"stateMutability": "payable",
			"inputs": [{
				"name": "calls",
				"type": "tuple[]",
				"components": [
					{"name": "data", "type": "bytes"},
					{"name": "to", "type": "address"},
					{"name": "value", "type": "uint256"}
				]
			}]
		},
		{
			"name": "mint",
			"type": "function",
			"inputs": [
				{"name": "token", "type": "address"},
				{"name": "to", "type": "address"},
				{"name": "amount", "type": "uint256"}
			]
		},
		{
			"name": "transfer",
			"type": "function",
			"inputs": [
				{"name": "token", "type": "address"},
				{"name": "to", "type": "address"},
				{"name": "amount", "type": "uint256"}
			]
		},
		{
			"name": "sendETH",
			"type": "function",
			"inputs": [
				{"name": "to", "type": "address"},
				{"name": "amount", "type": "uint256"}
			]
		}
	]`

	var err error
	simpleDelegateABI, err = abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		panic(fmt.Sprintf("Failed to parse ABI: %v", err))
	}
}

// CallDataBuilder usando ABI correto
type CallDataBuilder struct{}

// Mint - CORRIGIDO para SimpleDelegateContract.mint(address,address,uint256)
func (c *CallDataBuilder) Mint(tokenAddr, to common.Address, amount *big.Int) string {
	// mint(address token, address to, uint256 amount) DO SimpleDelegateContract
	selector := crypto.Keccak256([]byte("mint(address,address,uint256)"))[:4]

	tokenPadded := common.LeftPadBytes(tokenAddr.Bytes(), 32)
	toPadded := common.LeftPadBytes(to.Bytes(), 32)
	amountPadded := common.LeftPadBytes(amount.Bytes(), 32)

	data := append(selector, tokenPadded...)
	data = append(data, toPadded...)
	data = append(data, amountPadded...)

	return "0x" + hex.EncodeToString(data)
}

// Transfer - SimpleDelegateContract.transfer(address,address,uint256)
func (c *CallDataBuilder) Transfer(tokenAddr, to common.Address, amount *big.Int) string {
	// transfer(address token, address to, uint256 amount) DO SimpleDelegateContract
	selector := crypto.Keccak256([]byte("transfer(address,address,uint256)"))[:4]

	tokenPadded := common.LeftPadBytes(tokenAddr.Bytes(), 32)
	toPadded := common.LeftPadBytes(to.Bytes(), 32)
	amountPadded := common.LeftPadBytes(amount.Bytes(), 32)

	data := append(selector, tokenPadded...)
	data = append(data, toPadded...)
	data = append(data, amountPadded...)

	return "0x" + hex.EncodeToString(data)
}

// SendETH -
func (c *CallDataBuilder) SendETHDirectly(to common.Address, amount *big.Int) string {
	// sendETH(address to, uint256 amount) DO SimpleDelegateContract
	selector := crypto.Keccak256([]byte("sendETH(address,uint256)"))[:4]

	addressPadded := common.LeftPadBytes(to.Bytes(), 32)
	amountPadded := common.LeftPadBytes(amount.Bytes(), 32)

	data := append(selector, addressPadded...)
	data = append(data, amountPadded...)

	return "0x" + hex.EncodeToString(data)
}

// Execute - para multicall
func (c *CallDataBuilder) ExecuteCalls(calls []Call) string {
	// Converter para struct que o ABI espera
	type ABICall struct {
		Data  []byte
		To    common.Address
		Value *big.Int
	}

	abiCalls := make([]ABICall, len(calls))
	for i, call := range calls {
		abiCalls[i] = ABICall{
			Data:  call.Data,
			To:    call.To,
			Value: call.Value,
		}
	}

	data, err := simpleDelegateABI.Pack("execute", abiCalls)
	if err != nil {
		panic(fmt.Sprintf("Failed to pack execute: %v", err))
	}
	return "0x" + hex.EncodeToString(data)
}

// BuildGenericCall constrói call data para qualquer função
func (c *CallDataBuilder) BuildGenericCall(functionSig string, params []interface{}) (string, error) {
	// CASO ESPECIAL: execute((bytes,address,uint256)[])
	if functionSig == "execute((bytes,address,uint256)[])" {
		return c.buildExecuteCall(params)
	}

	// Calcular function selector
	selector := crypto.Keccak256([]byte(functionSig))[:4]

	// Encode parâmetros
	encodedParams, err := c.encodeParameters(params)
	if err != nil {
		return "", err
	}

	// Combine selector + params
	data := append(selector, encodedParams...)

	return "0x" + hex.EncodeToString(data), nil
}

// buildExecuteCall - Tratamento especial para execute((bytes,address,uint256)[])
func (c *CallDataBuilder) buildExecuteCall(params []interface{}) (string, error) {
	if len(params) != 1 {
		return "", fmt.Errorf("execute expects exactly 1 parameter (array of calls)")
	}

	// O primeiro parâmetro deve ser um array
	arrayParam, ok := params[0].([]interface{})
	if !ok {
		return "", fmt.Errorf("execute parameter must be an array")
	}

	// Converter para []Call
	calls := make([]Call, len(arrayParam))
	for i, callInterface := range arrayParam {
		callMap, ok := callInterface.(map[string]interface{})
		if !ok {
			return "", fmt.Errorf("call %d must be an object", i)
		}

		// Extrair campos
		dataStr, ok := callMap["data"].(string)
		if !ok {
			return "", fmt.Errorf("call %d: 'data' must be a string", i)
		}

		toStr, ok := callMap["to"].(string)
		if !ok {
			return "", fmt.Errorf("call %d: 'to' must be a string", i)
		}

		valueStr, ok := callMap["value"].(string)
		if !ok {
			return "", fmt.Errorf("call %d: 'value' must be a string", i)
		}

		// Converter valores
		var data []byte
		if dataStr != "0x" && dataStr != "" {
			var err error
			data, err = hex.DecodeString(strings.TrimPrefix(dataStr, "0x"))
			if err != nil {
				return "", fmt.Errorf("call %d: invalid data hex: %v", i, err)
			}
		}

		if !common.IsHexAddress(toStr) {
			return "", fmt.Errorf("call %d: invalid address: %s", i, toStr)
		}

		value, ok := new(big.Int).SetString(valueStr, 10)
		if !ok {
			return "", fmt.Errorf("call %d: invalid value: %s", i, valueStr)
		}

		calls[i] = Call{
			Data:  data,
			To:    common.HexToAddress(toStr),
			Value: value,
		}
	}

	// Usar a função existente
	return c.ExecuteCalls(calls), nil
}

// encodeParameters codifica parâmetros para ABI
func (c *CallDataBuilder) encodeParameters(params []interface{}) ([]byte, error) {
	var result []byte

	for _, param := range params {
		// Verificar se é array
		if reflect.TypeOf(param).Kind() == reflect.Slice {
			return nil, fmt.Errorf("arrays not supported in generic encoding, use specific functions")
		}

		encoded, err := c.encodeParameter(param)
		if err != nil {
			return nil, err
		}
		result = append(result, encoded...)
	}

	return result, nil
}

// encodeParameter codifica um parâmetro individual
func (c *CallDataBuilder) encodeParameter(param interface{}) ([]byte, error) {
	switch v := param.(type) {
	case string:
		// Se é um endereço hex
		if common.IsHexAddress(v) {
			addr := common.HexToAddress(v)
			return common.LeftPadBytes(addr.Bytes(), 32), nil
		}
		// Se é um número em string
		if num, ok := new(big.Int).SetString(v, 10); ok {
			return common.LeftPadBytes(num.Bytes(), 32), nil
		}
		// Se é hex data
		if strings.HasPrefix(v, "0x") {
			data, err := hex.DecodeString(v[2:])
			if err != nil {
				return nil, err
			}
			return common.LeftPadBytes(data, 32), nil
		}
		return nil, fmt.Errorf("unsupported string format: %s", v)

	case int, int8, int16, int32, int64:
		val := reflect.ValueOf(v).Int()
		bigInt := big.NewInt(val)
		return common.LeftPadBytes(bigInt.Bytes(), 32), nil

	case uint, uint8, uint16, uint32, uint64:
		val := reflect.ValueOf(v).Uint()
		bigInt := new(big.Int).SetUint64(val)
		return common.LeftPadBytes(bigInt.Bytes(), 32), nil

	case *big.Int:
		return common.LeftPadBytes(v.Bytes(), 32), nil

	case common.Address:
		return common.LeftPadBytes(v.Bytes(), 32), nil

	case bool:
		if v {
			return common.LeftPadBytes([]byte{1}, 32), nil
		}
		return common.LeftPadBytes([]byte{0}, 32), nil

	case []interface{}:
		return nil, fmt.Errorf("arrays not supported, use specific functions")

	default:
		return nil, fmt.Errorf("unsupported parameter type: %T", param)
	}
}

// EtherToWei converte ETH para Wei
func EtherToWei(eth string) *big.Int {
	ethFloat := new(big.Float)
	ethFloat.SetString(eth)

	weiFloat := new(big.Float)
	weiFloat.Mul(ethFloat, big.NewFloat(1e18))

	wei, _ := weiFloat.Int(nil)
	return wei
}

// TokenAmountToWei converte quantidade de token para unidades base
func TokenAmountToWei(amount string, decimals int) *big.Int {
	amountFloat := new(big.Float)
	amountFloat.SetString(amount)

	multiplier := new(big.Float)
	multiplier.SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil))

	weiFloat := new(big.Float)
	weiFloat.Mul(amountFloat, multiplier)

	wei, _ := weiFloat.Int(nil)
	return wei
}

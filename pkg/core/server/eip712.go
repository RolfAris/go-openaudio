package server

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	v1 "github.com/OpenAudio/go-openaudio/pkg/api/core/v1"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
)

const (
	ProdRegistryAddress  = "0xd976d3b4f4e22a238c1A736b6612D22f17b6f64C"
	StageRegistryAddress = "0xc682C2166E11690B64338e11633Cb8Bb60B0D9c0"
	DevRegistryAddress   = "0xABbfF712977dB51f9f212B85e8A4904c818C2b63"

	ProdAcdcAddress  = "0x1Cd8a543596D499B9b6E7a6eC15ECd2B7857Fd64"
	StageAcdcAddress = "0x1Cd8a543596D499B9b6E7a6eC15ECd2B7857Fd64"
	DevAcdcAddress   = "0x254dffcd3277C0b1660F6d42EFbB754edaBAbC2B"

	ProdAcdcChainID  = 31524
	StageAcdcChainID = 1056801
	DevAcdcChainID   = 1337
)

func DeterministicEntityManagerAddressAndChainID(chainID string) (string, uint64) {
	switch chainID {
	case "mainnet-alpha-beta":
		return ProdAcdcAddress, ProdAcdcChainID
	case "audius-testnet-alpha":
		return StageAcdcAddress, StageAcdcChainID
	case "openaudio-devnet":
		return DevAcdcAddress, DevAcdcChainID
	}

	h := sha256.Sum256([]byte(chainID))

	// Address
	addrBytes := crypto.Keccak256(h[:])
	address := fmt.Sprintf("0x%x", addrBytes[12:])

	// Chain ID
	v := new(big.Int).SetBytes(h[:])
	space := big.NewInt(1_000_000_000)
	mod := new(big.Int).Mod(v, space)
	prefix := big.NewInt(440_000_000_000)
	out := new(big.Int).Add(prefix, mod)

	return address, out.Uint64()
}

func InjectSigner(entityManagerAddress string, chainId uint64, em *v1.ManageEntityLegacy) error {
	address, _, err := RecoverPubkeyFromCoreTx(entityManagerAddress, chainId, em)
	if err != nil {
		return err
	}

	em.Signer = address
	return nil
}

func RecoverPubkeyFromCoreTx(contractAddress string, chainId uint64, em *v1.ManageEntityLegacy) (string, *ecdsa.PublicKey, error) {
	var nonce [32]byte
	copy(nonce[:], toBytes(em.Nonce))

	var typedData = apitypes.TypedData{
		Types: apitypes.Types{
			"EIP712Domain": []apitypes.Type{
				{
					Name: "name",
					Type: "string",
				},
				{
					Name: "version",
					Type: "string",
				},
				{
					Name: "chainId",
					Type: "uint256",
				},
				{
					Name: "verifyingContract",
					Type: "address",
				},
			},
			"ManageEntity": []apitypes.Type{
				{
					Name: "userId",
					Type: "uint",
				},
				{
					Name: "entityType",
					Type: "string",
				},
				{
					Name: "entityId",
					Type: "uint",
				},
				{
					Name: "action",
					Type: "string",
				},
				{
					Name: "metadata",
					Type: "string",
				},
				{
					Name: "nonce",
					Type: "bytes32",
				},
			},
		},
		Domain: apitypes.TypedDataDomain{
			Name:              "Entity Manager",
			Version:           "1",
			ChainId:           math.NewHexOrDecimal256(int64(chainId)),
			VerifyingContract: contractAddress,
		},
		PrimaryType: "ManageEntity",
		Message: map[string]interface{}{
			"userId":     fmt.Sprintf("%d", em.UserId),
			"entityType": em.EntityType,
			"entityId":   fmt.Sprintf("%d", em.EntityId),
			"action":     em.Action,
			"metadata":   em.Metadata,
			"nonce":      nonce,
		},
	}

	pubkeyBytes, err := recoverPublicKey(toBytes(em.Signature), typedData)
	if err != nil {
		return "", nil, err
	}

	pubkey, err := crypto.UnmarshalPubkey(pubkeyBytes)
	if err != nil {
		return "", nil, err
	}

	address := crypto.PubkeyToAddress(*pubkey).String()
	return address, pubkey, nil
}

func toBytes(str string) []byte {
	v, _ := hex.DecodeString(strings.TrimPrefix(str, "0x"))
	return v
}

// taken from:
// https://gist.github.com/APTy/f2a6864a97889793c587635b562c7d72#file-main-go
func recoverPublicKey(signature []byte, typedData apitypes.TypedData) ([]byte, error) {

	domainSeparator, err := typedData.HashStruct("EIP712Domain", typedData.Domain.Map())
	if err != nil {
		return nil, fmt.Errorf("eip712domain hash struct: %w", err)
	}

	typedDataHash, err := typedData.HashStruct(typedData.PrimaryType, typedData.Message)
	if err != nil {
		return nil, fmt.Errorf("primary type hash struct: %w", err)
	}

	// add magic string prefix
	rawData := []byte(fmt.Sprintf("\x19\x01%s%s", string(domainSeparator), string(typedDataHash)))
	sighash := crypto.Keccak256(rawData)

	// update the recovery id
	// https://github.com/ethereum/go-ethereum/blob/55599ee95d4151a2502465e0afc7c47bd1acba77/internal/ethapi/api.go#L442
	if len(signature) > 64 {
		signature[64] -= 27
	}

	return crypto.Ecrecover(sighash, signature)

}

// SignManageEntity creates an EIP712 signature for a ManageEntityLegacy message and sets it on the message
func SignManageEntity(contractAddress string, chainId uint, em *v1.ManageEntityLegacy, privateKey *ecdsa.PrivateKey) error {
	var nonce [32]byte
	copy(nonce[:], toBytes(em.Nonce))

	var typedData = apitypes.TypedData{
		Types: apitypes.Types{
			"EIP712Domain": []apitypes.Type{
				{
					Name: "name",
					Type: "string",
				},
				{
					Name: "version",
					Type: "string",
				},
				{
					Name: "chainId",
					Type: "uint256",
				},
				{
					Name: "verifyingContract",
					Type: "address",
				},
			},
			"ManageEntity": []apitypes.Type{
				{
					Name: "userId",
					Type: "uint",
				},
				{
					Name: "entityType",
					Type: "string",
				},
				{
					Name: "entityId",
					Type: "uint",
				},
				{
					Name: "action",
					Type: "string",
				},
				{
					Name: "metadata",
					Type: "string",
				},
				{
					Name: "nonce",
					Type: "bytes32",
				},
			},
		},
		Domain: apitypes.TypedDataDomain{
			Name:              "Entity Manager",
			Version:           "1",
			ChainId:           math.NewHexOrDecimal256(int64(chainId)),
			VerifyingContract: contractAddress,
		},
		PrimaryType: "ManageEntity",
		Message: map[string]interface{}{
			"userId":     fmt.Sprintf("%d", em.UserId),
			"entityType": em.EntityType,
			"entityId":   fmt.Sprintf("%d", em.EntityId),
			"action":     em.Action,
			"metadata":   em.Metadata,
			"nonce":      nonce,
		},
	}

	domainSeparator, err := typedData.HashStruct("EIP712Domain", typedData.Domain.Map())
	if err != nil {
		return fmt.Errorf("eip712domain hash struct: %w", err)
	}

	typedDataHash, err := typedData.HashStruct(typedData.PrimaryType, typedData.Message)
	if err != nil {
		return fmt.Errorf("primary type hash struct: %w", err)
	}

	// add magic string prefix
	rawData := []byte(fmt.Sprintf("\x19\x01%s%s", string(domainSeparator), string(typedDataHash)))
	sighash := crypto.Keccak256(rawData)

	// Sign the hash
	signature, err := crypto.Sign(sighash, privateKey)
	if err != nil {
		return fmt.Errorf("failed to sign: %w", err)
	}

	// Adjust recovery id for Ethereum
	signature[64] += 27

	em.Signature = "0x" + hex.EncodeToString(signature)
	return nil
}

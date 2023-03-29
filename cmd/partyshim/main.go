package main

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"

	wgrams "github.com/TeaPartyCrypto/PartyShim/wrappedgrams"
)

func main() {
	// import the private key from the environment variable
	privateKey := os.Getenv("PRIVATE_KEY")
	if privateKey == "" {
		panic("PRIVATE_KEY environment variable not set")
	}
	RPCURL := os.Getenv("RPC_URL")
	if RPCURL == "" {
		panic("RPC_URL environment variable not set")
	}
	ContractAddress := os.Getenv("CONTRACT_ADDRESS")
	if ContractAddress == "" {
		panic("CONTRACT_ADDRESS environment variable not set")
	}

	// create a new ecdsa private key from the plain text private key
	pkECDSA, err := crypto.HexToECDSA(privateKey)
	if err != nil {
		fmt.Println(err)
	}

	ps := &PartyShim{
		privateKey:      pkECDSA,
		RPCURL:          RPCURL,
		ContractAddress: ContractAddress,
	}

	// start an http server
	http.HandleFunc("/mint", ps.mint)
	http.HandleFunc("/transfer", ps.transfer)
	http.ListenAndServe(":8080", nil)
}

// mint will sign a transaction and return the signed transaction
func (e *PartyShim) mint(w http.ResponseWriter, r *http.Request) {
	mintRequest := &MintRequest{}
	// decode the request body into the MintRequest struct
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(mintRequest)
	if err != nil {
		fmt.Println(err)
	}

	// Contract owners, feel free to add additional logic here
	// to farther validate the transaction before signing it
	// Just notify me if you do.

	// mint the transaction
	err = e.completeMint(*mintRequest)
	if err != nil {
		fmt.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
	}

	// return the signed transaction
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(mintRequest)
}

// transfer will sign a transaction and return the signed transaction
func (e *PartyShim) transfer(w http.ResponseWriter, r *http.Request) {
	transferRequest := &MintRequest{}
	// decode the request body into the MintRequest struct
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(transferRequest)
	if err != nil {
		fmt.Println(err)
	}

	// Contract owners, feel free to add additional logic here
	// to farther validate the transaction before signing it
	// Just notify me if you do.

	// Complete the transfer
	txid, err := e.completeTransfer(*transferRequest)
	if err != nil {
		fmt.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
	}

	// return the transaction id
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(txid)
}

func (e *PartyShim) completeTransfer(mr MintRequest) (*string, error) {
	// Contract owners, feel free to add additional logic here
	// to farther validate the transaction before signing it
	// Just notify me if you do.

	ctx := context.Background()
	// initialize the Party Chain nodes.
	partyclient, err := ethclient.Dial(e.RPCURL)
	if err != nil {
		return nil, err
	}

	publicKey := e.privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("error casting public key to ECDSA")
	}

	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)
	nonce, err := partyclient.PendingNonceAt(ctx, fromAddress)
	if err != nil {
		return nil, err
	}

	gasPrice, err := partyclient.SuggestGasPrice(ctx)
	if err != nil {
		return nil, err
	}
	gasLimit := uint64(21000)

	// set chain id
	chainID, err := partyclient.ChainID(ctx)
	if err != nil {
		return nil, err
	}
	toAddress := common.HexToAddress(mr.ToAddress)
	var data []byte
	tx := types.NewTransaction(nonce, toAddress, &mr.Amount, gasLimit, gasPrice, data)

	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), e.privateKey)
	if err != nil {
		return nil, err
	}

	if err := partyclient.SendTransaction(ctx, signedTx); err != nil {
		return nil, err
	}

	fmt.Printf("tx sent: %s", signedTx.Hash().Hex())
	transactionID := signedTx.Hash().Hex()

	return &transactionID, nil
}

// completeMint will complete the minting of the transaction
func (e *PartyShim) completeMint(mr MintRequest) error {
	ctx := context.Background()
	// initialize the Party Chain nodes.
	partyclient, err := ethclient.Dial(e.RPCURL)
	if err != nil {
		log.Fatal(err)
	}

	publicKey := e.privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		log.Fatal("error casting public key to ECDSA")
	}

	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)
	nonce, err := partyclient.PendingNonceAt(ctx, fromAddress)
	if err != nil {
		log.Fatal(err)
	}

	gasPrice, err := partyclient.SuggestGasPrice(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// set chain id
	chainID, err := partyclient.ChainID(ctx)
	if err != nil {
		log.Fatal(err)
	}

	auth, err := bind.NewKeyedTransactorWithChainID(e.privateKey, chainID)
	if err != nil {
		log.Fatal(err)
	}
	auth.Nonce = big.NewInt(int64(nonce))
	auth.Value = big.NewInt(0)      // in wei
	auth.GasLimit = uint64(3000000) // in units
	auth.GasPrice = gasPrice
	auth.From = fromAddress

	contractaddress := common.HexToAddress(e.ContractAddress)
	instance, err := wgrams.NewBe(contractaddress, partyclient)
	if err != nil {
		log.Fatal(err)
	}

	tx, err := instance.Mint(auth, common.HexToAddress(mr.ToAddress), &mr.Amount)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("tx sent: %s", tx.Hash().Hex())

	return nil
}

type MintRequest struct {
	ToAddress string  `json:"toAddress"`
	Amount    big.Int `json:"amount"`
}

type PartyShim struct {
	// the private key of the contract owner
	privateKey      *ecdsa.PrivateKey
	RPCURL          string
	ContractAddress string
}

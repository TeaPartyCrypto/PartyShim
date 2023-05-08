package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"

	bridge "github.com/TeaPartyCrypto/PartyShim/contract/v2"
)

type MintRequest struct {
	ToAddress string   `json:"toAddress"`
	Amount    *big.Int `json:"amount"`
	FromPK    string   `json:"fromPK"`
}

type PartyShim struct {
	// the private key of the contract owner
	privateKey        *ecdsa.PrivateKey
	defaultPaymentKey *ecdsa.PrivateKey
	RPCURL            string
	RPCURL2           string
	ContractAddress   string
}

func main() {
	// import the private key from the environment variable
	privateKey := os.Getenv("PRIVATE_KEY")
	if privateKey == "" {
		panic("PRIVATE_KEY environment variable not set")
	}
	defaultPaymentPK := os.Getenv("DEFAULT_PAYMENT_PRIVATE_KEY")
	if defaultPaymentPK == "" {
		panic("DEFAULT_PAYMENT_PRIVATE_KEY environment variable not set")
	}
	RPCURL := os.Getenv("RPC_URL")
	if RPCURL == "" {
		panic("RPC_URL environment variable not set")
	}
	RPCURL2 := os.Getenv("RPC_URL2")
	if RPCURL2 == "" {
		panic("RPC_URL2 environment variable not set")
	}
	ContractAddress := os.Getenv("CONTRACT_ADDRESS")
	if ContractAddress == "" {
		panic("CONTRACT_ADDRESS environment variable not set")
	}
	CACertLocation := os.Getenv("SHIM_CA_CERT")
	if CACertLocation == "" {
		panic("SHIM_CA_CERT environment variable not set")
	}

	// create a new ecdsa private key from the plain text private key
	pkECDSA, err := crypto.HexToECDSA(privateKey)
	if err != nil {
		fmt.Println(err)
	}

	// create a new ecdsa private key from the plain text private key
	defaultPaymentPKECDSA, err := crypto.HexToECDSA(defaultPaymentPK)
	if err != nil {
		fmt.Println(err)
	}

	ps := &PartyShim{
		privateKey:        pkECDSA,
		defaultPaymentKey: defaultPaymentPKECDSA,
		RPCURL:            RPCURL,
		RPCURL2:           RPCURL2,
		ContractAddress:   ContractAddress,
	}
	// Read the certificate and private key files
	cert, err := tls.LoadX509KeyPair(CACertLocation+"/server.crt", CACertLocation+"/server.key")
	if err != nil {
		log.Fatalf("failed to load certificate and private key: %v", err)
	}

	// Load the CA certificate used to sign the client certificates.
	caCert, err := ioutil.ReadFile(CACertLocation + "/ca.crt")
	if err != nil {
		log.Fatalf("failed to read CA certificate: %v", err)
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	// Configure TLS options.
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		// ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs: caCertPool,
	}

	// Create a server with the TLS configuration
	server := &http.Server{
		Addr:      ":8080",
		Handler:   nil,
		TLSConfig: tlsConfig,
	}

	// Register the HTTP handlers
	http.HandleFunc("/mint", ps.mint)
	http.HandleFunc("/transfer", ps.transfer)

	fmt.Println("Starting shim on port 8080")
	// Start the HTTPS server with TLS
	log.Fatal(server.ListenAndServeTLS("", ""))
}

// mint exposes an interface to mint the wrapped currency
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
	err, txid := e.completeMint(*mintRequest)
	if err != nil {
		fmt.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
	}

	// return the signed transaction
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(txid)
}

// completeMint will complete the minting of the wrapped currency
func (e *PartyShim) completeMint(mr MintRequest) (error, string) {
	ctx := context.Background()
	// initialize the Party Chain nodes.
	partyclient, err := ethclient.Dial(e.RPCURL)
	if err != nil {
		return err, ""
	}

	publicKey := e.privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return err, ""
	}

	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)
	nonce, err := partyclient.PendingNonceAt(ctx, fromAddress)
	if err != nil {
		return err, ""
	}

	gasPrice, err := partyclient.SuggestGasPrice(ctx)
	if err != nil {
		return err, ""
	}

	// set chain id
	chainID, err := partyclient.ChainID(ctx)
	if err != nil {
		return err, ""
	}

	auth, err := bind.NewKeyedTransactorWithChainID(e.privateKey, chainID)
	if err != nil {
		return err, ""
	}
	auth.Nonce = big.NewInt(int64(nonce))
	auth.Value = big.NewInt(0)     // in wei
	auth.GasLimit = uint64(100000) // in units
	auth.GasPrice = gasPrice
	auth.From = fromAddress

	contractaddress := common.HexToAddress(e.ContractAddress)
	instance, err := bridge.NewPartyBridge(contractaddress, partyclient)
	if err != nil {
		return err, ""
	}

	toadr := common.HexToAddress(mr.ToAddress)

	// Call the mint function on the contract
	tx, err := instance.Mint(auth, toadr, mr.Amount)
	if err != nil {
		return err, ""
	}

	fmt.Printf("tx sent: %s \n", tx.Hash().Hex())

	// wait for the transaction to be mined
	for pending := true; pending; _, pending, err = partyclient.TransactionByHash(ctx, tx.Hash()) {
		if err != nil {
			return err, ""
		}
		time.Sleep(2 * time.Second)
	}

	fmt.Println("tx mined")

	return nil, tx.Hash().Hex()
}

// transfer starts the un-wrapping process of a coin
func (e *PartyShim) transfer(w http.ResponseWriter, r *http.Request) {
	transferRequest := &MintRequest{}
	// decode the request body into the MintRequest struct
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(transferRequest)
	if err != nil {
		fmt.Println(err)
	}

	fmt.Printf("transfer request: %+v", transferRequest)

	// Contract owners, feel free to add additional logic here
	// to farther validate the transaction before signing it
	// Just notify me if you do.

	// var pk *ecdsa.PrivateKey
	// if transferRequest.FromPK != "" {
	// 	// convert the privateKey string to ecdsa.PrivateKey
	// 	pkECDSA, err := crypto.HexToECDSA(transferRequest.FromPK)
	// 	if err != nil {
	// 		fmt.Println(err)
	// 		w.WriteHeader(http.StatusInternalServerError)
	// 		w.Write([]byte(err.Error()))
	// 	}
	// 	pk = pkECDSA
	// } else {
	// 	pk = e.defaultPaymentKey
	// }

	// Complete the transfer
	err, txid := e.completeTransferByPayingMaxAmountPossibleFromAccount(*transferRequest)
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

func (e *PartyShim) completeTransfer(mr MintRequest, privateKey *ecdsa.PrivateKey) (error, *string) {
	// amount := new(big.Int)
	// twoPercent := new(big.Float).SetFloat64(0.98)
	// amountFloat := new(big.Float).SetInt(mr.Amount)
	// newAmountFloat := new(big.Float).Mul(amountFloat, twoPercent)

	// // Convert the result back to a big.Int
	// newAmountFloat.Int(amount)

	// fmt.Printf("Amount after deducting 2%%: %s\n", amount.String())

	// mr.Amount = amount

	ctx := context.Background()
	// initialize the Party Chain nodes.
	partyclient, err := ethclient.Dial(e.RPCURL2)
	if err != nil {
		return err, nil
	}

	// check the connection status of the ethclient
	i, err := partyclient.PeerCount(ctx)
	if err != nil {
		return err, nil
	}

	fmt.Println("Peer Count: ", i)

	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return errors.New("cannot assert type: publicKey is not of type *ecdsa.PublicKey"), nil
	}

	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)
	nonce, err := partyclient.PendingNonceAt(ctx, fromAddress)
	if err != nil {
		return err, nil
	}

	// Get suggested gas price
	gasPrice, err := partyclient.SuggestGasPrice(context.Background())
	if err != nil {
		return err, nil
	}

	toadr := common.HexToAddress(mr.ToAddress)

	// Estimate gas limit for the specific transaction
	msg := ethereum.CallMsg{
		From:     fromAddress,
		To:       &toadr,
		Value:    mr.Amount,
		Data:     []byte{},
		GasPrice: gasPrice,
	}

	estimatedGasLimit, err := partyclient.EstimateGas(ctx, msg)
	if err != nil {
		return err, nil
	}

	// Add a buffer to the estimated gas limit (e.g., 1%)
	bufferedGasLimit := uint64(float64(estimatedGasLimit) * 2.6)

	fmt.Println("estimated gas limit: ", estimatedGasLimit)
	fmt.Println("buffered gas limit: ", bufferedGasLimit)
	fmt.Println("gas price: ", gasPrice)

	// Calculate gas cost
	gasLimitBigInt := new(big.Int).SetUint64(bufferedGasLimit)
	gasPriceBigInt := new(big.Int).SetUint64(gasPrice.Uint64())
	gasCost := new(big.Int).Mul(gasLimitBigInt, gasPriceBigInt)

	fmt.Println("gas cost: ", gasCost)

	// Deduct gas cost from the amount
	newAmt := new(big.Int).Sub(mr.Amount, gasCost)
	fmt.Println("new amount: ", newAmt)

	if newAmt.Cmp(big.NewInt(0)) < 0 {
		fmt.Println("Insufficient funds, retrying with default payment private key")
		return e.completeTransferWithPrivateKey(mr)
	}

	// set chain id
	chainID, err := partyclient.ChainID(ctx)
	if err != nil {
		return err, nil
	}
	toAddress := common.HexToAddress(mr.ToAddress)
	var data []byte
	tx := types.NewTransaction(nonce, toAddress, newAmt, bufferedGasLimit, gasPrice, data)

	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), privateKey)
	if err != nil {
		return err, nil
	}

	err = partyclient.SendTransaction(ctx, signedTx)
	if err != nil {
		// Check if the error is due to insufficient funds
		if strings.Contains(err.Error(), "insufficient funds") && privateKey != e.defaultPaymentKey {
			// Get the balance of the fromAddress
			balance, err := partyclient.BalanceAt(ctx, fromAddress, nil)
			if err != nil {
				return err, nil
			}

			// Calculate the maximum possible amount to send, taking into account the gas cost
			maxAmount := new(big.Int).Sub(balance, gasCost)

			// Check if the maxAmount is greater than zero
			if maxAmount.Cmp(big.NewInt(0)) > 0 {
				fmt.Printf("Sending the maximum possible amount: %s\n", maxAmount.String())
				mr.Amount = maxAmount
				return e.completeTransfer(mr, privateKey)
			} else {
				fmt.Println("Insufficient funds, retrying with default payment private key")
				return e.completeTransferWithPrivateKey(mr)
			}
		}
		return err, nil
	}

	fmt.Printf("transfer tx sent: %s on chain id: %s to address: %s from address: %s", signedTx.Hash().Hex(), chainID.String(), toAddress.String(), fromAddress.String())
	transactionID := signedTx.Hash().Hex()

	// wait for the transaction to be mined
	for pending := true; pending; _, pending, err = partyclient.TransactionByHash(ctx, signedTx.Hash()) {
		if err != nil {
			return err, nil
		}
		time.Sleep(2 * time.Second)
	}

	fmt.Println("transfer tx mined")

	// burn the minted tokens
	err = e.burn(mr)
	if err != nil {
		return err, nil
	}

	return nil, &transactionID
}

// burn will remove the minted wrapped tokens from circulation
func (e *PartyShim) burn(mr MintRequest) error {
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

	// initialize the contract
	contract, err := bridge.NewPartyBridge(common.HexToAddress(e.ContractAddress), partyclient)
	if err != nil {
		log.Fatal(err)
	}

	// burn the tokens
	tx, err := contract.Burn(auth, common.HexToAddress(mr.ToAddress), mr.Amount)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("burn tx sent: %s", tx.Hash().Hex())

	return nil
}

// completeTransferWithPrivateKey
func (e *PartyShim) completeTransferWithPrivateKey(mr MintRequest) (error, *string) {
	ctx := context.Background()
	// initialize the Party Chain nodes.
	partyclient, err := ethclient.Dial(e.RPCURL2)
	if err != nil {
		return err, nil
	}

	// check the connection status of the ethclinet
	i, err := partyclient.PeerCount(ctx)
	if err != nil {
		return err, nil
	}

	fmt.Println("Peer Count: ", i)

	privateKey := e.defaultPaymentKey
	publicKey := e.defaultPaymentKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return errors.New("cannot assert type: publicKey is not of type *ecdsa.PublicKey"), nil
	}

	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)
	nonce, err := partyclient.PendingNonceAt(ctx, fromAddress)
	if err != nil {
		return err, nil
	}

	// Get suggested gas price
	gasPrice, err := partyclient.SuggestGasPrice(context.Background())
	if err != nil {
		return err, nil
	}

	toadr := common.HexToAddress(mr.ToAddress)

	// Estimate gas limit for the specific transaction
	msg := ethereum.CallMsg{
		From:     fromAddress,
		To:       &toadr,
		Value:    mr.Amount,
		Data:     []byte{},
		GasPrice: gasPrice,
	}

	estimatedGasLimit, err := partyclient.EstimateGas(ctx, msg)
	if err != nil {
		return err, nil
	}

	// Add a buffer to the estimated gas limit (e.g., 10%)
	bufferedGasLimit := uint64(float64(estimatedGasLimit) * 1.1)

	// Calculate gas cost
	gasLimitBigInt := new(big.Int).SetUint64(bufferedGasLimit)
	gasPriceBigInt := new(big.Int).SetUint64(gasPrice.Uint64())
	gasCost := new(big.Int).Mul(gasLimitBigInt, gasPriceBigInt)

	// Deduct gas cost from the amount
	newAmt := new(big.Int).Sub(mr.Amount, gasCost)
	fmt.Println("new amount: ", newAmt)

	if newAmt.Cmp(big.NewInt(0)) < 0 {
		return errors.New("insufficient funds for gas * price + value"), nil
	}

	// set chain id
	chainID, err := partyclient.ChainID(ctx)
	if err != nil {
		return err, nil
	}
	toAddress := common.HexToAddress(mr.ToAddress)
	var data []byte
	tx := types.NewTransaction(nonce, toAddress, newAmt, bufferedGasLimit, gasPrice, data)

	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), privateKey)
	if err != nil {
		return err, nil
	}

	err = partyclient.SendTransaction(ctx, signedTx)
	if err != nil {
		// Check if the error is due to insufficient funds
		if strings.Contains(err.Error(), "insufficient funds for gas * price + value") && privateKey != e.defaultPaymentKey {
			return errors.New("insufficient funds for gas * price + value"), nil
		}
		return err, nil
	}

	fmt.Printf("transfer tx sent: %s on chain id: %s to address: %s from address: %s", signedTx.Hash().Hex(), chainID.String(), toAddress.String(), fromAddress.String())
	transactionID := signedTx.Hash().Hex()

	// wait for the transaction to be mined
	for pending := true; pending; _, pending, err = partyclient.TransactionByHash(ctx, signedTx.Hash()) {
		if err != nil {
			return err, nil
		}
		time.Sleep(2 * time.Second)
	}

	fmt.Println("transfer tx mined")

	// burn the minted tokens
	err = e.burn(mr)
	if err != nil {
		return err, nil
	}

	return nil, &transactionID
}

// completeTransferByPayingMaxAmountPossibleFromAccount
func (e *PartyShim) completeTransferByPayingMaxAmountPossibleFromAccount(mr MintRequest) (error, *string) {
	ctx := context.Background()
	// initialize the Party Chain nodes.
	partyclient, err := ethclient.Dial(e.RPCURL2)
	if err != nil {
		return err, nil
	}
	toAddress := common.HexToAddress(mr.ToAddress)
	nonce, err := partyclient.PendingNonceAt(ctx, toAddress)
	if err != nil {
		return err, nil
	}
	// set chain id
	chainID, err := partyclient.ChainID(ctx)
	if err != nil {
		return err, nil
	}

	// check the connection status of the ethclient
	i, err := partyclient.PeerCount(ctx)
	if err != nil {
		return err, nil
	}

	fmt.Println("Peer Count: ", i)

	// get the public address from the mr.FromPk field
	pkECDSA, err := crypto.HexToECDSA(mr.FromPK)
	if err != nil {
		return err, nil
	}

	fromAddress := crypto.PubkeyToAddress(pkECDSA.PublicKey)

	// Get suggested gas price
	gasPrice, err := partyclient.SuggestGasPrice(context.Background())
	if err != nil {
		return err, nil
	}

	estimatedGasLimit := uint64(29000)
	gasCost := new(big.Int).Mul(new(big.Int).SetUint64(estimatedGasLimit), gasPrice)
	// Calculate the maximum possible amount to send, taking into account the gas cost
	maxAmount := new(big.Int).Sub(mr.Amount, gasCost)

	fmt.Println("max amount: ", maxAmount)
	var data []byte
	tx := types.NewTransaction(nonce, toAddress, maxAmount, estimatedGasLimit, gasPrice, data)

	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), pkECDSA)
	if err != nil {
		return err, nil
	}

	err = partyclient.SendTransaction(ctx, signedTx)
	if err != nil {
		if strings.Contains(err.Error(), "insufficient funds") {
			// modify the value of the amount to be sent
			// by subtracting 10% from it
			// and try again
			tenPercent := new(big.Int).Mul(mr.Amount, big.NewInt(10))
			tenPercent = new(big.Int).Div(tenPercent, big.NewInt(100))
			newAmt := new(big.Int).Sub(mr.Amount, tenPercent)
			fmt.Println("new amount: ", newAmt)
			mr.Amount = newAmt
			return e.completeTransferByPayingMaxAmountPossibleFromAccount(mr)
		}
	}

	// wait for the transaction to be mined

	for pending := true; pending; _, pending, err = partyclient.TransactionByHash(ctx, signedTx.Hash()) {
		if err != nil {
			return err, nil
		}
		time.Sleep(2 * time.Second)
	}

	fmt.Printf("transfer tx sent: %s on chain id: %s to address: %s from address: %s", signedTx.Hash().Hex(), chainID.String(), toAddress.String(), fromAddress.String())
	transactionID := signedTx.Hash().Hex()

	fmt.Println("transfer tx mined")

	// check the status of the transaction
	receipt, err := partyclient.TransactionReceipt(ctx, signedTx.Hash())
	if err != nil {
		return err, nil
	}

	if receipt.Status == 0 {
		return errors.New("transaction failed"), nil
	}

	return nil, &transactionID
}

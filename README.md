# PartyShim

PartyShim exists as a layer to seperate ownership of bridge contracts while still exposing an interface that `PartyBridge` can access.


## Overview


## Configure and Deploy

There is a pre-built docker image located at `gcr.io/mineonlium/partyshim` that you can use. Or feel free to build your own container image. 

Choose one of the following:

1/ Kubernetes deployment and service manifests are located in `config/shim.yaml`

1/ Docker compose file located in the root. 

1/ Bear metal execution of the binary.

After selecting one of the following you will need to provide your runtime with several enviorment variables:
       
       * CONTRACT_ADDRESS - This should point to the deployed contract on the OUTSIDE network.
       * PRIVATE_KEY - The private key that deployed the contract on the OUTSIDE network. 
       * DEFAULT_PAYMENT_PRIVATE_KEY - The private key that will be used if no private key is provided by `PartyBridge` at event time.
       * RPC_URL - This should point a RPC server for the WRAPPED asset. 
       * RPC_URL2 - This should point to a RPC server for the NATIVE currency. 
       * SHIM_CA_CERT - This should contain the location of of both the `cert.pem` and `key_pkcs1.pem` files. (I.E. `/home/jeff/cert`)


## Integrate a New Chain

Follow the steps below to integrate a new chain with `PartyBridge`:

1/ Deploy the contract located at `contract/bridge.sol` to the new chain. 

1/ Update `contract/deployments.md` with the contract address.

1/ PR this repository with the changes. 


## Development

Example request to the `/mint` route: 
```
curl -v "https://0.0.0.0:8080/mint" \
       -X POST \
       -k \
       -H "Content-Type: application/json" \
       -d '{"amount": 10000, "toAddress":"0x5dd4039c32F6EEF427D6F67600D8920c9631D59D"}' \
       --cert ./client.crt \
       --key ./key.crt
```


Example request to the `/transfer` route:

```
curl -v "https://0.0.0.0:8080/transfer" \
       -X POST \
       -k \
       -H "Content-Type: application/json" \
       -d '{"amount": 10000, "toAddress":"0x9cA67FFE69698d963A393E9338aD3BcfD2CEa02e","fromPK":<pk of funders address>}' \
       --cert ./client.crt \
       --key ./client.key

curl -v "https://0.0.0.0:8080/transfer" \
       -X POST \
       -k \
       -H "Content-Type: application/json" \
       -d '{"amount": 10000, "toAddress":"0x5bbfa5724260Cb175cB39b24802A04c3bfe72eb3"}' \
       --cert ./client.crt \
       --key ./client.key
```


How to generate a self-signed certificate


openssl genrsa -out ca.key 2048
openssl req -new -x509 -days 3650 -key ca.key -out ca.crt
openssl genrsa -out server.key 2048
openssl req -new -key server.key -out server.csr
openssl x509 -req -days 365 -in server.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out server.crt
openssl genrsa -out client.key 2048
openssl req -new -key client.key -out client.csr
openssl x509 -req -days 365 -in client.csr -CA ca.crt -CAkey ca.key -CAcreateserial -out client.crt



add a known authority 

openssl req -new -key server.key -out server.csr \
  -subj "/C=US/ST=California/L=San Francisco/O=TeaPartyCrypto/OU=IT/CN=192.168.50.26" \
  -addext "subjectAltName=IP:192.168.50.26" \
  -addext "subjectAltName=IP:192.168.50.25" 


openssl x509 -req -in server.csr -CA ca.crt -CAkey ca.key \
  -CAcreateserial -out server.crt \
  -days 365 -sha256 \
  -extfile <(printf "subjectAltName=IP:192.168.50.26,IP:192.168.50.25")








base64 encode for the kubernetes secret

cat ca.crt | base64
cat client.crt | base64
cat client.key | base64
cat server.crt | base64
cat server.key | base64


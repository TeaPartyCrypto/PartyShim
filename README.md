export PRIVATE_KEY=0x123
export RPC_URL=
export CONTRACT_ADDRESS=


curl -v "http://0.0.0.0:8080/mint" \
       -X POST \
       -H "Content-Type: application/json" \
       -d '{"amount": 10000, "toAddress":"0x5dd4039c32F6EEF427D6F67600D8920c9631D59D"}'


curl -v "http://0.0.0.0:8080/transfer" \
       -X POST \
       -H "Content-Type: application/json" \
       -d '{"amount": 10000, "toAddress":"0x5dd4039c32F6EEF427D6F67600D8920c9631D59D","fromPK":<pk of funders address>}'
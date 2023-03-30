export PRIVATE_KEY=
export DEFAULT_PAYMENT_PRIVATE_KEY=
export RPC_URL=https://rpc.octa.space
export RPC_URL_2=https://tea.mining4people.com/rpc
export CONTRACT_ADDRESS=0x0eeAaF074B23942CD660175dEaE6e1A5849d6614


curl -v "http://0.0.0.0:8080/mint" \
       -X POST \
       -H "Content-Type: application/json" \
       -d '{"amount": 10000, "toAddress":"0x5dd4039c32F6EEF427D6F67600D8920c9631D59D"}'


curl -v "http://0.0.0.0:8080/transfer" \
       -X POST \
       -H "Content-Type: application/json" \
       -d '{"amount": 10000, "toAddress":"0x9cA67FFE69698d963A393E9338aD3BcfD2CEa02e","fromPK":<pk of funders address>}'
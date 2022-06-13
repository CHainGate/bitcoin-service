# bitcoin-service

swagger url: http://localhost:9001/api/swaggerui/


openapi gen:
 ```
docker run --rm -v ${PWD}:/local openapitools/openapi-generator-cli generate -i /local/swaggerui/openapi.yaml -g go-server -o /local/ --additional-properties=sourceFolder=openApi,packageName=openApi
docker run --rm -v ${PWD}:/local openapitools/openapi-generator-cli generate -i https://raw.githubusercontent.com/CHainGate/proxy-service/main/swaggerui/openapi.yaml -g go -o /local/proxyClientApi --ignore-file-override=/local/.openapi-generator-ignore --additional-properties=sourceFolder=proxyClientApi,packageName=proxyClientApi
docker run --rm -v ${PWD}:/local openapitools/openapi-generator-cli generate -i https://raw.githubusercontent.com/CHainGate/backend/main/swaggerui/internal/openapi.yaml -g go -o /local/backendClientApi --ignore-file-override=/local/.openapi-generator-ignore --additional-properties=sourceFolder=backendClientApi,packageName=backendClientApi
goimports -w .
 ```

## Bitcoin cli commands
Start bitcoin daemon
```
.\bitcoind.exe -datadir=D:\bitcoin  
.\bitcoind.exe -testnet -datadir=D:\bitcoin
```

Create wallet
```
.\bitcoin-cli.exe -rpcuser=user --rpcpassword=pass createwallet "chaingate-main-wallet" false false "passphrase" false false true
.\bitcoin-cli.exe -testnet --rpcuser=user --rpcpassword=pass createwallet "chaingate-test-wallet" false false "passphrase" false false true

```

Create change address
```
.\bitcoin-cli.exe -rpcuser=user --rpcpassword=pass getrawchangeaddress
.\bitcoin-cli.exe -testnet --rpcuser=user --rpcpassword=pass getrawchangeaddress
```


## Bitcoin regtest
For regtest there is a docker-compose file in test_utils/docker.
It starts 4 nodes. A network, buyer, merchant and chaingate node.
Here is the setup documented to test the CHainGate application.

Start docker-compose and wait for 1min till all nodes are connected with each other. 
```
docker-compose up -V
```

Setup buyer node
```
docker exec -it docker_buyer_1 /bin/bash
/bitcoin-cli -regtest createwallet "buyer-wallet"
/bitcoin-cli -regtest getnewaddress
```

Setup merchant node
```
docker exec -it docker_merchant_1 /bin/bash
/bitcoin-cli -regtest createwallet "merchant-wallet"
/bitcoin-cli -regtest getnewaddress
```

Setup chaingate node
```
docker exec -it docker_chaingate_1 /bin/bash
/bitcoin-cli -regtest createwallet "chaingate-wallet" false false "secret" false false true
/bitcoin-cli -regtest getrawchangeaddress
```

Setup network node
```
docker exec -it docker_network_1 /bin/bash
/bitcoin-cli -regtest createwallet "network-wallet"
/bitcoin-cli -regtest getnewaddress
/bitcoin-cli -regtest generatetoaddress 101 “<addresse von getnewaddress>” #50BTC für die Addresse
/bitcoin-cli -regtest sendtoaddress “<addresse von buyer>” 10 #Sende 10 BTC an buyer
/bitcoin-cli -regtest generatetoaddress 6 “<addresse von getnewaddress>” #verify
```


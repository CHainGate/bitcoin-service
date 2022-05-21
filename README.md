# bitcoin-service

swagger url: http://localhost:9001/api/swaggerui/


openapi gen:
 ```
docker run --rm -v ${PWD}:/local openapitools/openapi-generator-cli generate -i /local/swaggerui/openapi.yaml -g go-server -o /local/ --additional-properties=sourceFolder=openApi,packageName=openApi
docker run --rm -v ${PWD}:/local openapitools/openapi-generator-cli generate -i https://raw.githubusercontent.com/CHainGate/proxy-service/main/swaggerui/openapi.yaml -g go -o /local/proxyClientApi --ignore-file-override=/local/.openapi-generator-ignore --additional-properties=sourceFolder=proxyClientApi,packageName=proxyClientApi
docker run --rm -v ${PWD}:/local openapitools/openapi-generator-cli generate -i https://raw.githubusercontent.com/CHainGate/backend/main/swaggerui/internal/openapi.yaml -g go -o /local/backendClientApi --ignore-file-override=/local/.openapi-generator-ignore --additional-properties=sourceFolder=backendClientApi,packageName=backendClientApi
goimports -w .
 ```

##Bitcoin regtest
Setup Guide: 
- https://bitcointalk.org/index.php?topic=5268794.0 \
- https://olivermouse.wordpress.com/2018/01/13/connecting-multiple-bitcoin-core-nodes-regtest/

Default directory in windows: %APPDATA%\Bitcoin


Start bitcoin regtest
```
.\bitcoind.exe -fallbackfee='0.001' -regtest -rpcuser=user -rpcpassword=XXXX
```

Create wallet
```
.\bitcoin-cli.exe -regtest createwallet "chaingate-wallet"
.\bitcoin-cli.exe -regtest createwallet "buyer-wallet"
.\bitcoin-cli.exe -regtest createwallet "merchant-wallet"
```

Load wallet
```
bitcoin-cli.exe -regtest loadwallet "chaingate-wallet"
```

Unload wallet
```
.\bitcoin-cli.exe -regtest unloadwallet "chaingate-wallet"
```
Create new address
```
.\bitcoin-cli.exe -regtest getnewaddress
```

Fund address
```
.\bitcoin-cli.exe -regtest generatetoaddress 101 "<address>"
```

Send btc
```
.\bitcoin-cli.exe -regtest sendtoaddress "bcrt1qch7g607n2sxnc2e229uc2a0gcf06s5zfxyqqgk" 0.1 "drinks" "room77" true
```



```
.\bitcoind.exe -regtest -fallbackfee='0.000086' -datadir=D:\bitcoin\regtest\network
.\bitcoin-cli.exe -regtest -datadir=D:\bitcoin\regtest\network createwallet "network-wallet"
.\bitcoin-cli.exe -regtest -datadir=D:\bitcoin\regtest\network getnewaddress
.\bitcoin-cli.exe -regtest -datadir=D:\bitcoin\regtest\network generatetoaddress 101 ""
.\bitcoin-cli.exe -regtest -datadir=D:\bitcoin\regtest\network sendtoaddress "" 10


.\bitcoind.exe -regtest -fallbackfee='0.000086' -datadir=D:\bitcoin\regtest\merchant
.\bitcoin-cli.exe -regtest -datadir=D:\bitcoin\regtest\merchant createwallet "merchant-wallet"
.\bitcoin-cli.exe -regtest -datadir=D:\bitcoin\regtest\merchant getnewaddress

.\bitcoind.exe -regtest -fallbackfee='0.000086' -datadir=D:\bitcoin\regtest\chaingate
.\bitcoin-cli.exe -regtest -datadir=D:\bitcoin\regtest\chaingate createwallet "chaingate-wallet" false false "secret"

.\bitcoind.exe -regtest -fallbackfee='0.000086' -datadir=D:\bitcoin\regtest\buyer
.\bitcoin-cli.exe -regtest -datadir=D:\bitcoin\regtest\buyer createwallet "buyer-wallet"
.\bitcoin-cli.exe -regtest -datadir=D:\bitcoin\regtest\buyer getnewaddress
.\bitcoin-cli.exe -regtest -datadir=D:\bitcoin\regtest\buyer sendtoaddress "" 

.\bitcoin-cli.exe -regtest -datadir=D:\bitcoin\regtest\buyer sendtoaddress "bcrt1qud0lp7q3gg2d96uhxj7aa9ee928x8t2zkqnt2s" 0.1
```

Bitcoin Regtest via Docker starten

```
docker exec -it docker_chaingate_1 /bin/bash
/bitcoin-cli -regtest generatetoaddress 1 bcrt1qgclvhaa2lgedrze402qzhpxrkj4sxrlszawn3n
```
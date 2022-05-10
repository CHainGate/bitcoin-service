# bitcoin-service

swagger url: http://localhost:9001/api/swaggerui/


openapi gen:
 ```
docker run --rm -v ${PWD}:/local openapitools/openapi-generator-cli generate -i /local/swaggerui/openapi.yaml -g go-server -o /local/ --additional-properties=sourceFolder=openApi,packageName=openApi
docker run --rm -v ${PWD}:/local openapitools/openapi-generator-cli generate -i https://raw.githubusercontent.com/CHainGate/proxy-service/main/swaggerui/openapi.yaml -g go -o /local/proxyClientApi --ignore-file-override=/local/.openapi-generator-ignore --additional-properties=sourceFolder=proxyClientApi,packageName=proxyClientApi
goimports -w .
 ```

##Bitcoin regtest
Setup Guide: https://bitcointalk.org/index.php?topic=5268794.0 \
New rpc now **18443**

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
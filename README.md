# bitcoin-service

swagger url: http://localhost:9001/api/swaggerui/


openapi gen:
 ```
docker run --rm -v ${PWD}:/local openapitools/openapi-generator-cli generate -i /local/swaggerui/openapi.yaml -g go-server -o /local/ --additional-properties=sourceFolder=openApi,packageName=openApi
docker run --rm -v ${PWD}:/local openapitools/openapi-generator-cli generate -i https://raw.githubusercontent.com/CHainGate/proxy-service/main/swaggerui/openapi.yaml -g go -o /local/proxyClientApi --ignore-file-override=/local/.openapi-generator-ignore --additional-properties=sourceFolder=proxyClientApi,packageName=proxyClientApi
goimports -w .
 ```
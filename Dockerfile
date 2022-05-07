FROM golang:alpine

RUN apk add build-base
WORKDIR /app

RUN apk update && apk add bash

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY *.go ./
COPY internal/ ./internal/
COPY swaggerui/ ./swaggerui/
COPY wait-for-it.sh ./
COPY .openapi-generator-ignore ./

# maybe there is a better way to use openapi-generator-cli
RUN apk add --update nodejs npm
RUN apk add openjdk11
RUN npm install @openapitools/openapi-generator-cli -g
RUN npx @openapitools/openapi-generator-cli generate -i ./swaggerui/openapi.yaml -g go-server -o ./ --additional-properties=sourceFolder=openApi,packageName=openApi
RUN npx @openapitools/openapi-generator-cli generate -i https://raw.githubusercontent.com/CHainGate/proxy-service/main/swaggerui/openapi.yaml -g go -o ./proxyClientApi --ignore-file-override=.openapi-generator-ignore --additional-properties=sourceFolder=proxyClientApi,packageName=proxyClientApi
RUN go install golang.org/x/tools/cmd/goimports@latest
RUN goimports -w .

RUN ["chmod", "+x", "wait-for-it.sh"]

RUN go build -o /bitcoin-service

EXPOSE 9001

CMD [ "/bitcoin-service" ]
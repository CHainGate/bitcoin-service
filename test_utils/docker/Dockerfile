# https://github.com/FreekPaans/bitcoin-multi-node-regtest
FROM ubuntu:22.04

RUN apt-get update

RUN apt-get -y install curl

RUN curl -OL https://bitcoin.org/bin/bitcoin-core-22.0/bitcoin-22.0-x86_64-linux-gnu.tar.gz

RUN tar zxvf bitcoin-22.0-x86_64-linux-gnu.tar.gz

RUN ln -s /bitcoin-22.0/bin/bitcoin-cli /bitcoin-cli

COPY bitcoin.conf /root/.bitcoin/bitcoin.conf

# rpc
EXPOSE 18444/tcp
# p2p
EXPOSE 18443/tcp

ENTRYPOINT ["/bitcoin-22.0/bin/bitcoind", "-regtest",  "-printtoconsole"]
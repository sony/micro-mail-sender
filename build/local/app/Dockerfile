FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y postfix postfix-lmdb rsyslog mailutils curl vim

# go
COPY ./.go-version /tmp/.

RUN go_version=`cat /tmp/.go-version` && curl -L "https://go.dev/dl/go${go_version}.linux-amd64.tar.gz" -o "/tmp/go${go_version}.linux-amd64.tar.gz" && \
  bash -c "cd /tmp ; tar -zxvf go${go_version}.linux-amd64.tar.gz ; tar -zxvf go${go_version}.linux-amd64.tar.gz -C /usr/local/"
ENV PATH $PATH:/usr/local/go/bin:/root/go/bin/

#  air
RUN go install github.com/air-verse/air@v1.61.7


WORKDIR /app

COPY ./build/local/app/entrypoint.sh /tmp/.

ENTRYPOINT ["/bin/bash", "/tmp/entrypoint.sh"]

#!/bin/sh

set -ex

sudo apt-get update
sudo apt-get install zip postgresql-client mailutils rsyslog -y

sudo rsyslogd

echo "postfix postfix/main_mailer_type string 'Internet Site'" > tmp.conf 
echo "postifx postfix/mailname string tmp.local" >> tmp.conf 
sudo debconf-set-selections tmp.conf

sudo apt-get install postfix -y
sudo sed -i -e 's/^\(myhostname = .*\)\.$/\1/g' /etc/postfix/main.cf

cat /etc/postfix/main.cf

sudo service postfix start

go_version=`cat ./.go-version`

curl -L "https://go.dev/dl/go${go_version}.linux-amd64.tar.gz"  --output "go${go_version}.linux-amd64.tar.gz"
sudo tar -C /usr/local -xzf "go${go_version}.linux-amd64.tar.gz"
export PATH=$PATH:/usr/local/go/bin
export GOPATH=$HOME/go
export PATH=$GOPATH/bin:$PATH
export GOBIN=$GOPATH/bin
go version
go mod tidy

psql -h 127.0.0.1 -v ON_ERROR_STOP=1 --username "ms" --dbname "mailsender_test" <  ./build/local/schema.sql

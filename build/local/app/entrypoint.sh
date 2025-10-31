#!/bin/bash

set -e

if [[ -n "${MAIL_NAME}" ]]; then
  echo $MAIL_NAME > /etc/mailname
else
  sed -i "s/, \/etc\/mailname//" /etc/postfix/main.cf
fi

if [[ -n "${RELAY_HOST}" ]]; then
  sed -i "s/relayhost =/relayhost = ${RELAY_HOST}/" /etc/postfix/main.cf
fi

rsyslogd

service postfix start

sleep 10

service postfix restart

air -c .air.toml

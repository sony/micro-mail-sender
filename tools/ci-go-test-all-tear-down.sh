#!/bin/sh

set -ex

sudo cat /var/log/mail.log
sudo service postfix stop

#!/bin/sh
set -ex

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
	CREATE DATABASE mailsender;
	CREATE DATABASE mailsender_test;
EOSQL


psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "mailsender" < /tmp/schema.sql
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "mailsender_test" < /tmp/schema.sql

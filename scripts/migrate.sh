#!/usr/bin/env sh
set -eu
echo "Apply api/migrations/*.sql with your MySQL migration tool or mysql client."
echo "Example: mysql \"$OPENDESK_MYSQL_DSN\" < api/migrations/001_initial_schema.sql"


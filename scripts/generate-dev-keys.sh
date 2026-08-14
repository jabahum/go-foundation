#!/usr/bin/env sh
set -eu
mkdir -p secrets
if [ ! -f secrets/jwt_private.pem ]; then
  openssl genrsa -out secrets/jwt_private.pem 2048 >/dev/null 2>&1
  chmod 600 secrets/jwt_private.pem
fi
if [ ! -f secrets/jwt_public.pem ]; then
  openssl rsa -in secrets/jwt_private.pem -pubout -out secrets/jwt_public.pem >/dev/null 2>&1
fi
echo "JWT development keys are ready in ./secrets"

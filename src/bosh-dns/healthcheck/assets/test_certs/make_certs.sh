#!/usr/bin/env bash

set -ex

# CERT_SET='test_in'

# # EC keys not approved for DoD use so we turned them off in.WithInternalServiceDefaults()
# # If they get added back in, you can switch back to EC keys.
# openssl ecparam -genkey -name secp384r1 -out test_ca.key
# openssl ecparam -genkey -name secp384r1 -out test_fake_ca.key
# openssl ecparam -genkey -name secp384r1 -out test_server.key
# openssl ecparam -genkey -name secp384r1 -out test_client.key

# openssl genrsa -out test_ca.key 2048
# openssl genrsa -out test_fake_ca.key 2048
# openssl genrsa -out test_server.key 2048
# openssl genrsa -out test_client.key 2048

# # Make root CA
# openssl req -x509 -new -nodes -key test_ca.key -sha256 -days 3650 \
#  -config openssl.root_ca.cnf \
#  -out test_ca.pem

# # Make fake root CA
# openssl req -x509 -new -nodes -key test_fake_ca.key -sha256 -days 3650 \
#  -config openssl.root_ca.cnf \
#  -out test_fake_ca.pem

# # Make correct server cert
# openssl req -new -key test_server.key -config openssl.server.cnf -out test_server.csr
# openssl x509 -req -in test_server.csr \
#   -CA test_ca.pem \
#   -CAkey test_ca.key \
#   -CAcreateserial \
#   -extensions req_ext \
#   -extfile openssl.server.cnf \
#   -days 3649 -sha256 \
#   -out test_server.pem

# # Make server cert with the wrong root CA
# openssl x509 -req -in test_server.csr \
#  -CA test_fake_ca.pem \
#  -CAkey test_fake_ca.key \
#  -CAcreateserial \
#  -extensions req_ext \
#  -extfile openssl.server.cnf \
#  -days 3649 -sha256 \
#  -out test_server_incorrect_CA.pem

# # Make correct client cert
# openssl req -new -key test_client.key -config openssl.client.cnf -out test_client.csr
# openssl x509 -req -in test_client.csr \
#  -CA test_ca.pem \
#  -CAkey test_ca.key \
#  -CAcreateserial \
#  -extensions req_ext \
#  -extfile openssl.client.cnf \
#  -days 3649 -sha256 \

# # Make client cert with the wrong root CA
# openssl x509 -req -in test_client.csr \
#  -CA test_fake_ca.pem \
#  -CAkey test_fake_ca.key \
#  -CAcreateserial \
#  -extensions req_ext \
#  -extfile openssl.client.cnf \
#  -days 3649 -sha256 \
#  -out test_fake_client.pem

# # Make client cert with the wrong CN
# openssl req -new -key test_client.key -config openssl.client.wrong_cn.cnf -out test_wrong_cn_client.csr
# openssl x509 -req -in test_wrong_cn_client.csr \
#   -CA test_ca.pem \
#   -CAkey test_ca.key \
#   -CAcreateserial \
#   -extensions req_ext \
#   -extfile openssl.client.wrong_cn.cnf \
#   -days 3649 -sha256 \
#   -out test_wrong_cn_client.pem

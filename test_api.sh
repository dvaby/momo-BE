#!/bin/bash

BASE_URL="http://localhost:8080/api/v1"
EMAIL="guru@example.com"
PASSWORD="password123"
NAMA="Guru Demo"

echo "=== 1. Register / Check Login Guru ==="
REGISTER_RESP=$(curl -s -X POST "$BASE_URL/guru/register" \
  -H "Content-Type: application/json" \
  -d "{
    \"nama\": \"$NAMA\",
    \"email\": \"$EMAIL\",
    \"password\": \"$PASSWORD\"
  }")

LOGIN_RESP=$(curl -s -X POST "$BASE_URL/guru/login" \
  -H "Content-Type: application/json" \
  -d "{
    \"email\": \"$EMAIL\",
    \"password\": \"$PASSWORD\"
  }")

TOKEN=$(echo "$LOGIN_RESP" | jq -r '.token // .data.token // empty')

if [ -z "$TOKEN" ]; then
  echo "Gagal mengambil token. Response: $LOGIN_RESP"
  exit 1
fi

echo "Login Berhasil. Token diperoleh."
echo ""

echo "=== 2. Buat Modul Baru ==="
CREATE_MODUL_RESP=$(curl -s -X POST "$BASE_URL/modul" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "nama": "Modul Matematika Dasar",
    "deskripsi": "Pembahasan aljabar dan aritmatika dasar"
  }')
echo "Response Buat Modul: $CREATE_MODUL_RESP"
echo ""

echo "=== 3. Get Semua Modul (/api/v1/modul) ==="
curl -s -X GET "$BASE_URL/modul" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" | jq .
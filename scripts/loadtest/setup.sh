#!/bin/bash
# setup.sh — Create test aliases for k6 redirect load test
set -e

SSO="localhost:44044"
API="http://127.0.0.1:8080"
export no_proxy="*"
N=${1:-500}  # default: 500 aliases

# Helper to extract JSON field safely
json() { python3 -c "import sys,json; print(json.load(sys.stdin)$1)" 2>/dev/null || echo "None"; }

echo "=== Getting JWT token ==="
# Примечание: убедитесь, что grpcurl возвращает именно {"token": "..."}
TOKEN=$(grpcurl -plaintext \
    -d '{"email":"me@example.com","password":"mypassword123","app_name":"test"}' \
    "$SSO" auth.Auth/Login 2>/dev/null | json "['token']")

if [ -z "$TOKEN" ] || [ "$TOKEN" = "None" ]; then
    echo "ERROR: Failed to get token. Check grpcurl output manually."
    exit 1
fi
echo "Token obtained successfully"

echo "=== Creating $N aliases ==="
# Очищаем временные файлы
rm -f /tmp/aliases_raw.txt /tmp/aliases.json

for i in $(seq 1 $N); do
    # Генерируем случайный 8-символьный алиас
    ALIAS=$(python3 -c "import random,string; print(''.join(random.choices(string.ascii_letters+string.digits,k=8)))")

    RESP=$(curl -s --noproxy '*' -X POST "$API/url" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${TOKEN}" \
        -d "{\"url\": \"https://example.com/$i\", \"alias\": \"$ALIAS\"}")

    # Проверяем, что ответ содержит поле 'alias' (значит, создание прошло успешно)
    if echo "$RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); exit(0 if d.get('alias') else 1)" 2>/dev/null; then
        echo "$ALIAS" >> /tmp/aliases_raw.txt
    fi

    # Прогресс
    if [ $((i % 50)) -eq 0 ]; then
        echo "  Processed $i/$N requests..."
    fi
done

# БЕЗОПАСНОЕ создание JSON через Python (гарантирует валидный синтаксис)
python3 -c "
import json
try:
    with open('/tmp/aliases_raw.txt', 'r') as f:
        aliases = [line.strip() for line in f if line.strip()]
    with open('/tmp/aliases.json', 'w') as f:
        json.dump(aliases, f)
    print(f'Successfully saved {len(aliases)} valid aliases to /tmp/aliases.json')
except Exception as e:
    print(f'Error creating JSON: {e}')
    exit(1)
"

rm -f /tmp/aliases_raw.txt
echo "=== Done. Run: k6 run scripts/loadtest/redirect.js ==="
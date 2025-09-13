# Gophermart project

## Операции с сервисом

Базовые действия с сервисом настроены в Taskfile.yml (https://taskfile.dev/)

### Запуск сервиса

```sh
task compose-up
```

### Очистка БД

```sh
task compose-clean
```

### Запуск автотестов

```sh
task autotests
```

### Создать рандомный JWT secret

```sh
export JWT_SECRET="$(openssl rand -base64 32)"
```

### Получить токен через метод Login

```sh
JWT_TOKEN=$(curl -s -X POST http://localhost:8080/api/user/login -H 'Content-type: application/json' --data '{"login": "test_user", "password": "test_password_111"}' | jq -r '.token')
```

### Полезные запросы в accrual

```sh
# POST /api/goods
curl -s -X POST 'http://localhost:8081/api/goods' \
  -H 'Content-Type: application/json' \
  -d '{
    "match": "Test",
    "reward": 10,
    "reward_type": "%"
  }'

# POST /api/orders
curl -s -X POST 'http://localhost:8081/api/orders' \
  -H 'Content-Type: application/json' \
  -d '{
    "order": "79927398713",
    "goods": [
      {
        "description": "Чайник Test",
        "price": 27.3
      }
    ]
  }'

# GET /api/orders/79927398713
curl -s -X GET 'http://localhost:8081/api/orders/79927398713' \
  -H 'Content-Length: 0' \
  -H 'Accept: application/json'
```

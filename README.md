# Gophermart project

## Утилиты

### Создать рандомный JWT secret

```sh
export JWT_SECRET="$(openssl rand -base64 32)"
```

### Получить токен через метод Login

```sh
JWT_TOKEN=$(curl -s -X POST http://localhost:8080/api/user/login -H 'Content-type: application/json' --data '{"login": "test_user", "password": "test_password_111"}' | jq -r '.token')
```

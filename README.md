# request-monitoring

## Требования
- Git
- Docker Desktop: https://www.docker.com/products/docker-desktop/ (скачать, установить, запустить)

## Запуск проекта
1. Склонируй проект и зайди в папку: `git clone ВСТАВЬ_ССЫЛКУ_НА_РЕПО.git`, затем `cd request-monitoring`
2. Создай свой личный файл переменных из шаблона: `cp .env.example .env` 
3. Открой `.env`: поставь свой логин и пароль. Если порт 5432 занят — поменяй `POSTGRES_PORT` на 5433.
4. Запусти базу (при первом запуске Docker сам скачает образ PostgreSQL — это нормально, подожди): `docker compose up -d`
5. Проверь, что работает: `docker ps` — должен быть контейнер `helpdesk_postgres` со статусом `Up`.

### Подключиться к базе
`docker exec -it helpdesk_postgres psql -U <твой_логин_из_.env> -d obshaga_db`
Выход из консоли: `\q`

### Остановить базу (данные сохранятся)
`docker compose down`

### Удалить базу полностью вместе с данными
`docker compose down -v`
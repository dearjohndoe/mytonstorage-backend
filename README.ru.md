# mytonstorage-backend

**[English version](README.md)**

Backend-сервис для mytonstorage.org.

## Описание

Backend для загрзуки и управления файлами в TON Storage:
- Загрузка файлов в TON Storage
- Управление жизненным циклом storage-контрактов (инициализация, пополнение, закрытие, обновление провайдеров)
- Автоизация через TON Connect
- Мониторит storage-контракты и уведомляет провайдеров о новых bags для загрузки
- REST API эндпоинты для фронтенд-приложения
- Собирает метрики через **Prometheus**

## Установка и настройка

Для начала нам потребуется чистый сервер на Debian 12 с рут пользователем.

1. **Склонируйте скрипт для подключения по ключу**

Вместо логина по паролю, скрипт безопасности требует использовать логин по ключу. Этот скрипт нужно запускать на рабочей машине, он не потребует sudo, а только пробросит ключи для доступа.

```bash
wget https://raw.githubusercontent.com/dearjohndoe/mytonstorage-backend/refs/heads/master/scripts/init_server_connection.sh
```

2. **Пробрасываем ключи и закрываем доступ по паролю**

```bash
USERNAME=root PASSWORD=supersecretpassword HOST=123.45.67.89 bash init_server_connection.sh
```

В случае ошибки man-in-the-middle, возможно вам стоит удалить known_hosts.

3. **Заходим на удаленную машину и качаем скрипт установки**

```bash
ssh root@123.45.67.89 # Если требует пароль, то предыдущий шаг завершился с ошибкой.

wget https://raw.githubusercontent.com/dearjohndoe/mytonstorage-backend/refs/heads/master/scripts/setup_server.sh
```

4. **Запускаем настройку и установку сервера**

Займет несколько минут.

```bash
PG_USER=pguser PG_PASSWORD=pgpassword PG_DB=storagedb NEWFRONTENDUSER=janefrontside  NEWSUDOUSER=janedoe NEWUSER_PASSWORD=newpassword  INSTALL_SSL=false APP_USER=appuser API_PASSWORD=apipassword bash setup_server.sh
```

По завершении выведет полезную информацию по использованию сервера.


## Разработка:
### Настройка VS Code
Создайте `.vscode/launch.json`:
```json
{
    "version": "0.2.0",
    "configurations": [
        {
            "name": "Launch Package",
            "type": "go",
            "request": "launch",
            "mode": "auto",
            "program": "${workspaceFolder}/cmd",
            "buildFlags": "-tags=debug",    // для обработки OPTIONS запросов без nginx при разработке
            "env": {...}
        }
    ]
}
```

## Структура проекта

```
├── cmd/                   # Точка входа приложения, конфиги, инициализация
├── pkg/                   # Пакеты приложения
│   ├── cache/             # Кастомный кеш
│   ├── clients/           # Клиенты TON blockchain и TON Storage
│   ├── httpServer/        # Обработчики и маршруты Fiber сервера
│   ├── models/            # Модели данных БД и API
│   ├── repositories/      # Слой базы данных (PostgreSQL)
│   ├── services/          # Бизнес-логика (auth, files, contracts, providers)
│   └── workers/           # Фоновые воркеры
├── db/                    # Схема базы данных
├── scripts/               # Скрипты установки и утилиты
```

## API эндпоинты

Сервер предоставляет REST API эндпоинты для:
- Логин через TON Connect
- Работа с файлами: загрузка, удаление, отслеживание неоплаченных bags, краткая инфа о bags
- Управление контрактами: создание, пополнение баланса, вывод денег, смена провайдеров
- Получение предложений от провайдеров и их тарифов

## Воркеры

В фоне крутятся воркеры, которые следят за порядком:
- **Files Worker**: Чистит неоплаченные и старые bags, дергает провайдеров на загрузку, проверяет статус
- **Cleaner Worker**: Чистит базу данных от устаревшей информации

## Лицензия

Apache-2.0



Этот проект был создан по заказу участника сообщества TON Foundation.

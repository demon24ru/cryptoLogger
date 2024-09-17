
#### Для запуска проекта в режиме отладки:

```shell
# копируем файл настроек
cp ./.env.dev ./.env
```

#### Устанавливаем зависимости:
``` shell
# устанавливаем зависимости
npm i
```

#### Запускаем проект в dev режиме:
```shell
# Запускаем проект
npm start
```

#### Запускаем проект в обычном режиме:
``` shell
# Запускаем проект
npm start:prod
```
сервер доступен по адресу: http://localhost:5000

## Запуска проекта в Docker контейнере

```shell
# Запуск
docker-compose -f docker-compose.yml up -d --build

# Останов
docker-compose -f docker-compose.yml down

# Удаление остановленных контейнеров
docker system prune -a
```

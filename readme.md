# Weather Forecast & DB Client

## Стек
- Golang
- Postgres
- Docker
- Minikube

## Описание приложения
Приложение позволяет получать прогноз погоды для запрашиваемого города.
При запросе по адресу `/forecast?city_name=<cityName>` проверяется, есть ли в БД неустаревшая информация (обновленная менее получаса назад) и отдается при наличии. Если информации нет или она устарела, то отправляется запрос к weatherapi.com, и из ответа этого сервиса достается температура, добавляется в БД и возвращается новое значение клиенту.

## Запуск
### Minikube и окружение Docker
```bash
minikube start --driver=docker
eval $(minikube docker-env)
```
### Docker-образы
```bash
docker build -t weather-app:latest -f Dockerfile .
docker build -t weather-postgres:latest -f Dockerfile.postgres .
```
### Развертывание
```bash
kubectl apply -f kube.yaml
minikube service weather-app --url
```
### Запрос
```bash
curl "http://http://127.0.0.1:<nodePort>/forecast?city_name=<cityName>"
```
# Uptime

Checks sites every minute for status codes and sends slack alerts.

## Install and Usage

create default uptime.db

```sh
go install github.com/webmocha/uptime
uptime
```

specify a db file path

```sh
uptime /var/db/uptime.db
```


## DB

[bbolt](https://github.com/etcd-io/bbolt) K/V store

### Sites Bucket

```yaml
key: value(Site)
https://webmocha.com:
  FirstCheck: "2019-06-21T17:03:45.584Z"
  LastCheck: "2019-06-21T18:03:45.584Z"
```

### Status Bucket

```yaml
key: value
https://webmocha.com|2019-06-21T17:03:45.584Z: 200
https://webmocha.com|2019-06-21T17:04:45.584Z: 200
https://webmocha.com|2019-06-21T17:05:45.584Z: 200
https://webmocha.com|2019-06-21T17:06:45.584Z: 503
```

## Dev

```sh
ag -g '\.go' . | entr sh -c 'clear && env PORT=8080 make dev'
```

## API

| method | path       | params / payload | returns                                |
|--------|------------|------------------|----------------------------------------|
| GET    | /api/sites |                  | list sites with last status and uptime |
| POST   | /api/sites | key={url}        |                                        |

### GET /api/sites

```json
[
  {
    "key": "https://webmocha.com",
    "firstCheck": "2019-06-21T17:03:45.584Z",
    "lastCheck": "2019-06-21T18:03:45.584Z",
    "status": 200,
    "statusText": "OK",
    "uptime": "2hrs"
  }
]
```

### Testing

curl -X POST --data "key=https://webmocha.com" localhost:8080/api/sites

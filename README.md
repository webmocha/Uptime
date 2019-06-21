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

```json
{
  "https://webmocha.com" : {
    "First": "2019-06-21T17:03:45.584Z",
    "Last": "2019-06-21T18:03:45.584Z"
  }
}
```

### Status Bucket

```json
{
  "https://webmocha.com|2019-06-21T17:03:45.584Z" : 200,
  "https://webmocha.com|2019-06-21T17:04:45.584Z" : 200,
  "https://webmocha.com|2019-06-21T17:05:45.584Z" : 200,
  "https://webmocha.com|2019-06-21T17:06:45.584Z" : 503
}
```

## Dev

```sh
ag '*.go' | entr make dev
```

## API

| method | path   | params / payload | returns                                |
|--------|--------|------------------|----------------------------------------|
| GET    | /sites |                  | list sites with last status and uptime |
| POST   | /sites | key={url}        |                                        |



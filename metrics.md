Работаем в репозитории:

https://github.com/iaon/s3s5

## Цель

Реализовать этап P0 подготовки оптимизации производительности `s3s5`:

1. Добавить детальные метрики S3-операций и туннельных сессий.
2. Создать воспроизводимый набор performance-тестов для разных паттернов TCP-трафика.
3. Добавить механизм сохранения сырых результатов и генерации читаемого отчёта.
4. Зафиксировать исходный baseline текущего протокола v1.

На этом этапе не оптимизировать протокол и не менять его wire format. Задача — получить объективные измерения, на основании которых позже будут проверяться P1-изменения.

## Основные ограничения

* Не менять текущий S3 mailbox protocol v1.
* Не менять key layout, ACK-семантику, close-семантику, chunking и polling behavior.
* Не добавлять size-or-deadline aggregation на этом этапе.
* Не менять совместимость Go-сервера и Android-клиента.
* Не требовать реальных AWS/Yandex/MinIO credentials для обычных unit-тестов и CI.
* Не добавлять тяжёлую внешнюю telemetry-инфраструктуру, если задача решается внутренним collector.
* Все новые тесты должны быть воспроизводимыми.
* Performance-тесты не должны флапать из-за требований к абсолютному времени выполнения.
* Пороговые проверки должны основываться преимущественно на количестве операций и логических событиях, а не на wall-clock latency.

## Сначала изучить

Перед изменениями изучить:

* `internal/tunnel/tunnel.go`
* `internal/tunnel/stats.go`
* `internal/objectstore/objectstore.go`
* `internal/objectstore/memory`
* `internal/objectstore/s3`
* `internal/protocol/protocol.go`
* существующие tunnel/integration tests
* `docs/PERFORMANCE.md`
* `docs/PROTOCOL.md`
* CLI-конфигурацию клиента и сервера
* Android-реализацию только для понимания совместимости; Android-код в рамках P0 менять только при наличии действительно общей обязательной метрики, без которой результаты будут некорректны

Сначала кратко опиши текущие точки выполнения `PUT`, `GET`, `HEAD`, `LIST` и `DELETE`, а затем реализуй instrumentation.

## 1. Архитектура метрик

Добавить instrumentation на уровне `ObjectStore`, а не размазывать измерение длительности по tunnel-коду.

Предпочтительный вариант — wrapper/decorator:

```go
type InstrumentedStore struct {
    Next      objectstore.ObjectStore
    Collector MetricsCollector
}
```

Он должен перехватывать:

* `PutObject`
* `GetObject`
* `HeadObject`
* `ListPrefix`
* `DeleteObject`

Wrapper должен измерять:

* тип операции;
* длительность;
* результат;
* класс ключа;
* размер request payload, где применимо;
* размер response payload, где применимо.

Не допускать попадания в метрики:

* access key;
* secret key;
* session token;
* PSK;
* полного object key с session ID;
* target hostname, если это может раскрывать чувствительные данные.

### Классы ключей

Классифицировать операции минимум по следующим значениям:

```text
open
open-result
data-c2s
data-s2c
ack-c2s
ack-s2c
close-client
close-server
heartbeat-client
heartbeat-server
list-open
unknown
```

Не использовать полный ключ как label, чтобы избежать высокой кардинальности.

### Результаты операций

Минимальные классы результата:

```text
success
not_found
cancelled
timeout
error
```

### Метрики S3/ObjectStore

Собирать минимум:

```text
objectstore_requests_total{operation,key_class,result}
objectstore_request_duration
objectstore_request_bytes
objectstore_response_bytes
```

Внутренний collector может хранить агрегаты без Prometheus dependency:

* count;
* total duration;
* min;
* max;
* набор samples либо bounded reservoir для вычисления percentiles;
* total request bytes;
* total response bytes.

Для benchmark run допустимо хранить все duration samples, поскольку количество операций ограничено.

В отчёте нужны:

```text
count
success
not_found
error
p50
p95
p99
max
request bytes
response bytes
```

## 2. Метрики tunnel/session уровня

Расширить существующую статистику или добавить отдельный session/performance collector.

Собирать минимум:

```text
sessions_started
sessions_opened
sessions_rejected
sessions_completed
sessions_failed

chunks_sent
chunks_received
plaintext_bytes_sent
plaintext_bytes_received
sealed_bytes_sent
sealed_bytes_received

time_to_open_result
session_duration
time_to_first_c2s_object
time_to_first_s2c_object
```

Добавить производные показатели в отчёт:

```text
S3 operations per session
S3 operations per MiB
GET misses per received data object
HEAD operations per session
LIST operations per opened session
ACK GET per data PUT
ACK PUT per received data object
sealed/plaintext size ratio
```

Если часть метрик нельзя надёжно получить без изменения архитектуры, явно отметить это в коде и отчёте. Не подменять измерение предположением.

## 3. Test ObjectStore с управляемой задержкой

Добавить test wrapper над memory store, позволяющий задавать:

```go
type DelayProfile struct {
    PutDelay    time.Duration
    GetDelay    time.Duration
    HeadDelay   time.Duration
    ListDelay   time.Duration
    DeleteDelay time.Duration
    Jitter      time.Duration
}
```

Требования:

* фиксированное seed для jitter;
* возможность отключить jitter;
* context cancellation должна прерывать ожидание;
* instrumentation должен измерять задержку так же, как она наблюдалась бы вызывающим кодом;
* обычный memory store должен продолжать работать без задержек;
* wrapper не должен менять consistency semantics memory store.

Дополнительно можно предусмотреть scripted behavior для отдельных тестов, например несколько последовательных `not found`, но не усложнять реализацию без необходимости.

## 4. Traffic pattern harness

Создать отдельный performance/integration harness, который запускает реальный путь:

```text
SOCKS5 client
→ s3s5 client
→ instrumented/delayed ObjectStore
→ s3s5 server
→ TCP target
```

Не ограничиваться прямыми вызовами внутренних функций, если это исключает open/open-result и SOCKS handshake.

Каждый сценарий должен иметь:

* уникальное имя;
* описание;
* параметры;
* timeout;
* ожидаемый объём переданных данных;
* структурированный результат.

Минимальный набор сценариев:

### A. `one-byte-echo-active`

* открыть SOCKS-соединение;
* отправить один байт;
* получить один байт echo;
* измерить RTT;
* соединение до этого активно, без длительного idle.

### B. `one-byte-echo-after-idle`

* открыть соединение;
* держать соединение idle заданное время, чтобы polling backoff достиг `poll-max`;
* значение по умолчанию для realistic/local baseline — 10 секунд;
* отправить один байт;
* получить echo;
* измерить RTT после idle;
* не использовать жёстко зашитые 30 секунд в unit/CI варианте: разрешить уменьшенные poll settings;
* smoke/unit варианты должны явно уменьшать `idle-duration`.

### C. `small-chatty-writes`

* выполнять записи в течение заданного времени;
* значение по умолчанию — 10 секунд;
* размер каждой записи — 100 байт;
* интервал между записями по умолчанию — 5 мс;
* echo server должен вернуть данные;
* проверить целостность и порядок;
* измерить количество data objects и общий request count.

### D. `bulk-one-mib`

* передать 1 MiB непрерывным потоком;
* получить его обратно либо принять на sink server;
* проверить SHA-256 или точное совпадение;
* измерить throughput;
* измерить chunks и операции на MiB.

### E. `short-connections`

* последовательно открыть заданное число коротких SOCKS-соединений;
* на каждом передать маленький request/response;
* значение по умолчанию для локального запуска — 20;
* измерить operations per session и open latency.

### F. `concurrent-idle-sessions`

* открыть 20 SOCKS-соединений;
* оставить их idle на ограниченный период;
* значение периода по умолчанию — 10 секунд;
* измерить фоновые GET, HEAD и LIST;
* результат нормализовать как operations per idle session per second.

### G. `mixed-traffic`

Одновременно:

* один bulk stream;
* несколько небольших request/response streams, удерживаемых по `chatty-duration`;
* несколько idle streams, удерживаемых по `idle-duration`.

Измерить:

* корректность всех потоков;
* latency небольших запросов;
* общий request count;
* число активных сессий;
* влияние bulk-потока на остальные соединения.

Если смешанный сценарий слишком большой для первого patch set, допускается реализовать его отдельным benchmark test, но он должен присутствовать в итоговом плане и документации.

## 5. Профили окружения

Добавить минимум три профиля:

### `memory`

* без искусственной задержки;
* используется для проверки логики и точного количества операций;
* подходит для обычного CI.

### `simulated-s3`

Рекомендуемые defaults:

```text
PUT:    100 ms
GET:    100 ms
HEAD:   100 ms
LIST:   120 ms
DELETE: 100 ms
jitter: 10 ms
```

Значения должны настраиваться флагами.

Этот профиль нужен для оценки влияния последовательных object-store round trips.

### `real-s3`

Опциональный режим, запускаемый только при наличии переменных окружения.

Поддержать существующую конфигурацию провайдеров:

```text
aws
yandex
minio
custom
```

Требования:

* использовать отдельный случайный benchmark prefix;
* не затрагивать пользовательские данные;
* в конце пытаться удалить созданные объекты;
* при ошибке cleanup вывести prefix;
* никогда не печатать credentials;
* не запускать в обычном CI;
* явно требовать opt-in флаг или environment variable.

## 6. Формат результата

Каждый запуск должен создавать machine-readable JSON.

Предлагаемая структура:

```json
{
  "schema_version": 1,
  "timestamp": "...",
  "git_commit": "...",
  "dirty_worktree": false,
  "go_version": "...",
  "os": "...",
  "arch": "...",
  "profile": "memory",
  "provider": "memory",
  "config": {
    "chunk_size": 65536,
    "poll_min": "50ms",
    "poll_max": "2s",
    "window_chunks": 16,
    "idle_timeout": "2m"
  },
  "scenarios": [
    {
      "name": "small-chatty-writes",
      "status": "passed",
      "parameters": {},
      "duration_ms": 0,
      "traffic": {},
      "session_metrics": {},
      "objectstore_metrics": {},
      "derived_metrics": {}
    }
  ]
}
```

Не включать:

* credentials;
* PSK;
* полный S3 endpoint с query parameters;
* session IDs;
* полный список object keys.

## 7. Хранение результатов в репозитории

Создать структуру:

```text
benchmarks/
  README.md
  results/
    README.md
    baseline-v1-memory.json
    baseline-v1-simulated-s3.json
  reports/
    baseline-v1.md
```

Требования:

* baseline-файлы должны иметь стабильные имена;
* временные локальные результаты с timestamp не должны обязательно коммититься;
* определить отдельную директорию или шаблон имени для локальных результатов;
* добавить нужные правила в `.gitignore`;
* committed baseline должен содержать информацию о commit SHA и конфигурации;
* JSON должен быть отформатирован детерминированно;
* Markdown report должен генерироваться из JSON, а не поддерживаться вручную.

Добавить команды, например:

```text
make perf-test
make perf-test-simulated
make perf-report
make perf-baseline
```

Семантика:

* `perf-test` — быстрый memory profile;
* `perf-test-simulated` — simulated S3 profile;
* `perf-report` — генерация Markdown из указанного JSON;
* `perf-baseline` — явное обновление committed baseline, без автоматического выполнения в CI.

Если более подходящими будут Go subcommands или отдельный CLI, это допустимо, но интерфейс должен быть документирован и прост для повторного запуска.

## 8. Отчёт

Сгенерированный Markdown-отчёт должен содержать:

1. Commit и окружение.
2. Конфигурацию протокола.
3. Таблицу сценариев.
4. Для каждого сценария:

   * passed/failed;
   * переданные bytes;
   * duration;
   * S3 operation counts;
   * not-found polling misses;
   * p50/p95/p99 latency по операциям;
   * operations per session;
   * operations per MiB;
   * sealed/plaintext ratio.
5. Общую таблицу операций по key class.
6. Наблюдения baseline без предложений, не подтверждённых измерениями.

Пример таблицы:

```text
| Scenario | PUT | GET hit | GET miss | HEAD | LIST | DELETE | Ops/session | Ops/MiB |
```

Для idle-сценария:

```text
| Scenario | Sessions | Duration | GET/s | HEAD/s | LIST/s | Ops/session/s |
```

## 9. Автоматические проверки

Unit tests должны проверить:

* корректную классификацию ключей;
* корректный учёт success/not_found/error;
* учёт request/response bytes;
* percentiles на известном наборе значений;
* отсутствие session ID в labels;
* корректную context cancellation для delayed store;
* детерминированный JSON;
* генерацию Markdown из fixture;
* корректность каждого traffic pattern;
* отсутствие утечек goroutine после завершения сценариев;
* корректный cleanup benchmark prefix для real-S3 режима, насколько это можно проверить на memory store.

Для memory profile добавить точные assertions по operation counts там, где они стабильны.

Не добавлять строгие CI assertions вида:

```text
RTT < 200 ms
```

Допускаются только широкие safety timeout и логические проверки.

## 10. Интеграция с CI

В обычный CI включить:

* unit tests instrumentation;
* memory performance smoke;
* генерацию отчёта из test fixture;
* проверку, что committed baseline JSON соответствует schema.

Не включать:

* simulated profile с длительным ожиданием, если он заметно замедляет CI;
* реальные S3 providers;
* сравнение wall-clock performance между CI runs.

Можно добавить отдельный ручной workflow для полного performance run, если это не требует secrets по умолчанию.

## 11. Документация

Обновить:

* `docs/PERFORMANCE.md`
* при необходимости `CONTRIBUTING.md`
* `benchmarks/README.md`

Документация должна объяснять:

* какие сценарии существуют;
* как запустить memory, simulated и real-S3 profiles;
* где сохраняются результаты;
* как обновить baseline;
* почему wall-clock результаты разных машин нельзя напрямую сравнивать;
* какие показатели можно сравнивать надёжно: прежде всего request counts, misses, chunks и bytes;
* что P0 не меняет протокол и не является оптимизацией.

Добавить раздел «Baseline before v1 optimizations».

## 12. Требования к качеству реализации

* Использовать `gofmt`.
* Не ломать `go test ./...`.
* Запустить `go test -race` минимум для изменённых Go packages, если окружение позволяет.
* Не использовать глобальные mutable collectors.
* Метрики разных тестов не должны смешиваться.
* Collector должен быть безопасен для concurrent use.
* Не создавать unbounded high-cardinality labels.
* Не хранить чувствительные значения.
* Не менять production behavior при отключённой instrumentation.
* Изменения должны быть разбиты на понятные компоненты:

  * collector;
  * instrumented store;
  * delayed test store;
  * traffic harness;
  * result schema;
  * report generator;
  * documentation.

## 13. Критерии готовности

Задача считается выполненной, когда:

1. `go test ./...` проходит.
2. Есть воспроизводимый memory performance run.
3. Есть simulated-S3 run с настраиваемой задержкой.
4. Все обязательные traffic patterns реализованы либо явно указана аргументированная причина переноса только смешанного сценария.
5. Каждый run сохраняется в структурированный JSON.
6. Из JSON генерируется Markdown report.
7. В репозитории сохранён baseline текущего протокола v1.
8. В отчёте видны отдельные:

   * data GET hit;
   * data GET miss;
   * ACK GET/PUT;
   * close HEAD;
   * open LIST;
   * операции на сессию;
   * операции на MiB.
9. Нет изменений wire protocol v1.
10. Go/Android protocol compatibility не нарушена.

## 14. Итоговый ответ

В конце работы предоставить:

* краткое описание архитектуры instrumentation;
* перечень изменённых файлов;
* команды для запуска;
* расположение baseline JSON и Markdown report;
* таблицу исходных результатов;
* найденные измерениями hotspots;
* список ограничений и неизвестных;
* подтверждение, что protocol v1 и Android compatibility не изменены;
* результаты:

  * `go test ./...`
  * race tests;
  * performance smoke;
* рекомендуемый следующий небольшой P1 patch, но не реализовывать его в рамках этой задачи.

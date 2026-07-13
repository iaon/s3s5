Работаем в репозитории:

https://github.com/iaon/s3s5

## Цель

Реализовать этап P1 оптимизации S3 mailbox protocol `s3s5`.

Основные цели:

1. Уменьшить количество S3-операций.
2. Уменьшить задержки интерактивного и post-idle трафика.
3. Уменьшить размер S3 data objects.
4. Улучшить масштабирование по числу сессий.
5. Подтвердить каждое изменение результатами performance harness, созданного на этапе P0.

Обратная совместимость с предыдущими версиями Go- и Android-клиентов не требуется.

Все компоненты обновляются согласованно:

* Go client;
* Go server;
* Android client;
* protocol documentation;
* tests;
* benchmark fixtures.

При этом не менять основную архитектуру protocol v1:

```text
один SOCKS5 CONNECT
→ одна mailbox session
→ последовательные data objects c2s и s2c
→ cumulative ACK objects
→ close markers
→ S3 polling
```

Не реализовывать на этом этапе:

* общий persistent mailbox для нескольких SOCKS-соединений;
* multiplexing stream IDs;
* protocol v2;
* SQS/SNS/Event Notification transport;
* замену S3 другим транспортом.

## Предварительные условия

Перед началом проверить наличие результатов P0:

* instrumented `ObjectStore`;
* раздельные метрики `PUT`, `GET`, `HEAD`, `LIST`, `DELETE`;
* классификация ключей;
* traffic pattern harness;
* memory profile;
* simulated-S3 profile;
* baseline JSON;
* генератор Markdown-отчёта.

Если отдельной части P0 не хватает для объективного сравнения, сначала реализовать минимально необходимую instrumentation. Не заменять измерения предположениями.

## Основное правило реализации

Каждую оптимизацию выполнять как отдельный измеримый этап:

1. Зафиксировать состояние до изменения.
2. Реализовать изменение.
3. Добавить unit и integration tests.
4. Запустить memory profile.
5. Запустить simulated-S3 profile.
6. Сохранить JSON.
7. Сравнить результат с baseline и предыдущим этапом.
8. Только затем переходить к следующему изменению.

Все промежуточные результаты должны содержать commit SHA и полную performance-конфигурацию.

## Порядок изменений

Рекомендуемый порядок:

```text
P1.1 Stateful cumulative ACK cache
P1.2 Удаление ненужного final ACK
P1.3 Исправление close check cadence
P1.4 LIST pagination и lifecycle open objects
P1.5 Обязательный обмен directional chunk limits
P1.6 Size-or-deadline aggregation
P1.7 Бинарный data encryption envelope
P1.8 Activity-triggered polling wake-up
P1.9 Финальная настройка параметров по результатам benchmark matrix
```

Порядок можно изменить только с объяснением в итоговом отчёте.

# P1.1. Stateful cumulative ACK cache

## Проблема

После заполнения initial window отправитель может выполнять `GET` ACK object перед почти каждым следующим data object, хотя ранее прочитанное cumulative ACK уже разрешает отправку нескольких chunks.

## Требуемое поведение

Для каждого направления каждой session создать постоянное состояние send window:

```go
type SendWindow struct {
    AckedNextSeq uint64
}
```

Перед отправкой `seq`:

```text
если seq < AckedNextSeq + WindowChunks:
    отправка разрешена без GET ACK

иначе:
    читать ACK object;
    монотонно обновлять AckedNextSeq;
    ждать освобождения окна.
```

Требования:

* initial window не выполняет ACK GET;
* ACK читается только при реальном достижении локальной границы окна;
* старое значение ACK не уменьшает cached state;
* одинаковое значение ACK не приводит к busy loop;
* ожидание использует adaptive backoff;
* context cancellation немедленно завершает ожидание;
* состояние не разделяется между разными sessions или направлениями;
* Go и Android реализуют одинаковую логику.

## Тесты

Добавить тесты:

* initial window не читает ACK;
* первый ACK открывает отправку нескольких chunks;
* повторное чтение ACK до следующей границы не выполняется;
* stale ACK игнорируется;
* sender блокируется при полном окне;
* новый ACK разблокирует sender;
* cancellation завершает ожидание;
* разные направления имеют независимое состояние.

## Основные метрики

```text
ACK GET per data PUT
ACK GET per MiB
total GET
total operations
```

# P1.2. Удаление ненужного final ACK после close

## Проблема

Receive path может записывать финальный ACK после обнаружения peer close, хотя sender к этому моменту:

1. уже записал все data objects;
2. завершил чтение локального TCP stream;
3. записал close marker;
4. больше не использует ACK для отправки данных.

## Изменение

После обнаружения peer close не записывать финальный ACK, если он уже не может использоваться активным sender.

Промежуточные ACK, необходимые для освобождения send window во время передачи, сохранить.

## Тесты

* короткий поток меньше ACK interval не создаёт final ACK;
* поток, которому требовались промежуточные ACK, завершается корректно;
* проверка обоих направлений;
* close не приводит к потере последних data objects;
* sender и receiver не зависают.

## Основные метрики

```text
ACK PUT per short session
PUT per short session
total operations per short session
```

# P1.3. Исправление close check cadence

## Проблема

После успешного data GET следующий `not found` может сразу вызывать `HEAD close`, даже если ожидаемое поведение — проверка close после нескольких последовательных misses.

## Изменение

Добавить конфигурацию:

```text
close-check-after-misses
```

Начальное значение:

```text
4
```

Семантика:

* после успешного data GET число последовательных misses сбрасывается в `0`;
* каждый `GET data → not found` увеличивает счётчик;
* `HEAD close` выполняется при достижении configured threshold;
* после `HEAD close → not found` счётчик сбрасывается;
* успешный data GET снова сбрасывает счётчик;
* значение `1` означает проверку после каждого data miss;
* значение `0` запрещено либо имеет чётко документированную семантику.

## Тесты

* точное количество GET misses перед HEAD;
* успешный data GET сбрасывает счётчик;
* close обнаруживается;
* threshold `1`;
* invalid config;
* cancellation;
* одинаковое поведение Go и Android.

## Основные метрики

```text
HEAD per session
HEAD per idle second
close detection delay
total operations
```

# P1.4. LIST pagination и lifecycle open objects

## Проблемы

Текущая реализация может:

* читать только первые 1000 ключей;
* многократно возвращать active open objects в LIST;
* хранить обработанные session IDs без ограничения;
* не обнаруживать новые sessions за первой страницей;
* бесконечно повторять обработку invalid open objects.

## Pagination

Изменить интерфейс object store так, чтобы он поддерживал настоящую `ListObjectsV2` pagination.

Пример:

```go
type ListOptions struct {
    MaxKeys           int
    ContinuationToken string
}

type ListPage struct {
    Keys                  []string
    IsTruncated           bool
    NextContinuationToken string
}
```

Обновить:

* S3 store;
* memory store;
* instrumented wrapper;
* delayed wrapper;
* doctor;
* cleanup helpers;
* tests.

Не эмулировать pagination повторным запросом первой страницы.

## Lifecycle open object

После того как сервер:

1. обнаружил open key;
2. успешно загрузил объект;
3. расшифровал request;
4. провалидировал структуру;
5. зарегистрировал session как in-flight;

удалить open object.

Не ждать завершения TCP session.

Для permanently invalid request:

1. записать rejected `open-result`;
2. удалить invalid open object;
3. не допускать hot loop.

Для временной ошибки S3 или context cancellation не удалять объект до успешного чтения.

## In-flight state

Вместо вечного `processed sync.Map` использовать map только для одновременно обрабатываемых sessions.

Session ID удаляется из map после завершения goroutine.

Обеспечить защиту от двойного запуска одной session между конкурентными LIST rounds.

## Тесты

* более 1000 open objects обрабатываются;
* continuation token корректно передаётся;
* active open object удаляется после принятия;
* active session не присутствует в следующих LIST;
* invalid open не создаёт hot loop;
* временная ошибка оставляет open object;
* session удаляется из in-flight map после завершения;
* одна session не запускается дважды;
* cleanup проходит через все страницы.

## Основные метрики

```text
LIST operations
LIST response bytes
keys returned per LIST
open objects repeatedly listed
time to open result
```

# P1.5. Обязательный обмен directional chunk limits

Обратная совместимость не требуется. Поэтому chunk limits сделать обязательной частью handshake.

## Семантика

Каждая сторона сообщает максимальный размер plaintext chunk, который она готова принять.

### OpenRequest

Клиент передаёт:

```json
{
  "version": 1,
  "session_id": "...",
  "target": {
    "type": "domain",
    "host": "example.com",
    "port": 443
  },
  "max_receive_chunk_size": 65536,
  "created_at": "..."
}
```

Поле означает:

```text
максимальный plaintext chunk,
который клиент принимает в направлении s2c
```

### OpenResult

Сервер передаёт:

```json
{
  "version": 1,
  "session_id": "...",
  "accepted": true,
  "max_receive_chunk_size": 65536,
  "created_at": "..."
}
```

Поле означает:

```text
максимальный plaintext chunk,
который сервер принимает в направлении c2s
```

## Effective limits

Для client-to-server:

```text
effective c2s max =
    server OpenResult.max_receive_chunk_size
```

Для server-to-client:

```text
effective s2c max =
    client OpenRequest.max_receive_chunk_size
```

Локальная сторона может иметь меньший configured send target, поэтому фактическое значение:

```text
effective send max =
    min(local configured send max,
        peer advertised receive max)
```

## Требования

* оба поля обязательны;
* отсутствие поля является protocol error;
* нулевое или отрицательное значение является protocol error;
* значение выше hard maximum является protocol error;
* не использовать silent legacy fallback;
* сервер не начинает data path до успешной negotiation;
* клиент не отвечает SOCKS success до получения валидного server limit;
* negotiated limits сохраняются в session state;
* effective limits доступны в metrics без session ID;
* Go и Android используют одинаковую validation logic.

## Ограничения

Определить constants:

```go
const (
    MinChunkSize = 1024
    MaxChunkSize = 16 * 1024 * 1024
)
```

Конкретный hard maximum выбрать после анализа:

* memory allocation;
* Android memory;
* AES-GCM;
* максимального допустимого benchmark object;
* server limits.

Не позволять удалённой стороне инициировать неограниченное выделение памяти.

## Версионирование

Поскольку wire compatibility не требуется, допустимо:

* сохранить key prefix `v1`;
* сохранить `Version = 1`;
* обновить protocol specification и fixtures согласованно.

Однако все текущие бинарники должны считаться несовместимыми с обновлённым handshake. Это явно указать в release notes.

## Тесты

* корректный c2s limit;
* корректный s2c limit;
* разные directional limits;
* minimum;
* maximum;
* zero;
* negative или malformed JSON;
* значение выше hard maximum;
* отсутствие поля;
* effective local minimum;
* rejected open;
* Go fixtures;
* Android fixtures.

# P1.6. Size-or-deadline aggregation

## Цель

Несколько небольших TCP reads должны объединяться в один data object.

`ChunkSize` больше не является только размером одного вызова `Read`. Он задаёт локальный target, ограниченный negotiated receive maximum peer.

## Модель

После получения первого байта начать aggregation window.

Flush выполняется при первом наступившем событии:

```text
size:
    буфер достиг effective max chunk;

deadline:
    истёк flush delay после получения первого байта;

EOF:
    source завершился, а буфер непустой;

error:
    reader вернул данные вместе с ошибкой;

cancellation:
    session завершается.
```

## Конфигурация

Добавить:

```text
--chunk-size
--flush-delay
```

Семантика:

```text
chunk-size:
    локальный желаемый максимум plaintext chunk;

effective chunk:
    min(local chunk-size, peer max_receive_chunk_size);

flush-delay:
    максимальное время накопления после первого полученного байта.
```

Предлагаемые начальные значения:

```text
chunk-size: 64 KiB
flush-delay: 10 ms
```

Окончательные defaults выбрать только после benchmark matrix.

## Поведение flush-delay=0

Определить:

```text
flush-delay=0 отключает ожидание дополнительных данных
и сохраняет немедленный flush каждого непустого read.
```

Это позволит сравнивать aggregation с legacy-like behavior.

## Реализация

Выделить independently testable component.

Пример интерфейса:

```go
type AggregatorConfig struct {
    MaxBytes   int
    FlushDelay time.Duration
}

type AggregatedRead struct {
    Data        []byte
    FlushReason FlushReason
}
```

Компонент не должен использовать `io.ReadFull`, поскольку тот задержит маленький интерактивный поток до заполнения всего chunk.

Допустимые варианты:

* reader goroutine и timer;
* deadline-aware чтение для `net.Conn`;
* buffered event loop;
* другой корректный механизм.

Не допускать:

* утечки goroutine;
* потери bytes;
* изменения порядка;
* блокировки после cancellation;
* двойного flush одного buffer;
* превышения effective max;
* data race между timer и read.

## Reader contract

Корректно обработать:

```go
n > 0 && err == io.EOF
n > 0 && err != nil
n == 0 && err == nil
```

При `n > 0` bytes должны быть обработаны до EOF/error.

При non-EOF error:

1. отправить уже прочитанные bytes, если session остаётся валидной;
2. записать close marker с причиной;
3. вернуть исходную ошибку.

## Метрики

Добавить:

```text
aggregation_flush_total{reason=size|deadline|eof|error}
aggregation_plaintext_bytes
aggregation_fill_ratio
socket_reads_total
socket_reads_per_data_object
aggregation_wait_duration
effective_chunk_size
```

## Benchmark matrix

Прогнать минимум:

```text
flush delay:
0 ms
2 ms
5 ms
10 ms
20 ms
50 ms

chunk size:
16 KiB
32 KiB
64 KiB
128 KiB
256 KiB
```

Не обязательно запускать полный декартов продукт в обычном CI. Полная matrix запускается вручную или отдельной benchmark-командой.

Выбрать defaults по следующим критериям:

* small-chatty-writes data PUT;
* one-byte latency;
* bulk operations per MiB;
* memory usage;
* sealed object size;
* post-idle latency.

## Тесты

* один байт;
* несколько мелких reads;
* размер ровно effective max;
* размер больше effective max;
* deadline flush;
* EOF flush;
* empty EOF;
* error with bytes;
* error without bytes;
* cancellation;
* timer/read race;
* negotiated maximum меньше local chunk size;
* negotiated maximum больше local chunk size;
* сохранение точного порядка bytes.

# P1.7. Бинарный data encryption envelope

## Проблема

Текущий data payload после AES-GCM:

1. кодируется Base64;
2. помещается в JSON envelope.

Для 64-КиБ plaintext это увеличивает data object примерно на треть.

Обратная совместимость не требуется, поэтому data objects перевести на бинарный envelope.

Control objects можно оставить JSON внутри существующего encryption envelope либо также перевести позже. Главный обязательный выигрыш — data objects.

## Бинарный формат data object

Определить компактный формат, например:

```text
magic            4 bytes
envelope version 1 byte
algorithm        1 byte
nonce length     1 byte
flags            1 byte
plaintext length 4 bytes
nonce            12 bytes
ciphertext       N bytes
GCM tag          16 bytes
```

Допускается другой формат, если он:

* однозначно парсится;
* имеет version;
* содержит bounded lengths;
* не доверяет входным length fields;
* не дублирует данные без необходимости;
* не использует Base64;
* не использует JSON для data objects.

Можно не хранить plaintext length, если ciphertext length однозначно вычисляется и проверяется. Предпочтителен минимальный формат.

## Crypto requirements

Сохранить:

* AES-256-GCM;
* HKDF-SHA256;
* session/direction key separation;
* random unique nonce;
* AAD binding:

  * object type;
  * session ID;
  * direction;
  * sequence;
  * protocol/envelope version.

Не переиспользовать nonce.

Не ослаблять authentication.

## API

Разделить codec methods при необходимости:

```go
SealControl(...)
OpenControl(...)

SealData(...)
OpenData(...)
```

Либо добавить envelope mode без неоднозначного autodetection.

Поскольку совместимость не нужна, старый JSON data envelope не требуется распознавать.

## Тесты

* binary round trip;
* tampered header;
* tampered nonce;
* tampered ciphertext;
* tampered tag;
* wrong session;
* wrong direction;
* wrong sequence;
* unsupported version;
* unsupported algorithm;
* truncated input;
* oversized declared length;
* trailing garbage;
* cross-language Go/Android fixture;
* максимальный chunk;
* empty plaintext, если он допускается.

## Метрики

Сравнить:

```text
sealed/plaintext ratio
S3 request bytes
S3 response bytes
CPU time
allocations
```

Для 64-КиБ plaintext ожидаемый overhead должен быть близок к размеру бинарного header, nonce и GCM tag, а не к Base64 expansion.

# P1.8. Activity-triggered polling wake-up

## Проблема

После простоя receive poller может спать до `poll-max`.

Когда локальная сторона отправляет новый request, её poller обратного направления продолжает старый sleep. Это увеличивает latency получения ответа.

## Изменение

Создать общий activity signal внутри одной локальной tunnel session.

Activity возникает при:

* локальном socket read;
* aggregation flush;
* успешном data PUT;
* успешном data GET;
* open request;
* open result;
* ACK progression.

Receive poller должен ждать:

```text
context cancellation
или
backoff timer
или
activity signal
```

После activity:

* прервать текущий sleep;
* установить delay в `poll-min`;
* перейти в active polling period.

## Active polling period

Добавить:

```text
--active-poll-duration
```

Начальная matrix:

```text
0
250 ms
500 ms
1 s
2 s
```

Во время active period poller может использовать `poll-min` либо отдельный `poll-active`.

При завершении active period должен вернуться exponential backoff.

## Требования

* activity notification не блокирует sender;
* bounded channel или generation counter;
* notifications coalesce;
* отсутствие lost wake-up, который приводит к полному `poll-max` sleep после локальной отправки;
* отсутствие busy loop;
* idle request rate остаётся ограниченным;
* activity одной session не будит остальные;
* одинаковое поведение Go и Android;
* context cancellation;
* отсутствие goroutine leaks.

## Метрики

```text
poll_wakeup_total{reason=activity|timer}
active_poll_periods
poll_delay
one-byte echo RTT after idle
GET misses after local send
idle operations per second
```

# P1.9. Настройка defaults

После реализации всех механизмов подобрать defaults на основании P0/P1 benchmark results.

Не выбирать значения только интуитивно.

Настроить:

```text
chunk size
flush delay
window chunks
ACK cadence
poll min
poll max
active poll duration
close check threshold
```

Результат должен учитывать разные профили:

* интерактивный;
* balanced;
* bulk/cost-saving.

Допускается добавить presets:

```text
interactive
balanced
bulk
```

Но не добавлять presets без чётко зафиксированных параметров и benchmark justification.

Минимально требуется один новый default profile.

# Сохранение результатов

После каждого этапа сохранять отдельные JSON results:

```text
benchmarks/results/
  baseline-v1-memory.json
  baseline-v1-simulated-s3.json

  p1-ack-cache-memory.json
  p1-ack-cache-simulated-s3.json

  p1-final-ack-memory.json
  p1-final-ack-simulated-s3.json

  p1-close-check-memory.json
  p1-close-check-simulated-s3.json

  p1-list-lifecycle-memory.json
  p1-list-lifecycle-simulated-s3.json

  p1-chunk-negotiation-memory.json
  p1-chunk-negotiation-simulated-s3.json

  p1-aggregation-memory.json
  p1-aggregation-simulated-s3.json

  p1-binary-data-envelope-memory.json
  p1-binary-data-envelope-simulated-s3.json

  p1-activity-wake-memory.json
  p1-activity-wake-simulated-s3.json

  p1-final-memory.json
  p1-final-simulated-s3.json
```

Если P0 определяет другую naming policy, следовать ей, сохраняя возможность сопоставить каждый результат с конкретным этапом.

Каждый JSON должен содержать:

* schema version;
* commit SHA;
* dirty worktree state;
* Go version;
* Android protocol fixture version;
* profile;
* provider;
* directional configured chunk limits;
* directional negotiated chunk limits;
* effective chunk sizes;
* flush delay;
* polling parameters;
* close check threshold;
* window chunks;
* envelope format;
* scenario parameters;
* operation counts;
* latency percentiles.

# Traffic patterns

После каждого этапа запускать минимум:

```text
one-byte-echo-active
one-byte-echo-after-idle
small-chatty-writes
bulk-one-mib
short-connections
concurrent-idle-sessions
mixed-traffic
```

Для aggregation дополнительно:

```text
single-byte-writes
small-writes-burst
writes-below-chunk
writes-at-chunk
writes-above-chunk
EOF-before-deadline
deadline-before-full
```

Для chunk limits:

```text
client-smaller-limit
server-smaller-limit
asymmetric-limits
minimum-limit
maximum-limit
invalid-limit
```

Для pagination:

```text
1001-open-sessions
multiple-list-pages
invalid-open-object
concurrent-list-rounds
```

# Сравнительный отчёт

Создать:

```text
benchmarks/reports/p1-v1-optimizations.md
```

Отчёт должен содержать:

## Общая таблица

```text
| Stage | Scenario | PUT | GET hit | GET miss | HEAD | LIST | DELETE | Total ops | Δ ops |
```

## Bulk

```text
ACK GET per data PUT
operations per MiB
data objects per MiB
throughput
plaintext bytes per object
sealed bytes per object
sealed/plaintext ratio
```

## Chatty traffic

```text
socket reads
data objects
data PUT
average object plaintext size
flush reasons
request/response latency
```

## Short sessions

```text
operations per session
ACK PUT per session
HEAD per session
LIST cost
time to open result
```

## Idle sessions

```text
GET miss per second
HEAD per second
LIST per second
operations per session per second
```

## Post-idle

```text
one-byte RTT p50/p95/p99
poll wakeups
GET misses before response
total operations
```

## Envelope

```text
plaintext bytes
sealed bytes
wire expansion
CPU time
allocations
```

Для каждого изменения показать:

* абсолютный результат;
* отличие от baseline;
* отличие от предыдущего этапа;
* положительный эффект;
* отрицательный эффект;
* решение оставить или отклонить изменение.

# Protocol documentation

Обновить `docs/PROTOCOL.md`.

Явно описать:

1. Обязательный `max_receive_chunk_size` в `OpenRequest`.
2. Обязательный `max_receive_chunk_size` в `OpenResult`.
3. Directional semantics.
4. Effective send max.
5. Hard minimum и maximum.
6. Ошибки handshake.
7. Size-or-deadline aggregation.
8. Flush reasons.
9. Binary data envelope.
10. ACK cache не меняет ACK wire format.
11. Data key layout не меняется.
12. Sequence semantics не меняются.
13. Close markers сохраняются.
14. Старые клиенты и серверы несовместимы с обновлённой реализацией.
15. Несмотря на breaking wire update, общая архитектура protocol v1 сохраняется.

Обновить также:

* `docs/PERFORMANCE.md`;
* Android protocol documentation;
* configuration documentation;
* benchmark README;
* release notes.

# Конфигурация

Добавить или обновить:

```text
--chunk-size
--flush-delay
--active-poll-duration
--close-check-after-misses
--window-chunks
--poll-min
--poll-max
```

В документации разделить:

```text
configured send chunk size
configured receive chunk limit
peer advertised receive limit
effective send chunk size
```

Если один `--chunk-size` используется и как send target, и как receive limit, это должно быть явно описано.

Более гибкий вариант:

```text
--send-chunk-size
--receive-chunk-limit
```

Выбрать интерфейс, который не создаёт неоднозначности.

Все значения должны поддерживаться через environment variables и package configuration.

Android configuration должна использовать ту же семантику.

# Production safety

* Ограничивать все размеры hard maximum.
* Проверять integer overflow.
* Не выделять buffer по непроверенному remote value.
* Не логировать session IDs в labels.
* Не логировать credentials и PSK.
* Не менять target policy.
* Не ослаблять AES-GCM.
* Не изменять nonce uniqueness.
* Не использовать unbounded queues.
* Не создавать goroutine на каждый polling attempt.
* Не допускать бесконечного accumulation при stalled S3.
* Не допускать unlimited benchmark output.
* Все новые collectors должны быть thread-safe.

# CI

В обычный CI включить:

* Go unit tests;
* protocol tests;
* aggregation tests;
* binary envelope tests;
* pagination tests;
* memory traffic smoke;
* deterministic operation-count tests;
* Android JVM tests;
* cross-language crypto fixtures;
* race tests для изменённых Go packages, если время позволяет.

Не включать как blocking:

* real S3;
* строгие wall-clock thresholds;
* полную parameter matrix;
* длительный simulated-S3 run.

Добавить manual workflow для полного P1 performance run, если это соответствует существующей CI-архитектуре.

# Критерии готовности

Задача завершена, когда:

1. Все P1-изменения реализованы либо конкретное изменение отклонено на основании сохранённых измерений.
2. `go test ./...` проходит.
3. Android JVM tests проходят.
4. Race tests проходят либо указана точная причина невозможности запуска.
5. Более 1000 open objects корректно обрабатываются.
6. Open objects удаляются после принятия.
7. In-flight session state не растёт бесконечно.
8. ACK GET per data PUT уменьшен.
9. Ненужный final ACK удалён.
10. HEAD close count уменьшен или не увеличен.
11. Handshake содержит обязательные directional chunk limits.
12. Effective chunk никогда не превышает peer receive limit.
13. Aggregation не теряет и не переставляет bytes.
14. Chatty traffic создаёт меньше data objects.
15. Binary envelope существенно уменьшает sealed/plaintext ratio.
16. Activity wake уменьшает post-idle latency либо отклонён по измерениям.
17. Idle request rate остаётся ограниченным.
18. Сохранены промежуточные JSON results.
19. Сгенерирован итоговый Markdown report.
20. Основная архитектура mailbox protocol v1 не изменена.
21. Multiplexing нескольких SOCKS sessions не реализован.
22. Старые клиентские и серверные версии явно объявлены несовместимыми.

# Итоговый ответ

В конце работы предоставить:

1. Краткое описание архитектурных изменений.
2. Список изменённых файлов.
3. Описание directional chunk negotiation.
4. Описание aggregation.
5. Спецификацию binary data envelope.
6. Команды запуска тестов.
7. Команды запуска benchmark profiles.
8. Результаты:

   * `go test ./...`;
   * race tests;
   * Android tests;
   * memory profile;
   * simulated-S3 profile.
9. Ссылки на JSON results.
10. Ссылку на итоговый Markdown report.
11. Таблицу baseline против final P1.
12. Какое изменение дало наибольшее уменьшение S3 operations.
13. Какое изменение дало наибольшее уменьшение latency.
14. Какое изменение дало наибольшее уменьшение wire bytes.
15. Какие изменения дали отрицательный или нулевой эффект.
16. Выбранные defaults и обоснование измерениями.
17. Известные ограничения.
18. Рекомендацию следующего этапа без реализации multiplexed protocol v2.


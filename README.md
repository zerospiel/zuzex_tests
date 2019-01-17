# README

В этой документации указаны основные принципы работы каждого из репозиториев, на что следует обратить внимание, как работать с кодогенерацией, как работать на фронте, что делать при работе над фичей одновременно в нескольких репозиториях.

Также перечислены общие дальнейшие действия, которые желательно бы было сделать, что происходит на сервере и как там устроен проект, некоторые моменты по поводу CI и ещё пара мелочей.

При возникновении вопросов можно [писать на мой личный email](mailto:ww@bk.ru).

## Содержание

- [Код и принцип работы](#code-main)
  - [Репозитории](#repos)
    - [ADDRESS](#address)
    - [API](#api)
      - [Go-Swagger](#api-swagger)
    - [APP](#app)
      - [Go-Swagger](#app-swagger)
      - [Front-end](#app-front)
    - [FETCHER](#fetcher)
    - [IMPORTER](#importer)
    - [INDEXER](#indexer)
    - [IO](#io)
    - [MUNROLL](#munroll)
    - [NORMALIZER](#normalizer)
    - [PREIMPORT](#preimport)
    - [SCRAPER](#scraper)
    - [UTILS](#utils)
- [Deploy](#deploy)
  - [CI](#wercker)
- [Сервер (DigitalOcean)](#serverside)
  - [Dokku](#dokku)
  - [Docker](#docker)
- [Сборка, запуск, разработка](#build-main)
  - [Установка](#prereq)
  - [Сборка проекта](#build-project)
  - [Разработка в нескольких репозиториях](#dep-work)
- [TODO](#todo)
  - [Оркестрация](#swarm)
  - [Разбан сервера](#crawlera)
  - [Preimport as a Service](#preimport-aas)
  - [Normalizer as a Service](#normalizer-aas)
  - [Включить PapperTrail](#paper-logger)
- [Другие README](#other-readme)

<div id="code-main"/>

## Код и принцип работы

Здесь и далее перечислены основные репозитории и особенности их работы. Репозиторий **normapi** не указан в связи с устареванием.

Все репозитории были обновлены и частично пересобраны для того, чтобы их можно было собирать и всё работало (до этого сборка была практически неработающая с невозможностью каких-либо обновлений зависимостей). Указанный выше **normapi** также пересобран и обновлён.

*Все репозитории используют менеджер пакетов **[dep][dep-link]***.

<div id="repos"/>

## Репозитории

В проекте имеется основной репозиторий, который выполняет роль главного бэк-энда и в нём же лежит фронт-энд — это репозиторий **[APP](#app)**. Также репозиторий **[API](#api)**, который содержит в себе необходимые для сервера и микросервисов модели, а также код для работы с миграциями.

Остальные репозитории являются *микросервисами*, которые общаются между собой с помощью менеджера сообщений **[RabbitMQ]**.

**Общая схема работы** простая:

1. Каждый микросервис слушает какую-то/какие-то определённые очереди.
2. Пользователь создал задачу, Задача передаётся на сервер.
3. Сервер обрабатывает задачу и передаёт обработку микросервису A через какую-то из очередей.
4. Сервис A получает задачу, обрабатывает данные и посылает промежуточный результат микросервису B через какую-то из очередей для дальнейшей обработки.
5. Микросервис B получает данные из слушаемой очереди, обрабатывает их и по необходимости передаёт куда-то ещё.

**Простой пример**:

1. Пользователь на фронте меняет количество «потоков».
2. Сервер получил число, обработал типы и передал значение в очередь микросервису **[IO](#io)**.
3. Микросервис **IO** получил данные из очереди, обработал их и передал очереди микросервиса **[Scraper](#scraper)**.
4. Микросервис **Scraper** получил данные из очереди, использовав полученное число, изменил ёмкость семафора пула задач.

### Замечание про микросервисы

На сервере на текущий момент имеются микросервисы не под все репозитории (см. **[Normalizer as a Service](#normalizer-aas)** и **[Preimport as a Service](#preimport-aas)**), поэтому количество запущенных бинарников на локальной машине и количество контейнеров с сервисами на сервере могут отличаться в бóльшую сторону на локальной машине. В этом нет ничего страшного, просто стоит знать об этой особенности.

### Замечание про детализированность описаний

Я успел «плотно» поработать лишь с несколькими сервисами, поэтому где-то для сервиса будет намного более детальное описание, а где-то лишь то, с чем я работал при обновлении и пересборке репозиториев.

>*В любом случае, обязательно следует предварительно ознакомиться с кодом и общими принципами работы основных файлов (например, отвечающих за **build** бинарника)*.

<div id="address"/>

## `Address`

Сервис, который отвечает за распознавание адресов. Использует несколько API сторонних сервисов (Google Maps, Smarty Streets).

### Детали работы `Address`

Сервис имеет несложную структуру. Используется 3 различных карточных сервиса:

- [SmartyStreets];
- [Google Maps];
- [YAddress];

Методы по нормализации адреса работают следующим образом:

1. Сначала SmartyStreets пробегается со своим поиском. Если заданный адрес не содержит в себе штата, города и индекса, то SS занимается свободным поиском и валидацией адреса, в другом же случае поиск для адресов США не осуществляется (потому что все данные имеются).
2. Если SS фейлится с поиском/валидацией адреса, то в работу вступают Google Maps (*API для Place*), которые ищут заданный адрес. Если фейлится Place API, то используется *Geocode API*.
3. После поиска GM опять работает SS с адресом, который отдали GM. Если опять фейлится валидация, то из полученного адреса удаляются город и штат, п.2 повторяется.
4. Если опять проиходит фейл, то используется *Google Custom Search API* и опять вызывается API SS.
5. Если и п.4 фейлится, то вызывается *API YAddress*, происходит, насколько возможно, коррекция адреса и опять вызывается SS для нормализации адреса.

### Тесты `Address`

Для директорий `Client`, `Provider`, `Utils` имеются тесты, но они по большей части нерабочие, я их не правил, поэтому надо быть внимательным при работе с кодом, либо переписать тесты.

***Сейчас почти все тесты существуют с вызовом `t.Skip()`!***

### Ключи API `Address`

В этом репозитории (в конфиге) и в настройках контейнера [на сервере](#serverside) ***используются ключи первого разработчика*** для Google Maps и для SmartyStreets. В случае второго сервиса ключ от бесплатного аккаунта, поэтому SmartyStreets работают ***только с адресами США***.

В идеале надо создать свои собственные ключи и отдать аккаунты клиенту.

### **Индус в `Address`**

Многие изменения в коде в этом репозитории были сделаны индусом, поэтому многие функции, методы или какие-то строки кода могут показаться абсолютно нелогичными (с точки зрения исполнения, именования или вызова).

В этом репозитории я избавлялся от откровенно ужасных кусков кода и допилил лишь пару функций.

<div id="api"/>

## `API`

Репозиторий с сгенерированными моделями для использования на бэке сервера и в других микросервисах, основная программа для создания миграций.

### Детали работы `API`

В этом репозитории почти нет особых деталей работы, миграции сидят на пакете [SQLMigrate]. Миграции работают в связке с пакетом [Go-Bindata], которому необходимо скормить все файлы `*.sql` из директории `migrations` и манифест `swagger`:

```sh
$ $(GOPATH)/go-bindata -pkg pgtransp -o path_to_api/client/pgtransp/bindata.go path_to_api/fixed.yaml $(find path_to_api/client/pgtransp/migrations -type f -name "*.sql")
# отдаёт пакету манифест и все миграции
# команду не следует запускать руками, она присутствует в Makefile
```

Сгенерированный файл `bindata.go` и используется при создании миграций.

Следует обратить внимание на пакет `pgtransp`, в котором находится код для обработки различных API запросов, например запросов `PATCH`, `DELETE` или `GET`. В файле `query.go` написаны методы для обработки структур, объявленных в манифесте `swagger`, чтобы сматчить их с таблицами БД.

>***Замечание***: *на бэке я написал пару методов, они что-то делают с БД, я писал прямые `SQL` запросы к БД, не понимая того, каким образом работают условные методы удаления без фактического обращения к базе. Потратив пару минут, я обнаружил, что имеются файлы типа `delete_*.go` или `post_*.go` в этом репозитории, где на самом деле и происходит обращение к базе (через ORM). Это замечание стоит перепроверить, если всё так, то методы удаления, патча или обычного поста ­— следует обрабатывать в репозитории `API`, таким образом унифицируя все обработчики на бэке.*

### Тесты `API`

Имеется обширное покрытие тестами почти всех используемых пакетов, тесты рабочие, но следует учесть два фактора:

1. Тесты есть не для всех функций.
2. Тесты прогоняются на маленьком количестве данных (кейсов).

В связи с этим следует либо **дописывать тесты по мере использования** и модификации репозитория, либо действовать на свой страх и риск, даже если все тесты успешно завершаются.

### Использование CLI `API`

```sh
$ $(GOPATH)/api-cli init
# обычная инициализация по дефолту к БД с названием snl, не несёт полезной функции
$ $(GOPATH)/api-cli migrate
# применяет миграции из директории `client/pgrtransp/migrations` по дефолту к БД с названием snl
$ $(GOPATH)/api-cli migrate down
# откатить миграции
```

<div id="api-swagger"/>

## Go-Swagger `API`

Проект активно использует кодогенерацию через [Go-Swagger], в частности репозиторий `API`, в котором генерируются все модели и базовые операции над ними, которые в дальнейшем используются в `APP` и остальных микросервисах. Немного ознакомиться с тем, что такое Swagger, Go-Swagger и Open API — можно на [официальном сайте с документацией][swagger-off] или [в этом блоге][swagger-blog].

В этом репозитории необходимо лишь объявлять новые модели (структуры) и операции (функции), генерируя таким образом `REST API` клиента.

*Сложный пример сложной структуры*:

```yaml
...
definitions:
  ...
  BackgroundTaskSerializer:
    properties:
      id:
        format: long
        type: integer
      created_by:
        format: long
        type: integer
      action:
        type: string
        enum:
          - upload
          - export
          - restart
          - run_bots
          - munroll
          - import
          - export_leads
          - reindex_property
          - reindex_property_all
          - reindex_addresses_all
          - fetch
          - run_bots_deceased_dade
          - run_bots_miami
          - run_bots_broward
          - run_bots_duval
          - run_bots_orange
          - run_bots_osceola
          - run_bots_palm-beach
          - run_bots_pasco
          - run_bots_pinellas
          - run_bots_seminole
          - run_bots_volusia
          - run_bots_hillsborough
          - run_bots_realtdm

      done:
        type: boolean
      hidden:
        type: boolean
      sent:
        type: boolean
      scheduled_on:
        type: string
        format: date-time
      recurrent_in:
        type: integer
        format: long
      total:
        format: int
        type: integer
      processed:
        format: int
        type: integer
      payload:
        type: object
        properties:
          import:
            $ref: '#/definitions/ImportUploadSerializer'
          files:
            type: array
            items:
              type: string
          email:
            type: string
          records:
            type: array
            items:
              type: string
          uploads:
            type: array
            items:
              type: integer
              format: int
          upload:
            $ref: '#/definitions/ScrapperUploadSerializer'
          appraisal:
            type: boolean
          bills:
            type: boolean
          preimport:
            type: boolean
          replace:
            type: boolean
          fullUploads:
            type: array
            items:
              $ref: '#/definitions/ScrapperUploadSerializer'
          munroll:
            $ref: '#/definitions/MunrollUploadSerializer'
          criteria:
            type: string
          leads:
            $ref: '#/definitions/ExportLeadsSerializer'
          ids:
            type: array
            items:
              type: integer
              format: int
        additionalProperties:
          type: object
      result:
        type: object
        additionalProperties:
          type: object
      err:
        type: string
      created_on:
        type: string
        format: date-time
      updated_on:
        type: string
        format: date-time
    required:
    - created_by
    - action
    - err
    - done
    - sent
    - hidden
    - created_on
    - updated_on
    - scheduled_on
    title: BackgroundTaskSerializer
    type: object
  ...
```

*Пример простой операции*:

```yaml
...
paths:
  ...
  /api/background/tasks/delete:
    delete:
      x-table: background_task
      description: Destroy `Background Task` ((plain-detail))
      operationId: Delete_Background_Task_DELETE_
      parameters:
      - name: identifiers
        in: body
        schema:
          type: array
          items:
            type: integer
            format: long
      produces:
      - application/json
      responses:
        "204":
          description: No Content
        "400":
          description: Error
          schema:
            $ref: '#/definitions/Error'
  ...
```

При пересборке репозиториев я столкнулся с тем, что возникала коллизия в названиях пакетов в репозитории `APP` с его собственными пакетами. Использовался некорректный пакет для некоторых операций, где объявлялись сложные возвращаемые значения.

Из-за этого эти значения не были видны. Я обернул их в рекурсивную структуру. Несмотря на то, что в примере выше также используются рекурсивные структуры, файлы генерировались некорректно, пришлось немного поменять обработку манифеста для правильной обработки структур и последующего их использования с базой.

*Пример простой рекурсивной структуры в* `definitions`:

```yaml
...
paths:
  ...
  BackgroundTaskRespOK:
    properties:
      count:
        format: long
        type: integer
      results:
        items:
          $ref: '#/definitions/BackgroundTaskSerializer'
        type: array
    title: PListBackgroundTaskSerializer
    type: object
  ...
```

> ***Совет***: *Из-за того, что файл манифеста может показаться большим, следует использовать [онлайн-редактор][swagger-editor], который сразу подскажет имеются ли какие-то ошибки в новой объявленной структуре или операции.*

После описания манифеста, остаётся лишь сгенерировать файлы клиентского API. Сделать это можно руками или же через билд проекта (см. [сборку проекта](#build-project)):

```sh
$ $(GOBIN)/swagger generate client -A api -f path_to_api/fixed.yaml
# будут сгенерированы все необходимые структуры и их методы на основе указанного файла
```

<div id="app"/>

## `APP`

Главный репозиторий, где лежит и частично генерируется (через `webpack`) front-end, а также обрабатываются запросы `REST API`. Все модели и сам манифест `swagger` из репозитория `API` лежат здесь. Также в этом репозитории есть свой собственный манифест `swagger`, о нём [ниже](#app-swagger).

Модели `API` клиента здесь не генерируются, а используются из репозитория `API`. Чтобы они были используемы, при билде я произвожу следующую хитрую операцию:

```sh
$ find path_to_app/rest/operations -type f -name "*.go" -print0 | xargs -0 sed -i '/\tmodels/s#/app/models#/api/models#g'
# эту команду не надо запускать руками, она заменяет все использования пакетов моделей (структур) app в операциях на api, в app этих файлов по итогу нет, а в api они есть, в этом репозитории они цепляются из vendor'а
```

### Детали работы `APP`

Главный пакет — `handlers`, в котором лежат все обработчики `REST API`, эти обработчики необходимо вешать на сгенерированные функции операций в файле `configure_app.go` пакета `rest`. Как только будет написан обработчик и объявлен в этом файле — новый метод API сразу же заработает.

Маленький пакет `claims` помогает в авторизации, генерации и верификации токенов авторизации.

В пакете `operations` лежат сгенерированные для сервера функции, их не требуется рассматривать.

>***Замечание***: *на текущий момент в проекте имеется менее 50% готовых обработчиков для сгенерированных функций.*

Следует обратить внимание на файл `handlers.go` в пакете `handlers`, в нём присутствует конструктор, пара полезных методов и метод `pushTask()`, который и отвечает за **передачу данных от сервера к другим микросервисам**. Конкретно этот метод передаёт данные очереди `actions` — дефолтное название очереди сервиса [IO](#io).

Присутствуют 2 пакета для бинарников:

1. Бинарник `app-cli`, который используется для расширения манифеста `swagger` текущего репозитория манифестом из репозитория `API`.

```sh
$ app-cli extend path_to_app/vendor/bitbucket.org/dadebotsdb/api/fixed.yaml > path_to_app/swagger.yaml
# эту команду также не следует запускать руками, пример того, для чего нужен бинарник `app-cli`
```

2. Бинарник `app-server`, который и **является сервером**, у него есть несколько ключей

```sh
$ app-server --help
# вывод возможных ключей запуска сервера
$ app-server --host="localhost" --port 50000 --scheme http
# стандартный пример, который используется при выполнении `make serve`
# присутствует возможность запуска https, но на локалке это не имеет смысла
# на сервере я уже настроил всё для работы по HTTPS по дефолту
```

Более подробно про работу со [Swagger](#app-swagger) и [фронтом](#app-front) читать в соответствующих разделах.

### Тесты `APP`

**Тесты в этом репозитории отсутствуют** (за исключением 1 файла с 1 функцией). Их следует написать самому.

<div id="app-swagger"/>

## Go-Swagger `APP`

Манифест `swagger` в этом репозитории устроен проще, чем в репозитории `API`. Здесь требуется добавлять лишь те операции, которых нет в манифесте `API`, при этом **модели добавлять не имеет смысла**, потому что они скипаются при билде.

*Пример добавления операции*:

```yaml
...
paths:
  ...
  /api/token:
    post:
      consumes:
      - application/x-www-form-urlencoded
      description: Get JWT token with credentials of oauth token
      operationId: Get_JWT_Token
      parameters:
      - description: google plus oauth code
        in: formData
        name: gtoken
        type: string
      - description: login
        in: formData
        name: login
        type: string
      - description: password
        in: formData
        name: password
        type: string
      responses:
        "201":
          description: '...'
          schema:
            type: string
      summary: Get JWT token
      tags:
      - auth
  ...
```

<div id="app-front"/>

## Front-End

Весь фронт расположен в директориях `client` и `data`, при этом последняя директория генерируется за счёт `webpack`. Поэтому весь код для изменения или добавления следует размещать в `client`. Там же хранятся все роуты, компоненты и формы.

Проект написан на **`React.js`** и **`ECMAScript 6`**, что очень удобно и просто. Формы и большинство интерфейсных объектов фронта написаны с использованием **`Redux`**.

>***Замечание***: *при пересборке фронта не получится использовать вотчер `webpack`, потому что нужно полностью перегенерировать все файлы из `data` и пересобрать заново бинарник сервера. Это не очень удобно и занимает некоторое время, зато это не сильно сложнее, чем запустить вотчер:*

```sh
$ pwd
# $GOPATH/bitbucket.org/dadebotsdb/app
$ make serve
# пересборка сервера и запуск его с переменной среды DEVELOPMENT
$ ps aux | grep app-server | grep -v grep
# ... app-server PID ...
```

В головной директории лежат настройки линтера `eslint`, его необходимо запускать перед каждым пушем в репозиторий (подробнее в [CI](#wercker)):

```sh
$ pwd
# $GOPATH/bitbucket.org/dadebotsdb/app
$ ./.node_modules/.bin/eslint ./client
# запуск линтера по всему фронту, скорее всего будет >50000 критических ошибок
$ ./.node_modules/.bin/eslint --fix ./client
# скорее всего будет 0 критических ошибок
```

Подробнее про различные ключи можно [прочесть здесь][eslint-cmd].

<div id="fetcher"/>

## `Fetcher`

Сервис, который делает запросы к сайтам, которые парсятся. ***Все запросы делаются через proxy-сервер `Crawlera`, поэтому IP сервера не должен оказаться в бане*** (см. [TODO: разбан сервера](#crawlera)). Очень простой и понятный микросервис.

### Детали работы `Fetcher`

Сервис всегда слушает очередь `fetcher_rpc`, в которую поступают данные от сторонних сервисов при вызове метода `Call()` инстанса клиента сервиса `Fetcher`.

Основной функционал реализован в файле `server.go` и не является сложным. Функционал клиента лежит в пакете `client` и также простой.

>*С примерами можно ознакомиться в коде других сервисов (напр., [Indexer](#indexer)), из [других ReadMe](#other-readme) или с помощью следующей простой команды*:

```sh
$ find path_to_fetcher -maxdepth 2 -type f -name "*.md" | tail -1 | xargs less
# в открытом less будет присутствовать пример использования клиента
```

### Тесты `Fetcher`

**Тесты** для текущего репозитория **неполные**, но присутствуют. Желательно увеличить процент покрытия кода тестами. Тесты не требуют запуска с ключом `-race`

```sh
$ pwd
# $GOPATH/bitbucket.org/dadebotsdb/fetcher
$ go test -v ./client -run='.'
# простой запуск тестов для пакета `client`
```

<div id="importer"/>

## `Importer`

Сервис для импорта записей в систему. Сохраняет спарсенные данные в `Elastic Search`.

### Детали работы `Importer`

>***Замечание***: *я почти не работал с этим сервисом, поэтому не могу написать никаких особых деталей.*

Сервис публикует данные в очередь `actions` сервиса [IO](#io), скорее всего уже обработанные в `Elastic`.

Основные действия по обработке импортируемых записей происходят в одном файле `api.go` с множеством обращений к БД.

### Тесты `Importer`

В репозитории имеется несколько тестов для пакета `parser`, запускать обязательно с флагом гонки:

```sh
$ pwd
# $GOPATH/bitbucket.org/dadebotsdb/importer
$ go test -v -race ./parser -run='.'
```

<div id="indexer"/>

## `Indexer`

Сервис получает сообщения о действиях из очереди `actions` и начинает выполнение поступившего действия. Сервис, по сути, отвечает за работу ботов-парсеров.

### Детали работы `Indexer`

Репозиторий удобно разбит на множество пакетов для простоты работы:

- generic — основной бот, который получает данные от всех скраперов

[//]: # (TODO: start ending up the doc from here)

### Тесты `Indexer`

<div id="io"/>

## `IO`

<div id="munroll"/>

## `Munroll`

<div id="normalizer"/>

## `Normalizer`

<div id="preimport"/>

## `Preimport`

<div id="scraper"/>

## `Scraper`

<div id="utils"/>

## `Utils`

<div id="deploy"/>

## Deploy

<div id="wercker"/>

## CI `Wercker`

<div id="serverside"/>

## Сервер (DigitalOcean)

<div id="dokku"/>

## `Dokku`

<div id="docker"/>

## `Docker`

<div id="build-main"/>

## Сборка, запуск, разработка

<div id="prereq"/>

## Установка

<div id="build-project"/>

## Сборка проекта

<div id="dep-work"/>

## Разработка в нескольких репозиториях

<div id="todo"/>

## TODO

<div id="swarm"/>

## Оркестрация

<div id="crawlera"/>

## Разбан сервера

<div id="preimport-aas"/>

## Preimport as a Service

<div id="normalizer-aas"/>

## Normalizer as a Service

<div id="paper-logger"/>

## Включить PapperTrail

<div id="other-readme"/>

## Другие README

[//]: # (Секция комментариев)

   [dep-link]: <https://github.com/golang/dep>
   [swagger-blog]: <https://posener.github.io/openapi-intro/>
   [swagger-off]: <https://swagger.io>
   [swagger-editor]: <https://editor.swagger.io>
   [eslint-cmd]: <https://eslint.org/docs/user-guide/command-line-interface>
   [RabbitMQ]: <https://www.rabbitmq.com>
   [SmartyStreets]: <https://smartystreets.com>
   [Google Maps]: <https://cloud.google.com/maps-platform/>
   [YAddress]: <https://www.yaddress.net>
   [SQLMigrate]: <https://github.com/rubenv/sql-migrate>
   [Go-Swagger]: <https://github.com/go-swagger/go-swagger/cmd/swagger>
   [Go-Bindata]: <https://github.com/jteeuwen/go-bindata/>
# gologger

**Русский** | [English](README.en.md)

Настраиваемый логер на базе стандартного `log/slog`: JSON и читаемый текстовый
вывод, атрибуты из контекста, несколько получателей записей и расширение через
собственные обработчики и middleware.

Требуется **Go 1.27+**. Рекомендуемый toolchain — **Go 1.27.1**.
Используются стандартные
[`slog.NewMultiHandler`](https://pkg.go.dev/log/slog@go1.27.0#NewMultiHandler)
и `slog.DiscardHandler`.

## Установка

```sh
go get github.com/assurrussa/gologger@latest
```

Путь импорта: `github.com/assurrussa/gologger`, имя пакета — `gologger`.

## Быстрый старт

Пример тела функции, возвращающей `error`. Нужны импорты `context`, `log/slog`,
`os` и `github.com/assurrussa/gologger`.

```go
log, err := gologger.New(gologger.Config{Level: "info"},
    gologger.WithWriter(os.Stdout),
)
if err != nil {
    return err
}

ctx := gologger.WithValue(context.Background(), slog.String("request_id", "req-1"))
log.WithNamed("worker").InfoContext(ctx, "started")
return log.Close()

```

В приложении вызывайте `Close` после завершения всех пользователей логера.
[Исполняемый пример](example_test.go) также проверяется командой `go test ./...`.

`New` создает независимый экземпляр: импорт пакета и создание логера не меняют
`LogLevel` или `slog.Default()`. Для глобальной настройки приложения вызовите
`gologger.SetDefault(log)` явно.

## Конфигурация

Поведение `gologger.New` без дополнительных опций:

| Поле `Config` | Значение по умолчанию | Поведение |
| --- | --- | --- |
| `Level` | WARN при пустой строке | Минимальный уровень. Неверное значение возвращает ошибку. |
| `Env` | Пустая строка | `local` и `development` включают читаемый pretty-формат; остальные значения — JSON. |
| `JSON` | `false` | `true` принудительно включает JSON и в локальном окружении. |
| `AddSource` | `false` | Добавляет файл и строку вызова. |
| `AddVerbose` | `false` | Добавляет `program_info`: PID, версию программы и Go. |
| `Rate` | `0` | Вероятность записи при `0 < Rate < 1`; `0` и `1` сохраняют все записи. |
| `Output` | Пустая строка | Путь дополнительного буферизованного JSON-файла. |

Основной вывод направляется в stdout. `Config.Output` добавляет файл, сохраняя
основной вывод, в том числе при использовании собственного обработчика.

`New` не читает переменные окружения и не применяет теги `value-default` или
`validate` самостоятельно. Теги в `Config` предназначены для конфигурационного
слоя приложения. Опции применяются поверх конфигурации.

## Настройка и расширение

- `WithWriter(io.Writer)` заменяет stdout у встроенного обработчика.
- `WithHandler(slog.Handler)` заменяет встроенный обработчик. Переданный
  обработчик сам определяет уровень, `AddSource` и `ReplaceAttr`.
- `WithAdditionalHandlers(...OptionHandler)` добавляет получателей записей.
  Каждая фабрика получает отдельную копию `slog.HandlerOptions`; результат `nil`
  пропускается.
- `WithMiddleware(...Middleware)` оборачивает общий обработчик. Middleware
  вызываются в порядке перечисления: первый — снаружи остальных.
- `WithLevel(slog.Leveler)` переопределяет `Config.Level`, не меняя переданное
  значение. `*slog.LevelVar` позволяет менять уровень во время работы.
- `WithReplaceAttr` настраивает фильтрацию, переименование или редактирование
  атрибутов встроенных обработчиков и дополнительных фабрик.
- `WithClosers(...io.Closer)` передает логеру ответственность за закрытие
  дополнительных ресурсов после успешного `New`.

Пример с динамическим уровнем, дополнительным выводом в stderr, редактированием
атрибута и middleware. Нужны импорты `log/slog`, `os` и `github.com/assurrussa/gologger`.

```go
level := new(slog.LevelVar)
level.Set(slog.LevelDebug)

log, err := gologger.New(gologger.Config{},
    gologger.WithLevel(level),
    gologger.WithAdditionalHandlers(func(opts *slog.HandlerOptions) slog.Handler {
        return slog.NewTextHandler(os.Stderr, opts)
    }),
    gologger.WithReplaceAttr(func(_ []string, attr slog.Attr) slog.Attr {
        if attr.Key == "password" {
            return slog.String(attr.Key, "[redacted]")
        }
        return attr
    }),
    gologger.WithMiddleware(func(next slog.Handler) slog.Handler {
        return next.WithAttrs([]slog.Attr{slog.String("service", "api")})
    }),
)
if err != nil {
    return err
}
log.Info("ready", "password", "example")
level.Set(slog.LevelWarn)
return log.Close()

```

`WithLevel` влияет на встроенные обработчики и фабрики, использующие переданные
им опции. У обработчика из `WithHandler` собственный порог. Изменение `LevelVar`
затрагивает только логеры, которым явно передан этот указатель.

Порядок обработки: **атрибуты контекста → middleware → sampling → получатели**.
Обработчики и middleware должны сохранять контракт `slog.Handler`: `Enabled`,
`WithAttrs`, `WithGroup` и безопасность конкурентных вызовов. Для общего writer,
который используется несколькими независимыми обработчиками, синхронизацию
обеспечивает вызывающий код.

## Контекст и дочерние логеры

`WithValue(ctx, slog.Attr)` создает контекст с дополнительным атрибутом. Передайте
его в `InfoContext`, `DebugContext`, `WarnContext`, `ErrorContext` или `LogAttrs`,
чтобы атрибут попал в запись. Дочерние контексты не изменяют атрибуты друг друга.

`WithNamed` добавляет поле `name`, `WithAttrs` — постоянные атрибуты. Оба метода
возвращают интерфейс `Logger` и сохраняют общие ресурсы с родителем. Встроенный
`*slog.Logger` доступен через поле `Logger`; унаследованные `With` и `WithGroup`
возвращают обычный `*slog.Logger`. Ресурсы закрывает исходный `*gologger.Log`.

`Error(err)` создает строковый атрибут `error`; передавайте ненулевую ошибку.
`Discard` создает логер без вывода, а `DiscardJSONWithWriter` и
`DiscardTextWithWriter` — логеры для переданного writer. Эти функции используют
общий `LogLevel`.

## Sampling

Для `0 < Rate < 1` библиотека принимает одно случайное решение на запись, поэтому
все получатели видят одинаковый набор выбранных записей. Sampling применяется ко
всем уровням, включая ERROR. Значения `0` и `1` отключают sampling.

Отрицательные значения, значения больше `1` и NaN в `New` возвращают ошибку.

## Публичные обработчики

Все пути ниже начинаются с `github.com/assurrussa/gologger/`:

- `handlers/slogcontext` — добавляет атрибуты из контекста.
- `handlers/slogpretty` — читаемый локальный вывод с группами, `LogValuer`,
  `ReplaceAttr` и информацией об источнике вызова.
- `handlers/slogdiscard` — обработчик без вывода.

Обработчики работают с обычным `slog.New`, без `gologger.Log`.

## Ресурсы и завершение работы

`Config.Output` создает новый файл с правами `0600`; права существующего файла
не меняются. Данные буферизуются и сохраняются при закрытии логера.

`Close()` сбрасывает буферы и закрывает принадлежащие логеру ресурсы один раз.
`Flush()` — совместимое имя для того же завершающего действия, а не периодический
сброс буфера. Все вызовы, включая конкурентные вызовы через дочерние логеры,
возвращают сохраненную ошибку закрытия. Перед закрытием остановите пользователей
логера; запись после закрытия не поддерживается.

Переданные writer и обработчик по умолчанию остаются в собственности вызывающего
кода и не закрываются библиотекой. `WithClosers` передает ответственность только
после успешного конструктора. Ресурсы закрываются в обратном порядке; каждый
`io.Closer` должен сам сбрасывать свои буферы. Библиотека не запускает горутины
или фоновые таймеры.

## Глобальный логер

`SetDefault(log)` устанавливает логер пакета и `slog.Default()`. Предыдущий
экземпляр остается под управлением приложения и автоматически не закрывается.
`Default()` возвращает текущий логер пакета.

`NewLogger` создает и устанавливает глобальный логер, меняет общий `LogLevel`,
использует WARN при неверном уровне, игнорирует `JSON` в `local`/`development`
и трактует rate вне `(0, 1)` как отключенный sampling. Для независимых экземпляров
используйте `New`. Общий `LogLevel` меняют через `.Set`, не заменой указателя.

## Проверки

Нужны Go 1.27+ и `golangci-lint`:

```sh
make check
./scripts/test-consumer.sh <published-version-or-commit>
```

`make check` запускает vet, lint и race-тесты пять раз. Consumer probe создает
отдельный Go-модуль с `GOWORK=off`, загружает указанную опубликованную версию без
`replace` и проверяет публичные импорты с race detector.

## Лицензия

[MIT](LICENSE).

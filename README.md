# gologger

Настраиваемый логер на базе стандартного `log/slog`, выделенный из
`github.com/assurrussa/goshared/pkg/logger`. Модуль не зависит от `goshared`.

Требуется Go 1.26+; рекомендуемый toolchain и финальные проверки — Go 1.27.1. Используются стандартные
[`slog.NewMultiHandler`](https://pkg.go.dev/log/slog@go1.27.0#NewMultiHandler)
и `slog.DiscardHandler`; зависимостей `slog-multi` и `slog-sampling` нет.

```sh
go get github.com/assurrussa/gologger@latest
```

## Создание

```go
log, err := gologger.New(gologger.Config{Level: "info", JSON: true},
    gologger.WithWriter(os.Stdout),
)
if err != nil {
    return err
}
// После завершения всех пользователей логера:
defer func() { _ = log.Close() }()

ctx := gologger.WithValue(ctx, slog.String("request_id", "req-1"))
log.WithNamed("worker").InfoContext(ctx, "started")
```

Импорты: `github.com/assurrussa/gologger`, `log/slog`, `os`.
Исполняемый пример: `example_test.go`.

`New` создает независимый экземпляр и не меняет глобальные логеры, `LogLevel`
или `slog.Default()`. Сам импорт модуля также не меняет `slog.Default()`.
Глобальную настройку host выполняет явно через `gologger.SetDefault(log)`.

По умолчанию используется JSON в stdout и уровень WARN. Для `local` и
`development` включается pretty-формат; `JSON: true` сохраняет JSON и в этих
окружениях. Неверный уровень или sampling rate вне диапазона `[0, 1]` возвращает
ошибку. `Rate: 0` и `Rate: 1` отключают sampling; промежуточное значение задает
вероятность записи. Решение о записи общее для всех destinations.

## Настройка и расширение

- `WithWriter(io.Writer)` заменяет stdout встроенного handler.
- `WithHandler(slog.Handler)` заменяет встроенный handler на любой совместимый
  со стандартным `slog`. У такого handler собственные настройки уровня,
  `AddSource` и `ReplaceAttr`.
- `WithAdditionalHandlers(...OptionHandler)` добавляет destinations. Каждая
  фабрика получает отдельную копию `slog.HandlerOptions`; результат `nil`
  пропускается.
- `WithMiddleware(...Middleware)` оборачивает общий handler. Порядок вызовов:
  context-атрибуты → middleware в порядке перечисления → sampling → destinations.
  Middleware обязан сохранять `Enabled`, `WithAttrs`, `WithGroup` и безопасность
  конкурентных вызовов по контракту `slog.Handler`.
- `WithLevel(slog.Leveler)` переопределяет `Config.Level`; `*slog.LevelVar`
  позволяет менять уровень во время работы, не затрагивая другие экземпляры.
- `WithReplaceAttr` настраивает фильтрацию, переименование или редактирование
  атрибутов встроенных handlers и дополнительных фабрик.
- `WithClosers(...io.Closer)` передает логеру ответственность за закрытие
  дополнительных ресурсов после успешного `New`.

```go
level := new(slog.LevelVar)
log, err := gologger.New(gologger.Config{},
    gologger.WithLevel(level),
    gologger.WithReplaceAttr(func(_ []string, attr slog.Attr) slog.Attr {
        if attr.Key == "password" {
            return slog.String(attr.Key, "[redacted]")
        }
        return attr
    }),
    gologger.WithMiddleware(func(next slog.Handler) slog.Handler {
        return next.WithAttrs([]slog.Attr{slog.String("service", "billing")})
    }),
)
```

Публичные пакеты handlers: `handlers/slogcontext`, `handlers/slogpretty`,
`handlers/slogdiscard`. Они работают и с обычным `slog.New`, без `gologger.Log`.
`Logger` сохраняет исходный интерфейс для consumer-owned adapters и mocks.

## Ресурсы

`Config.Output` добавляет буферизованный JSON-файл. Новый файл создается с
правами `0600`; права существующего файла не меняются. Это отличается от
прежнего `goshared`, который запрашивал `0666` с учетом umask.

`Close()` завершает запись и закрывает принадлежащие логеру ресурсы один раз.
`Flush()` — совместимое имя для того же завершающего действия, а не периодический
flush. Все вызовы, включая конкурентные вызовы через `WithNamed`/`WithAttrs`,
возвращают сохраненную ошибку закрытия. Сначала остановите пользователей логера.
Вызовы унаследованных `slog.Logger.With`/`WithGroup` возвращают обычный
`*slog.Logger`; ресурсы продолжает закрывать исходный `*gologger.Log`.

Переданные writer/handler по умолчанию принадлежат вызывающему коду и не
закрываются библиотекой. `WithClosers` передает ответственность только после
успешного конструктора; closers вызываются в обратном порядке и сами должны
сбрасывать свои буферы. Goroutines и фоновых timers у библиотеки нет.

## Миграция из goshared

Для сохранения старого поведения можно заменить импорт на
`logger "github.com/assurrussa/gologger"`. Сохраняются `Config`, `Logger`, `Log`,
`NewLogger`, `Default`, `Discard*`, `Error`, `WithValue` и `LogLevel`.
Прямой импорт `gologger` не устанавливает глобальный логер автоматически:
это делает явный вызов `NewLogger` или `SetDefault`.

`NewLogger` — legacy-конструктор: меняет общий `LogLevel` и process default,
сохраняет WARN fallback, игнорирует `JSON` в local/development и трактует rate
вне `(0, 1)` как отключенный sampling. Новые интеграции должны использовать
`New` с явными options. Общий `LogLevel` меняют через `.Set`, не заменой указателя.

В `goshared` прежние package paths остаются совместимыми адаптерами: контракты
являются type aliases, а context-атрибуты доступны через оба import paths.

## Проверки

```sh
make check
./scripts/test-consumer.sh <published-version-or-commit>
```

`make check` запускает vet, golangci-lint и race-тесты пять раз.
Consumer probe создает отдельный Go-модуль с `GOWORK=off`, загружает указанную
опубликованную версию без `replace` и проверяет публичные импорты.

Лицензия MIT; исходное copyright уведомление `goshared` сохранено.

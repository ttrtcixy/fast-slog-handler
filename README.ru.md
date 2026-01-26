[English](README.md) | [Русский](README.ru.md)

# Высокопроизводительный обработчик Slog

Библиотека предоставляет оптимизированные обработчики для пакета slog. Основной фокус — на скорости работы и удобстве локальной разработки.
* **Text Handler**: Создан для локальной разработки. ANSI-подсветка уровней, меток времени и метаданных. (Не рекомендуется для production).
* **JSON Handler**: Высокопроизводительный обработчик для production-сред.

## Ключевые особенности
* Использование `sync.Pool` минимизирует нагрузку на GC и выделение памяти в куче.
* Опциональная буферизация через `bufio` с фоновым сбросом (flush) данных для снижения задержек на системных вызовах.
* Простая передача TraceID или RequestID напрямую через `context.Context`.
* Полная потокобезопасность.

## Установка
```shell
go get github.com/ttrtcixy/color-slog-handler
```

## Использование

```go
package main

import (
	"context"
	"log/slog"
	"os"

	logger "github.com/ttrtcixy/color-slog-handler"
)

func main() {
	cfg := &logger.Config{
		Level:          int(slog.LevelDebug),
		BufferedOutput: true, // Включение буферезированного вывода и фоновую очистку буфера.
	}
    
	// или logger.NewTextHandler(os.Stdout, cfg) для разработки
	handler := logger.NewJsonHandler(os.Stdout, cfg)
	l := slog.New(handler)

	// Важно: Закройте обработчик, чтобы остановить очистку и сбросить оставшиеся журналы
	defer handler.Close(context.Background())

	l.LogAttrs(nil, slog.LevelInfo, "msg", slog.String("key", "val"))

	// Добавьте атрибуты в контекст
	ctx := handler.AppendAttrsToCtx(context.Background(), slog.String("trace_id", "af82-bx22"))

	// Логгер автоматически подберет их
	l.LogAttrs(ctx, slog.LevelInfo, "msg")
}
```

## Конфигурация
Структура `Config` поддерживает переменные среды через теги:
* `Level`: Уровень логирования (например, Debug=-4, Info=0).
* `BufferedOutput`: Включить/Отключить буфер 4 КБ с автоматической периодической очисткой.

## Важное примечание о буферизации
Если установлено значение `BufferedOutput`: true, необходимо вызвать `handler.Close(ctx)`:
* Он останавливает фоновую goroutine очистки.
* Он гарантирует, что все оставшиеся журналы в буфере размером 4096 байт будут записаны в выходные данные.

Вызов `Close()` для необработанного обработчика вернет `ErrNothingToClose`.

## Дорожная карта
* Поддержка `slog.LogValuer`.

# Задача 05 — Сравнение моделей

Программа отправляет один и тот же запрос трём моделям Anthropic и сравнивает время ответа, количество токенов и стоимость.

![demo](demo.gif)

## Модели

| Модель | Уровень | Цена вход / выход (за 1M токенов) |
|---|---|---|
| `claude-opus-4-6` | Сильная | $15 / $75 |
| `claude-sonnet-4-6` | Средняя | $3 / $15 |
| `claude-haiku-4-5-20251001` | Слабая | $0.80 / $4 |

## Использование

```bash
# Запрос по умолчанию (зашит в defaultTask)
./task05

# Свой запрос
./task05 -message "Как организовать кэширование в REST API?"

# Свой системный промпт
./task05 -message "..." -system "Ты опытный CTO стартапа."

# Ограничить длину ответа
./task05 -max_tokens 512
```

## Настройка

Запрос по умолчанию и список моделей задаются в `main.go`:

```go
const defaultTask = `...`   // запрос по умолчанию

var models = []Model{
    {ID: "claude-opus-4-6",            InputPricePer1M: 15.00, OutputPricePer1M: 75.00},
    {ID: "claude-sonnet-4-6",          InputPricePer1M:  3.00, OutputPricePer1M: 15.00},
    {ID: "claude-haiku-4-5-20251001",  InputPricePer1M:  0.80, OutputPricePer1M:  4.00},
}
```

## Переменные окружения

| Переменная | Описание |
|---|---|
| `ANTHROPIC_API_KEY` | API-ключ (обязательный) |
| `ANTHROPIC_GATEWAY_URL` | URL шлюза (по умолчанию: `https://api.anthropic.com`) |

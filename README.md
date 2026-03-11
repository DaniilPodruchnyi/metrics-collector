# go-musthave-metrics-tpl

Шаблон репозитория для трека «Сервер сбора метрик и алертинга».

## Начало работы

1. Склонируйте репозиторий в любую подходящую директорию на вашем компьютере.
2. В корне репозитория выполните команду `go mod init <name>` (где `<name>` — адрес вашего репозитория на GitHub без префикса `https://`) для создания модуля.

## Обновление шаблона

Чтобы иметь возможность получать обновления автотестов и других частей шаблона, выполните команду:

```
git remote add -m v2 template https://github.com/Yandex-Practicum/go-musthave-metrics-tpl.git
```

Для обновления кода автотестов выполните команду:

```
git fetch template && git checkout template/v2 .github
```

Затем добавьте полученные изменения в свой репозиторий.

## Запуск автотестов

Для успешного запуска автотестов называйте ветки `iter<number>`, где `<number>` — порядковый номер инкремента. Например, в ветке с названием `iter4` запустятся автотесты для инкрементов с первого по четвёртый.

При мёрже ветки с инкрементом в основную ветку `main` будут запускаться все автотесты.

Подробнее про локальный и автоматический запуск читайте в [README автотестов](https://github.com/Yandex-Practicum/go-autotests).

## Структура проекта

Приведённая в этом репозитории структура проекта является рекомендуемой, но не обязательной.

Это лишь пример организации кода, который поможет вам в реализации сервиса.

При необходимости можно вносить изменения в структуру проекта, использовать любые библиотеки и предпочитаемые структурные паттерны организации кода приложения, например:
- **DDD** (Domain-Driven Design)
- **Clean Architecture**
- **Hexagonal Architecture**
- **Layered Architecture**

## Профилирование памяти (pprof)

Для оценки и оптимизации потребления памяти были добавлены бенчмарки для ключевых компонентов, в том числе:

- **internal/repository**: операции `Store`, `Get`, `GetAll`, в том числе в конкурентном режиме.
- **internal/handler**: рендеринг HTML-дашборда метрик `GetAllMetricsHTML`.

### Как снять профили памяти

1. **Базовый профиль** (до оптимизаций или в качестве текущей базы):

```bash
go test ./internal/handler -run=^$ -bench=BenchmarkGetAllMetricsHTML -memprofile=profiles/base
```

2. **Повторный профиль** (после изменений/оптимизаций):

```bash
go test ./internal/handler -run=^$ -bench=BenchmarkGetAllMetricsHTML -memprofile=profiles/result
```

3. **Сравнение профилей с помощью pprof**:

```bash
go tool pprof -top -diff_base=profiles/base profiles/result
```

### Результаты оптимизации

В качестве основной точки оптимизации был выбран HTML-рендеринг дашборда метрик (`GetAllMetricsHTML`): ранее HTML-шаблон парсился при каждом запросе, что приводило к значительным аллокациям памяти. Шаблон был вынесен в глобальную переменную `metricsTemplate` и теперь парсится один раз при инициализации.

Вывод команды сравнения профилей:

```bash
File: handler.test.exe
Build ID: C:\Users\79680\AppData\Local\Temp\go-build3412524896\b001\handler.test.exe2026-03-11 13:00:34.8629052 +0300 MSK
Type: alloc_space
Time: 2026-03-11 13:00:03 MSK
Showing nodes accounting for 269.97MB, 71.84% of 375.81MB total
Dropped 49 nodes (cum <= 1.88MB)
      flat  flat%   sum%        cum   cum%
  151.02MB 40.18% 40.18%   151.02MB 40.18%  bytes.growSlice
   77.50MB 20.62% 60.81%   155.50MB 41.38%  reflect.Value.call
   40.50MB 10.78% 71.58%   222.01MB 59.07%  text/template.(*state).evalCall
      27MB  7.18% 78.77%    45.50MB 12.11%  reflect.MakeSlice
      24MB  6.39% 85.16%       24MB  6.39%  html/template.htmlReplacer
  -18.52MB  4.93% 80.23%   -18.52MB  4.93%  text/template.addFuncs (inline)
   18.50MB  4.92% 85.15%    18.50MB  4.92%  reflect.unsafe_NewArray
  -14.51MB  3.86% 81.29%   -14.51MB  3.86%  text/template.addValueFuncs
     -14MB  3.73% 77.56%      -14MB  3.73%  text/template/parse.(*Tree).newText (inline)
      13MB  3.46% 81.02%       26MB  6.92%  text/template.(*state).evalArg
  -11.51MB  3.06% 77.96%   -11.51MB  3.06%  maps.Copy[go.shape.map[string]html/template.context,go.shape.map[string]html/template.context,go.shape.string,go.shape.struct { html/template.state html/template.state; html/template.delim html/template.delim; html/template.urlPart html/template.urlPart; html/template.jsCtx html/template.jsCtx; html/template.jsBraceDepth []int; html/template.attr html/template.attr; html/template.element html/template.element; html/template.n text/template/parse.Node; html/template.err *html/template.Error }] (inline)
   11.51MB  3.06% 81.02%    16.01MB  4.26%  internal/fmtsort.Sort
      11MB  2.93% 83.95%       11MB  2.93%  text/template.(*state).validateType
  -10.01MB  2.66% 81.28%   -10.01MB  2.66%  text/template.builtins (inline)
      10MB  2.66% 83.95%       10MB  2.66%  github.com/DaniilPodruchnyi/metrics-collector/internal/repository.(*MemStorage).GetAll
   -9.01MB  2.40% 81.55%   -40.02MB 10.65%  html/template.(*escaper).escapeTemplateBody
    7.50MB  2.00% 83.55%     7.50MB  2.00%  net/http.Header.Clone (inline)
      -7MB  1.86% 81.68%       -7MB  1.86%  text/template/parse.(*ListNode).append (inline)
      -7MB  1.86% 79.82%       -7MB  1.86%  html/template.makeEscaper (inline)
   -6.50MB  1.73% 78.09%    -6.50MB  1.73%  text/template/parse.(*Tree).newPipeline (inline)
       6MB  1.60% 79.69%        6MB  1.60%  net/http.(*Request).WithContext (inline)
    5.50MB  1.46% 81.15%     5.50MB  1.46%  reflect.packEface
      -5MB  1.33% 79.82%       -5MB  1.33%  html/template.(*escaper).editActionNode
       5MB  1.33% 81.15%        5MB  1.33%  text/template.(*state).push (inline)
       5MB  1.33% 82.48%   400.03MB 106.44%  text/template.(*Template).execute
      -5MB  1.33% 81.15%       -5MB  1.33%  strings.genSplit
    4.50MB  1.20% 82.35%     4.50MB  1.20%  net/textproto.MIMEHeader.Set (inline)
    4.50MB  1.20% 83.55%     4.50MB  1.20%  net/http/httptest.NewRecorder (inline)
   -4.50MB  1.20% 82.35%       -9MB  2.39%  text/template/parse.(*Tree).newVariable (inline)
    4.50MB  1.20% 83.55%     4.50MB  1.20%  reflect.copyVal
      -4MB  1.06% 82.48%       -4MB  1.06%  text/template/parse.(*Tree).newIf (inline)
      -4MB  1.06% 81.42%    -7.50MB  2.00%  html/template.(*escaper).escapeAction
      -4MB  1.06% 80.35%       -4MB  1.06%  text/template/parse.(*Tree).newList (inline)
   -3.50MB  0.93% 79.42%    -3.50MB  0.93%  bytes.ToUpper
   -3.50MB  0.93% 78.49%    -3.50MB  0.93%  text/template/parse.NewIdentifier (inline)
      -3MB   0.8% 77.69%       -3MB   0.8%  text/template/parse.(*Tree).newChain (inline)
      -3MB   0.8% 76.89%       -3MB   0.8%  text/template/parse.(*Tree).newString (inline)
      -3MB   0.8% 76.10%       -3MB   0.8%  text/template/parse.(*Tree).newCommand (inline)
      -3MB   0.8% 75.30%       -3MB   0.8%  text/template/parse.(*CommandNode).append (inline)
       3MB   0.8% 76.10%        3MB   0.8%  fmt.Sprint
   -2.50MB  0.67% 75.43%       -7MB  1.86%  html/template.New
      -2MB  0.53% 74.90%       -2MB  0.53%  text/template.(*Template).init (inline)
      -2MB  0.53% 74.37%       -2MB  0.53%  text/template/parse.(*Tree).newAction (inline)
      -2MB  0.53% 73.83%       -4MB  1.06%  html/template.newIdentCmd (inline)
      -2MB  0.53% 73.30%   -37.50MB  9.98%  text/template/parse.(*Tree).pipeline
       2MB  0.53% 73.83%        2MB  0.53%  text/template.(*state).evalEmptyInterface
   -1.50MB   0.4% 73.43%    -1.50MB   0.4%  text/template/parse.New (inline)
   -1.50MB   0.4% 73.03%   -17.51MB  4.66%  html/template.(*escaper).escapeBranch
    1.50MB   0.4% 73.43%     1.50MB   0.4%  context.WithValue
   -1.50MB   0.4% 73.03%    -1.50MB   0.4%  text/template/parse.(*ChainNode).Add (inline)
   -1.50MB   0.4% 72.64%    -1.50MB   0.4%  text/template/parse.(*PipeNode).append (inline)
      -1MB  0.27% 72.37%   -41.02MB 10.91%  html/template.(*escaper).escapeTree
      -1MB  0.27% 72.10%       -2MB  0.53%  text/template/parse.(*ChainNode).String (inline)
   -0.50MB  0.13% 71.97%   -31.01MB  8.25%  html/template.(*escaper).escapeListConditionally
   -0.50MB  0.13% 71.84%    -2.50MB  0.67%  text/template.New (inline)
   -0.50MB  0.13% 71.70%   -37.53MB  9.99%  html/template.(*escaper).commit
    0.50MB  0.13% 71.84%   375.02MB 99.79%  text/template.(*state).walkRange
```

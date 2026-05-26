<div align="center">

# GC Medal Catcher

Auto-discovers and downloads every medal available on the GamersClub CDN.

Descobre automaticamente e baixa todas as medalhas disponíveis na CDN do GamersClub.

[![Go](https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat-square&logo=go&logoColor=white)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-blueviolet?style=flat-square)](LICENSE)

[English](#english) · [Português BR](#português-br)

</div>

---

<div align="center">

| #4 | #68 | #215 | #447 | #1302 | #1415 | #1503 | #1545 |
|:---:|:---:|:---:|:---:|:---:|:---:|:---:|:---:|
| <img src="samples/medal_4.png" width="72"> | <img src="samples/medal_68.png" width="72"> | <img src="samples/medal_215.png" width="72"> | <img src="samples/medal_447.png" width="72"> | <img src="samples/medal_1302.png" width="72"> | <img src="samples/medal_1415.png" width="72"> | <img src="samples/medal_1503.png" width="72"> | <img src="samples/medal_1545.png" width="72"> |

1 481 medals captured · IDs 0 – 1550

</div>

---

## English

### Overview

`gamersclub-medal-catcher` is a single-binary Go tool that probes the GamersClub CDN, determines the valid ID range at runtime via binary search, and downloads all available medal assets to disk. Concurrency, rate limiting, and retry logic are built in.

### Features

| Feature | Detail |
|---|---|
| Auto-discovery | Exponential probe + binary search locates the upper ID bound in ~22 HEAD requests, no hardcoded limit |
| Worker pool | 30 goroutines drain a shared buffered channel of pending IDs |
| Rate limiter | Token-bucket at 25 req/s with burst of 50 (`golang.org/x/time/rate`) |
| HTTP/2 | `ForceAttemptHTTP2` enables multiplexing over a single TLS connection |
| Exponential backoff | Up to 3 retries on network errors or HTTP 429/5xx, delays 500ms → 1s → 2s |
| Idempotent | Existing files are pre-loaded into a `map[int]struct{}` at startup; re-runs skip already-downloaded IDs with O(1) lookup |
| Zero-alloc streaming | `sync.Pool` of 64 KB buffers passed to `io.CopyBuffer`; no per-request heap allocation |

### Usage

```bash
git clone https://github.com/your-user/gamersclub-medal-catcher
cd gamersclub-medal-catcher
go run .
```

Build a standalone binary:

```bash
go build -o medal-catcher .
./medal-catcher       # Linux / macOS
.\medal-catcher.exe   # Windows
```

Output is written to `./medalhas/medal_<id>.png`.

### Configuration

Constants at the top of `main.go`:

| Constant | Default | Description |
|---|:---:|---|
| `startID` | `0` | First ID to probe |
| `outputDir` | `"medalhas"` | Output directory |
| `maxWorkers` | `30` | Concurrent download goroutines |
| `requestsPerSec` | `25` | Token-bucket fill rate |
| `burstSize` | `50` | Token-bucket burst capacity |
| `maxRetries` | `3` | Per-ID retry limit |
| `requestTimeout` | `15s` | Per-request context timeout |
| `retryBaseDelay` | `500ms` | Base delay for exponential backoff (`delay × 2ⁿ⁻¹`) |

### Architecture

```
handle()
  ├── http.Client          HTTP/2, pooled transport, explicit dial/TLS/header timeouts
  ├── discoverUpperBound() exponential probe (100, 200, 400 …) → binary search → upper bound
  ├── loadExisting()       scans outputDir → map[int]struct{} for O(1) skip decisions
  ├── rate.Limiter         token bucket, 25 req/s + burst 50
  ├── ids chan int          buffered channel containing only IDs not yet on disk
  │
  └── 30 × goroutine
        downloadMedal(ctx, client, limiter, id)
          limiter.Wait(ctx)        rate gate
          httpGet(ctx, …)          GET with 15s context; defer cancel() kept alive through body read
            200  → io.CopyBuffer to disk via 64KB pool buffer
            404  → counted as skipped, no retry
            429 / 5xx → exponential backoff, retry up to maxRetries
            error     → exponential backoff, retry up to maxRetries
```

### Dependencies

| Package | Purpose |
|---|---|
| [golang.org/x/time/rate](https://pkg.go.dev/golang.org/x/time/rate) | Token-bucket rate limiter |

### License

[MIT](LICENSE)

---

## Português BR

### Visão Geral

`gamersclub-medal-catcher` é uma ferramenta Go de binário único que sonda a CDN do GamersClub, determina o intervalo válido de IDs em tempo de execução via busca binária e baixa todos os assets de medalhas disponíveis para o disco. Concorrência, rate limiting e lógica de retry são integrados.

### Funcionalidades

| Funcionalidade | Detalhe |
|---|---|
| Auto-descoberta | Sonda exponencial + busca binária localiza o limite superior de IDs em ~22 requisições HEAD, sem limite fixo no código |
| Pool de workers | 30 goroutines consomem um canal bufferizado de IDs pendentes |
| Rate limiter | Token-bucket a 25 req/s com burst de 50 (`golang.org/x/time/rate`) |
| HTTP/2 | `ForceAttemptHTTP2` habilita multiplexação sobre uma única conexão TLS |
| Backoff exponencial | Até 3 tentativas em erros de rede ou HTTP 429/5xx, delays 500ms → 1s → 2s |
| Idempotente | Arquivos existentes são pré-carregados em `map[int]struct{}` na inicialização; re-execuções pulam IDs já baixados com lookup O(1) |
| Streaming sem alocação | `sync.Pool` de buffers de 64 KB passados para `io.CopyBuffer`; sem alocação de heap por requisição |

### Uso

```bash
git clone https://github.com/your-user/gamersclub-medal-catcher
cd gamersclub-medal-catcher
go run .
```

Compilar um binário independente:

```bash
go build -o medal-catcher .
./medal-catcher       # Linux / macOS
.\medal-catcher.exe   # Windows
```

A saída é escrita em `./medalhas/medal_<id>.png`.

### Configuração

Constantes no topo de `main.go`:

| Constante | Padrão | Descrição |
|---|:---:|---|
| `startID` | `0` | Primeiro ID a sondar |
| `outputDir` | `"medalhas"` | Diretório de saída |
| `maxWorkers` | `30` | Goroutines de download concorrentes |
| `requestsPerSec` | `25` | Taxa de preenchimento do token-bucket |
| `burstSize` | `50` | Capacidade de burst do token-bucket |
| `maxRetries` | `3` | Limite de retentativas por ID |
| `requestTimeout` | `15s` | Timeout de contexto por requisição |
| `retryBaseDelay` | `500ms` | Delay base para backoff exponencial (`delay × 2ⁿ⁻¹`) |

### Arquitetura

```
handle()
  ├── http.Client          HTTP/2, transport com pool, timeouts explícitos de dial/TLS/header
  ├── discoverUpperBound() sonda exponencial (100, 200, 400 …) → busca binária → limite superior
  ├── loadExisting()       lê outputDir → map[int]struct{} para decisões de skip O(1)
  ├── rate.Limiter         token bucket, 25 req/s + burst 50
  ├── ids chan int          canal bufferizado contendo apenas IDs ainda não no disco
  │
  └── 30 × goroutine
        downloadMedal(ctx, client, limiter, id)
          limiter.Wait(ctx)        controle de taxa
          httpGet(ctx, …)          GET com contexto de 15s; defer cancel() mantido ativo durante leitura do body
            200  → io.CopyBuffer para disco via buffer de pool de 64KB
            404  → contado como pulado, sem retry
            429 / 5xx → backoff exponencial, retry até maxRetries
            erro      → backoff exponencial, retry até maxRetries
```

### Dependências

| Pacote | Finalidade |
|---|---|
| [golang.org/x/time/rate](https://pkg.go.dev/golang.org/x/time/rate) | Rate limiter token-bucket |

### Licença

[MIT](LICENSE)

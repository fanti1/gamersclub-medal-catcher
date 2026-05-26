<div align="center">

# 🏅 GC Medal Catcher

**Auto-discovers and downloads every medal on the GamersClub CDN — concurrently, safely, and without hammering the servers.**

**Descobre automaticamente e baixa todas as medalhas da CDN do GamersClub — de forma concorrente, segura e sem sobrecarregar os servidores.**

[![Go](https://img.shields.io/badge/Go-1.23+-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-blueviolet?style=for-the-badge)](LICENSE)
[![CDN-safe](https://img.shields.io/badge/CDN-rate--limited-success?style=for-the-badge)](#%EF%B8%8F-configuration--configuração)

[🇺🇸 English](#-english) · [🇧🇷 Português](#-português-br)

</div>

---

## 🖼️ Medal Gallery · Galeria de Medalhas

<div align="center">

| #4 | #68 | #215 | #447 |
|:---:|:---:|:---:|:---:|
| <img src="medalhas/medal_4.png" width="80" alt="Medal 4"> | <img src="medalhas/medal_68.png" width="80" alt="Medal 68"> | <img src="medalhas/medal_215.png" width="80" alt="Medal 215"> | <img src="medalhas/medal_447.png" width="80" alt="Medal 447"> |

| #1302 | #1415 | #1503 | #1545 |
|:---:|:---:|:---:|:---:|
| <img src="medalhas/medal_1302.png" width="80" alt="Medal 1302"> | <img src="medalhas/medal_1415.png" width="80" alt="Medal 1415"> | <img src="medalhas/medal_1503.png" width="80" alt="Medal 1503"> | <img src="medalhas/medal_1545.png" width="80" alt="Medal 1545"> |

> **1 481 medals** captured from the CDN across IDs 0 – 1550 · **1 481 medalhas** capturadas da CDN no intervalo de IDs 0 – 1550

</div>

---

## 🇺🇸 English

### ✨ Features

| | |
|---|---|
| 🔍 **Auto-discovery** | Binary-search probes the CDN to find the upper ID bound — no hardcoded limits |
| ⚡ **Worker pool** | 30 concurrent goroutines drain a shared ID channel |
| 🚦 **Rate limiter** | Token-bucket (25 req/s, burst 50) — never overwhelms the CDN |
| 🚀 **HTTP/2** | Multiplexing over a single TLS connection reduces handshake overhead |
| 🔄 **Exponential backoff** | Up to 3 retries on network errors or 5xx / 429 responses |
| 📁 **Idempotent** | Pre-loads existing files at startup; re-runs are near-instant |
| 🧠 **Zero-alloc I/O** | `sync.Pool` of 64 KB buffers — no heap pressure during streaming |
| 📊 **Final summary** | Prints downloaded / skipped / failed counts + elapsed time |

---

### 🚀 Quick Start

```bash
# Clone
git clone https://github.com/your-user/gamersclub-medal-catcher
cd gamersclub-medal-catcher

# Run directly (medals saved to ./medalhas/)
go run .
```

Or build a standalone binary:

```bash
go build -o medal-catcher .
./medal-catcher          # Linux / macOS
.\medal-catcher.exe      # Windows
```

**That's it.** No flags, no config files. The tool auto-discovers the medal range and saves whatever the CDN returns as `200 OK`.

---

### ⚙️ Configuration

All knobs live as typed constants at the top of `main.go`:

| Constant | Default | Description |
|---|:---:|---|
| `startID` | `0` | First medal ID to probe |
| `outputDir` | `"medalhas"` | Directory where PNGs are written |
| `maxWorkers` | `30` | Number of concurrent download goroutines |
| `requestsPerSec` | `25` | Token-bucket fill rate (CDN-safe limit) |
| `burstSize` | `50` | Initial burst before rate limiting kicks in |
| `maxRetries` | `3` | Retry attempts per medal before giving up |
| `requestTimeout` | `15s` | Per-request context deadline |
| `retryBaseDelay` | `500ms` | Base delay for exponential backoff (`× 2ⁿ⁻¹`) |

---

### 🏗️ How It Works

```
main()
 └── handle()
       ├── os.MkdirAll("medalhas/")
       ├── http.Client  (HTTP/2, pooled transport, explicit timeouts)
       ├── discoverUpperBound()   → exponential probe + binary search (~22 HEAD reqs)
       ├── loadExisting()         → pre-built map[int]struct{} from disk (O(1) lookup)
       ├── rate.Limiter           → token bucket, 25 req/s + burst 50
       ├── ids chan int            → only IDs not yet on disk
       │
       └── 30 goroutines (worker pool)
             └── downloadMedal(ctx, client, limiter, id)
                   ├── limiter.Wait(ctx)           → rate gate
                   ├── GET with 15s timeout        → body streamed via 64KB pool buffer
                   ├── 200 → io.CopyBuffer to disk
                   ├── 404 → skip (not an error)
                   ├── 429 / 5xx → exponential backoff, retry
                   └── network error → exponential backoff, retry
```

**Discovery algorithm:**
1. Exponential probe: check IDs `100, 200, 400, 800…` until the CDN returns 404
2. Binary-search the window `[last-ok, first-404]` to pin the exact boundary
3. Result: ~22 HEAD requests regardless of how many medals exist

---

### 📦 Tech Stack

- **[Go 1.23](https://golang.org)** — single binary, zero runtime deps
- **[golang.org/x/time/rate](https://pkg.go.dev/golang.org/x/time/rate)** — production-grade token-bucket limiter

---

### 📜 License

[MIT](LICENSE) — do whatever you want, just don't blame us if GamersClub changes their CDN.

---

## 🇧🇷 Português BR

### ✨ Funcionalidades

| | |
|---|---|
| 🔍 **Auto-descoberta** | Busca binária sonda a CDN para encontrar o limite superior de IDs — sem limites fixos no código |
| ⚡ **Pool de workers** | 30 goroutines concorrentes consumindo um canal compartilhado de IDs |
| 🚦 **Rate limiter** | Token-bucket (25 req/s, burst 50) — nunca sobrecarrega a CDN |
| 🚀 **HTTP/2** | Multiplexação sobre uma única conexão TLS reduz overhead de handshake |
| 🔄 **Backoff exponencial** | Até 3 tentativas em erros de rede ou respostas 5xx / 429 |
| 📁 **Idempotente** | Pré-carrega arquivos existentes na inicialização; re-execuções são quase instantâneas |
| 🧠 **I/O sem alocação** | `sync.Pool` de buffers de 64 KB — sem pressão de heap durante streaming |
| 📊 **Resumo final** | Exibe contagens de baixados / pulados / falhados + tempo total |

---

### 🚀 Como Usar

```bash
# Clonar
git clone https://github.com/your-user/gamersclub-medal-catcher
cd gamersclub-medal-catcher

# Executar diretamente (medalhas salvas em ./medalhas/)
go run .
```

Ou compilar um binário independente:

```bash
go build -o medal-catcher .
./medal-catcher          # Linux / macOS
.\medal-catcher.exe      # Windows
```

**Só isso.** Sem flags, sem arquivos de configuração. A ferramenta auto-descobre o intervalo de medalhas e salva o que a CDN retornar como `200 OK`.

---

### ⚙️ Configuração

Todos os parâmetros ficam como constantes tipadas no topo do `main.go`:

| Constante | Padrão | Descrição |
|---|:---:|---|
| `startID` | `0` | Primeiro ID de medalha a sondar |
| `outputDir` | `"medalhas"` | Diretório onde os PNGs são salvos |
| `maxWorkers` | `30` | Número de goroutines de download concorrentes |
| `requestsPerSec` | `25` | Taxa de preenchimento do token-bucket (limite seguro para CDN) |
| `burstSize` | `50` | Burst inicial antes de o rate limiting entrar em ação |
| `maxRetries` | `3` | Tentativas por medalha antes de desistir |
| `requestTimeout` | `15s` | Timeout de contexto por requisição |
| `retryBaseDelay` | `500ms` | Delay base para backoff exponencial (`× 2ⁿ⁻¹`) |

---

### 🏗️ Como Funciona

```
main()
 └── handle()
       ├── os.MkdirAll("medalhas/")
       ├── http.Client  (HTTP/2, transport com pool, timeouts explícitos)
       ├── discoverUpperBound()   → sonda exponencial + busca binária (~22 HEAD reqs)
       ├── loadExisting()         → map[int]struct{} pré-construído do disco (lookup O(1))
       ├── rate.Limiter           → token bucket, 25 req/s + burst 50
       ├── ids chan int            → apenas IDs ainda não no disco
       │
       └── 30 goroutines (pool de workers)
             └── downloadMedal(ctx, client, limiter, id)
                   ├── limiter.Wait(ctx)           → controle de taxa
                   ├── GET com timeout de 15s      → corpo via buffer de pool 64KB
                   ├── 200 → io.CopyBuffer para disco
                   ├── 404 → pular (não é erro)
                   ├── 429 / 5xx → backoff exponencial, retry
                   └── erro de rede → backoff exponencial, retry
```

**Algoritmo de descoberta:**
1. Sonda exponencial: verifica IDs `100, 200, 400, 800…` até a CDN retornar 404
2. Busca binária na janela `[último-ok, primeiro-404]` para fixar o limite exato
3. Resultado: ~22 requisições HEAD independentemente de quantas medalhas existam

---

### 📦 Stack Tecnológica

- **[Go 1.23](https://golang.org)** — binário único, zero dependências de runtime
- **[golang.org/x/time/rate](https://pkg.go.dev/golang.org/x/time/rate)** — rate limiter de produção com token-bucket

---

### 📜 Licença

[MIT](LICENSE) — faça o que quiser, apenas não nos culpe se o GamersClub mudar a CDN.

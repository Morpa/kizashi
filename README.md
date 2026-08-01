# kizashi

Visualizador de logs no terminal: cores por nível, follow ao vivo e filtros.
Funciona com **qualquer framework** — Node, React/Vite, Astro, Go... se o teu
app escreve no terminal, o kizashi mostra.

## 🚀 Instalar

**Homebrew** (via release oficial):

```bash
brew install morpa/tap/kizashi
```

Build local:

```bash
make            # gera o binário bin/kizashi
make install    # instala no PATH (aí é só digitar `kizashi`)
```

Ou manualmente:

```bash
go build -o bin/kizashi ./cmd/kizashi
```

## ▶️ Usar (o principal)

Pega o comando que roda o teu projeto e cola na frente do kizashi:

```bash
pnpm dev 2>&1 | kizashi stream        # Astro / pnpm
npm run dev 2>&1 | kizashi stream     # Vite / React / Next
node server.js 2>&1 | kizashi stream  # Node
go run ./cmd/app 2>&1 | kizashi stream
```

O `2>&1` é importante: junta os erros no stdout, senão você perde os logs de
erro. Pronto — os logs aparecem seguindo ao vivo, coloridos por nível.

Também dá para acompanhar um arquivo de log (começa do fim, como `tail -f`):

```bash
kizashi stream -f app.log
kizashi stream -f app.log -f api.log        # vários arquivos
kizashi stream -f app.log --buffer 20000    # buffer maior
```

## ⌨️ Teclas

| Tecla                 | Ação                             |
| --------------------- | -------------------------------- |
| `q` / `Ctrl-C`        | sair                             |
| `/`                   | abrir o filtro                   |
| `Enter`               | aplicar o filtro                 |
| `Esc`                 | limpar o filtro                  |
| `Espaço` / `f`        | pausar / retomar o follow        |
| `↑` `↓` `PgUp` `PgDn` | rolar (↑ sai do follow)          |
| `g` / `G`             | início / fim (G retoma o follow) |

## 🔎 Filtros

Pressione `/` e digite. Termos separados por espaço = AND.

| Você digita     | Resultado                    |
| --------------- | ---------------------------- |
| `error`         | mensagens que contêm "error" |
| `level=error`   | só nível error               |
| `level>=warn`   | warn, error e fatal          |
| `service=api`   | campo `service` contém "api" |
| `request.id=42` | campo aninhado               |
| `error slow`    | os dois ao mesmo tempo       |

## 🧩 O que o kizashi entende

- **Linha JSON** (de uma linha só) → vira `hora serviço NÍVEL mensagem`, cor
  por nível. Funciona com logs estruturados de qualquer ferramenta (pino,
  Astro, ...), incluindo níveis numéricos (pino: 30 = info, 50 = error).
- **Texto puro** → passa como está, mantendo as cores ANSI do teu app
  (ex.: Vite). O nível é adivinhado por palavra (`error`, `warn`, ...).
- **Memória limitada**: guarda os últimos 10.000 logs (aumente com
  `--buffer 50000`).

## ⚠️ Limitações

- JSON "pretty-printed" (multilinha) é tratado como texto puro.
- `-f` começa do fim do arquivo; para ver o histórico todo, use
  `cat app.log | kizashi stream`.
- O destaque do filtro vale só para mensagens JSON.

## ✅ Testes

```bash
make check   # vet + testes + testes com race detector
```

## 📁 Estrutura

```
cmd/kizashi/          entrypoint
internal/cli/        cobra (stream, version)
internal/model/      Entry, Level
internal/logparse/   parsing de JSON + heurísticas
internal/buffer/     ring buffer thread-safe
internal/filter/     expressões de filtro
internal/ingest/     stdin + tail de arquivo
internal/tui/        LogView, painter, app
```

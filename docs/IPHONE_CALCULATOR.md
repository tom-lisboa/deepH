# Calculadora estilo iPhone com deepH

Guia reproduzivel para criar a mesma calculadora estilo iPhone que foi gerada com `deepH`, incluindo:

- workspace
- provider DeepSeek
- kit fullstack
- multiverso front-only
- ajuste de timeout
- prompts dos agents
- crew YAML
- recovery quando um universo falha

Os links abaixo sao clicaveis no GitHub. A abertura em nova aba depende do navegador/GitHub UI; use `Cmd`/`Ctrl` + clique se quiser abrir em outra aba.

## Navegacao rapida

- [Passo 1: instalar o deeph](#passo-1-instalar-o-deeph)
- [Passo 2: criar workspace e configurar a API key](#passo-2-criar-workspace-e-configurar-a-api-key)
- [Passo 3: bootstrap do workspace e aumento de timeout](#passo-3-bootstrap-do-workspace-e-aumento-de-timeout)
- [Passo 4: gerar a base fullstack da calculadora](#passo-4-gerar-a-base-fullstack-da-calculadora)
- [Passo 5: criar os agents de frontend iPhone](#passo-5-criar-os-agents-de-frontend-iphone)
- [Passo 6: colar os prompts completos dos agents](#passo-6-colar-os-prompts-completos-dos-agents)
- [Passo 7: criar o crew de multiverso front-only](#passo-7-criar-o-crew-de-multiverso-front-only)
- [Passo 8: rodar o multiverso](#passo-8-rodar-o-multiverso)
- [Passo 9: recovery se algum universo falhar](#passo-9-recovery-se-algum-universo-falhar)
- [Passo 10: pedir para o deeph configurar Tailwind e a paleta iPhone](#passo-10-pedir-para-o-deeph-configurar-tailwind-e-a-paleta-iphone)
- [Passo 11: subir backend e frontend](#passo-11-subir-backend-e-frontend)
- [Passo 12: conferir o que foi gerado](#passo-12-conferir-o-que-foi-gerado)

## O que esse guia entrega

- backend Node/Express para operacoes matematicas
- frontend Next.js App Router
- calculadora estilo iPhone
- multiverso com:
  - variante Tailwind
  - variante shadcn-like
  - synth final

## Passo 1: instalar o deeph

macOS / Linux:

```bash
curl -fsSL https://raw.githubusercontent.com/tom-lisboa/deepH/main/scripts/install.sh | bash
deeph
```

Windows PowerShell:

```powershell
iex ((iwr https://raw.githubusercontent.com/tom-lisboa/deepH/main/scripts/install.ps1 -UseBasicParsing).Content)
deeph
```

## Passo 2: criar workspace e configurar a API key

```bash
mkdir -p ~/deeph-workspace
cd ~/deeph-workspace
export DEEPSEEK_API_KEY="sk-SUA_CHAVE_REAL"
echo $DEEPSEEK_API_KEY
```

Windows CMD:

```bat
mkdir %USERPROFILE%\deeph-workspace
cd %USERPROFILE%\deeph-workspace
set DEEPSEEK_API_KEY=sk-SUA_CHAVE_REAL
echo %DEEPSEEK_API_KEY%
```

## Passo 3: bootstrap do workspace e aumento de timeout

`30000ms` ficou curto no fluxo real. O valor que estabilizou melhor foi `120000ms`.

```bash
deeph quickstart --deepseek
deeph provider add --name deepseek --model deepseek-chat --timeout-ms 120000 --set-default --force deepseek
deeph validate
```

## Passo 4: gerar a base fullstack da calculadora

Instale o kit fullstack:

```bash
deeph kit add crud-next-multiverse --provider-name deepseek --model deepseek-chat --set-default-provider
deeph validate
deeph crew list
deeph crew show crud_fullstack_multiverse
```

Gere a base backend/frontend:

```bash
deeph run --multiverse 0 @crud_fullstack_multiverse "Crie uma calculadora fullstack com frontend Next.js, rotas API, controller/service, operacoes soma/subtracao/multiplicacao/divisao e testes basicos"
```

Observacao:

- no fluxo real, esse passo criou a base backend com sucesso
- o frontend inicial saiu incompleto e foi refinado depois com um multiverso dedicado de frontend

## Passo 5: criar os agents de frontend iPhone

Garanta as skills:

```bash
deeph skill add --force file_read_range
deeph skill add --force file_write_safe
deeph skill add --force echo
```

Crie os agents:

```bash
deeph agent create --force --provider deepseek --model deepseek-chat calc_ui_tailwind
deeph agent create --force --provider deepseek --model deepseek-chat calc_ui_shadcn
deeph agent create --force --provider deepseek --model deepseek-chat calc_ui_synth
```

## Passo 6: colar os prompts completos dos agents

Cole os arquivos abaixo exatamente assim.

### `agents/calc_ui_tailwind.yaml`

```bash
cat > agents/calc_ui_tailwind.yaml <<'EOF'
name: calc_ui_tailwind
description: Gera variante Tailwind da calculadora estilo iPhone
provider: deepseek
model: deepseek-chat
system_prompt: |
  Voce e um agente de frontend focado em Next.js App Router.

  Objetivo:
  - Criar SOMENTE o arquivo `frontend/app/page.tw.tsx`.
  - Tela de calculadora estilo iPhone (dark, display grande, botoes circulares, operador laranja).

  Regras obrigatorias:
  - Nao editar nenhum outro arquivo.
  - Usar `"use client"`.
  - Nao adicionar dependencias novas.
  - Usar apenas React + classes Tailwind.
  - Fazer integracao com API backend:
    - base: `const API_BASE = process.env.NEXT_PUBLIC_API_BASE ?? "http://localhost:3000"`
    - endpoints:
      - `POST ${API_BASE}/api/calculator/add`
      - `POST ${API_BASE}/api/calculator/subtract`
      - `POST ${API_BASE}/api/calculator/multiply`
      - `POST ${API_BASE}/api/calculator/divide`
    - payload: `{ a, b }`
  - Implementar teclado com botoes 0-9, ".", "+", "-", "x", "÷", "=", "AC", "+/-", "%".
  - Em "=", chamar a API com a operacao correspondente.
  - Mostrar loading e erro de forma visivel no display.
  - Layout responsivo mobile-first.

  Saida final:
  - Responder apenas: `FRONTEND_DONE: frontend/app/page.tw.tsx`

skills:
  - file_read_range
  - file_write_safe
  - echo

io:
  outputs:
    - name: page
      produces: [frontend/page, summary/code]

metadata:
  max_tool_rounds: "8"
  tool_max_calls: "8"
  context_moment: "frontend_codegen"
EOF
```

### `agents/calc_ui_shadcn.yaml`

```bash
cat > agents/calc_ui_shadcn.yaml <<'EOF'
name: calc_ui_shadcn
description: Gera variante shadcn-like da calculadora estilo iPhone
provider: deepseek
model: deepseek-chat
system_prompt: |
  Voce e um agente de frontend focado em Next.js App Router com estilo shadcn-like.

  Objetivo:
  - Criar SOMENTE o arquivo `frontend/app/page.sc.tsx`.
  - UI estilo iPhone, mas com organizacao de componentes limpa (Card/Button-like).

  Regras obrigatorias:
  - Nao editar nenhum outro arquivo.
  - Usar `"use client"`.
  - Nao adicionar dependencias novas.
  - Se componentes shadcn reais nao existirem, usar componentes locais no proprio arquivo (ex.: `CalcButton`, `CalcDisplay`) com Tailwind.
  - Integracao API igual:
    - `const API_BASE = process.env.NEXT_PUBLIC_API_BASE ?? "http://localhost:3000"`
    - POST em add/subtract/multiply/divide com `{ a, b }`.
  - Teclado completo: 0-9, ".", "+", "-", "x", "÷", "=", "AC", "+/-", "%".
  - Em "=", chamar API.
  - Mostrar estados: loading, erro, resultado.

  Saida final:
  - Responder apenas: `FRONTEND_DONE: frontend/app/page.sc.tsx`

skills:
  - file_read_range
  - file_write_safe
  - echo

io:
  outputs:
    - name: page
      produces: [frontend/page, summary/code]

metadata:
  max_tool_rounds: "8"
  tool_max_calls: "14"
  context_moment: "frontend_codegen"
EOF
```

### `agents/calc_ui_synth.yaml`

```bash
cat > agents/calc_ui_synth.yaml <<'EOF'
name: calc_ui_synth
description: Faz sintese final entre variantes Tailwind e shadcn-like
provider: deepseek
model: deepseek-chat
system_prompt: |
  Voce e o agente de sintese de frontend.

  Objetivo:
  - Ler `frontend/app/page.tw.tsx` e `frontend/app/page.sc.tsx`.
  - Produzir versao final em `frontend/app/page.tsx`.
  - Se necessario, ajustar `frontend/app/globals.css` de forma minima e segura.

  Regras obrigatorias:
  - Manter estilo iPhone (dark, display alto, botoes circulares, operador laranja).
  - Garantir integracao com API backend:
    - `const API_BASE = process.env.NEXT_PUBLIC_API_BASE ?? "http://localhost:3000"`
    - POST add/subtract/multiply/divide com `{ a, b }`.
  - Garantir acessibilidade basica:
    - botoes com `aria-label`
    - foco visivel
  - Nao criar arquivos extras.
  - Editar apenas:
    - `frontend/app/page.tsx`
    - `frontend/app/globals.css` (opcional)

  Criterio de escolha:
  - Combine o melhor das duas variantes, priorizando clareza de codigo e UX.

  Saida final:
  - Responder apenas: `FRONTEND_DONE: frontend/app/page.tsx` (e `frontend/app/globals.css` se alterado)

skills:
  - file_read_range
  - file_write_safe
  - echo

io:
  inputs:
    - name: context
      accepts: [frontend/page, summary/code, message/agent]
      merge_policy: append3
      max_tokens: 260
  outputs:
    - name: page
      produces: [frontend/page, summary/code]

metadata:
  max_tool_rounds: "8"
  tool_max_calls: "16"
  context_moment: "synthesis"
EOF
```

Valide:

```bash
deeph validate
```

## Passo 7: criar o crew de multiverso front-only

```bash
mkdir -p crews
cat > crews/calc_iphone_front.yaml <<'EOF'
name: calc_iphone_front
description: Front-only multiverse for iPhone-like calculator (Tailwind + shadcn -> synth)
spec: calc_ui_tailwind
universes:
  - name: u_tw
    spec: calc_ui_tailwind
    output_port: page
    output_kind: frontend/page
    handoff_max_chars: 260

  - name: u_sc
    spec: calc_ui_shadcn
    output_port: page
    output_kind: frontend/page
    handoff_max_chars: 260

  - name: u_synth
    spec: calc_ui_synth
    depends_on: [u_tw, u_sc]
    input_port: context
    output_port: page
    output_kind: frontend/page
    merge_policy: append
    handoff_max_chars: 300
EOF
```

Valide e confira:

```bash
deeph validate
deeph crew list
deeph crew show calc_iphone_front
```

## Passo 8: rodar o multiverso

```bash
deeph run --coach=false --multiverse 0 @calc_iphone_front 'Crie a calculadora estilo iPhone front-only'
```

Resultado esperado:

- `frontend/app/page.tw.tsx`
- `frontend/app/page.sc.tsx`
- `frontend/app/page.tsx`

## Passo 9: recovery se algum universo falhar

No fluxo real, alguns universos bateram em:

- `tool loop exceeded max rounds`
- `context deadline exceeded`

Como recuperar sem refazer tudo:

```bash
deeph run --coach=false calc_ui_shadcn 'Crie SOMENTE frontend/app/page.sc.tsx. Faca no maximo 2 leituras e 1 escrita. Finalize com FRONTEND_DONE.'
deeph run --coach=false calc_ui_synth 'Leia frontend/app/page.tw.tsx e frontend/app/page.sc.tsx e escreva SOMENTE frontend/app/page.tsx final estilo iPhone. Finalize com FRONTEND_DONE.'
```

Se quiser usar a variante Tailwind como fallback imediato:

```bash
cp frontend/app/page.tw.tsx frontend/app/page.tsx
```

## Passo 10: pedir para o deeph configurar Tailwind e a paleta iPhone

Se a UI subir sem estilos, faltou configurar Tailwind/PostCSS no projeto frontend. O fluxo abaixo pede para o proprio `deeph` criar esses arquivos e ajustar a paleta.

### 10.1 Configurar Tailwind e PostCSS

```bash
deeph run --coach=false calc_ui_synth 'Use file_write_safe e crie/atualize SOMENTE: frontend/tailwind.config.js e frontend/postcss.config.js. Configure Tailwind para escanear ./app, ./pages, ./components. PostCSS com tailwindcss + autoprefixer. Responda apenas: CONFIG_DONE.'
```

### 10.2 Aplicar a paleta iPhone e ajustar a calculadora final

```bash
deeph run --coach=false calc_ui_synth 'Use file_write_safe e crie/atualize SOMENTE: frontend/app/globals.css e frontend/app/page.tsx. Objetivo: calculadora estilo iPhone. Paleta obrigatoria (CSS vars): --bg:#000000; --surface:#1c1c1e; --num:#333333; --fn:#a5a5a5; --op:#ff9f0a; --text:#ffffff; --muted:#8e8e93. Em page.tsx usar "use client", layout mobile-first, display grande, botoes circulares, operadores laranja, AC,+/-,%,0-9,.,=, integracao API em http://localhost:3000/api/calculator/{add|subtract|multiply|divide}. Mostrar loading/erro/resultado. Responda apenas: UI_DONE.'
```

## Passo 11: subir backend e frontend

Backend:

```bash
cd ~/deeph-workspace
npm install
npm i uuid
npm run dev
```

Frontend:

```bash
cd ~/deeph-workspace/frontend
npm install
PORT=3001 npm run dev
```

Abra:

- backend: `http://localhost:3000/health`
- frontend: `http://localhost:3001`

## Passo 12: conferir o que foi gerado

Arquivos backend esperados:

```bash
find . -maxdepth 3 -type f | sort | rg 'index.js|routes/calculator.js|controllers/calculatorController.js|services/calculatorService.js'
```

Arquivos frontend esperados:

```bash
find frontend/app -maxdepth 1 -type f | sort
```

Saida esperada no frontend:

- visual dark
- botoes circulares
- operadores laranja
- display grande no topo
- chamadas reais para a API local

## Observacoes finais

- Esse fluxo provou que o `deepH` consegue construir a calculadora com supervisao humana na orquestracao.
- O codigo final foi majoritariamente escrito pelos agents do `deepH`.
- O processo ainda exige operador humano para ajustar timeout, budgets e recovery quando um universo falha.

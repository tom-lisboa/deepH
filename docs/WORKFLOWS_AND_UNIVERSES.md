# Workflows, Universos e Comunicacao no deepH

Guia pratico para entender:

- o que e um workflow no `deepH`
- quando um workflow e so um agent spec
- quando um workflow vira um `crew`
- o que e um universo
- como os universos se comunicam
- quando eles nao se comunicam

Os links abaixo sao clicaveis no GitHub.

## Navegacao rapida

- [O que e um workflow](#o-que-e-um-workflow)
- [Workflow simples vs workflow nomeado](#workflow-simples-vs-workflow-nomeado)
- [O que e um universo](#o-que-e-um-universo)
- [Exemplo de crew com universos](#exemplo-de-crew-com-universos)
- [Como os universos se comunicam](#como-os-universos-se-comunicam)
- [Campos que controlam a comunicacao](#campos-que-controlam-a-comunicacao)
- [Quando os universos nao se comunicam](#quando-os-universos-nao-se-comunicam)
- [Como inspecionar isso na CLI](#como-inspecionar-isso-na-cli)
- [Quando usar workflow simples, crew ou multiverse](#quando-usar-workflow-simples-crew-ou-multiverse)
- [Erros comuns](#erros-comuns)

## O que e um workflow

No `deepH`, workflow e a forma como voce organiza um trabalho em etapas.

Na pratica, um workflow pode ser:

- um agent simples, como `guide`
- um agent spec com varios agents, como `planner+reader>coder>reviewer`
- um `crew`, que e um workflow salvo em arquivo

Exemplos:

```bash
deeph run guide "hello"
deeph run "planner>coder" "implemente X"
deeph run @reviewpack "analise este codigo"
```

## Workflow simples vs workflow nomeado

### Workflow simples

E quando voce escreve o spec diretamente no comando:

```bash
deeph run "planner+reader>coder>reviewer" "tarefa"
```

Isso e bom para:

- testar rapido
- iterar sem criar arquivo extra
- explorar uma ideia

### Workflow nomeado

E quando voce salva um workflow em `crews/*.yaml`.

Exemplo:

```yaml
name: reviewpack
description: Workflow salvo de review
spec: planner+reader>reviewer
```

Depois voce roda com:

```bash
deeph run @reviewpack "tarefa"
deeph trace @reviewpack "tarefa"
```

Isso e bom para:

- reaproveitar o mesmo fluxo
- dar um nome para um workflow
- compartilhar com outras pessoas
- configurar universos

## O que e um universo

Universo e uma variante de execucao dentro de um `crew`.

Pense assim:

- o workflow e o plano geral
- cada universo e um ramo desse plano

Um universo pode:

- usar um spec proprio
- receber `input_prefix` ou `input_suffix`
- depender de outros universos
- publicar uma saida para outros universos

Universos aparecem dentro de `crews/*.yaml`, em `universes:`.

Exemplo mental:

- universo 1: faz uma versao baseline
- universo 2: faz uma versao strict
- universo 3: sintetiza as duas

## Exemplo de crew com universos

```yaml
name: reviewpack
description: baseline + strict -> synth
spec: guide
universes:
  - name: baseline
    spec: guide
    output_port: result
    output_kind: summary/text

  - name: strict
    spec: guide
    output_port: result
    output_kind: diagnostic/test
    input_prefix: |
      [universe_hint]
      mode: strict
      be explicit about assumptions and tradeoffs.

  - name: synth
    spec: guide
    depends_on: [baseline, strict]
    input_port: context
    output_port: result
    output_kind: plan/summary
    merge_policy: append
    handoff_max_chars: 220
    input_prefix: |
      [universe_hint]
      Compare upstream universes and synthesize the best answer.
```

Leitura:

- `baseline` roda sozinho
- `strict` roda sozinho
- `synth` espera `baseline` e `strict`
- depois `synth` recebe as saidas resumidas dos dois

## Como os universos se comunicam

Os universos se comunicam por handoffs de multiverse.

Na pratica, essa comunicacao so acontece quando existe:

- `depends_on`

Exemplo:

```yaml
- name: synth
  depends_on: [baseline, strict]
```

Isso quer dizer:

- `synth` depende de `baseline`
- `synth` depende de `strict`
- quando `baseline` e `strict` terminam, suas saidas podem ser empacotadas e entregues para `synth`

O runtime monta canais entre universos nesse formato logico:

```text
u1.result->u3.context#summary/text
u2.result->u3.context#diagnostic/test
```

Ou seja:

- universo de origem
- porta de saida da origem
- universo de destino
- porta de entrada do destino
- tipo semantico do handoff

No input do universo de destino, isso vira um bloco compilado parecido com:

```text
[multiverse_handoffs]
kind: context/compiled
target: synth
- channel: u1.result->u3.context#summary/text
  kind: summary/text
  from: baseline
  status: ok
  sink_outputs:
    - agent: guide
      text: ...
```

Em outras palavras:

- o universo downstream nao recebe o output bruto inteiro de forma cega
- ele recebe uma versao estruturada e compacta
- o runtime pode truncar o texto conforme a configuracao

## Campos que controlam a comunicacao

### `depends_on`

Define de quais universos o universo atual depende.

Exemplo:

```yaml
depends_on: [u_backend, u_test]
```

Sem isso, nao existe handoff entre universos.

### `input_port`

Define em qual porta o universo downstream recebe o handoff.

Exemplo:

```yaml
input_port: context
```

Se ficar vazio, o runtime normalmente usa `context`.

### `output_port`

Define qual porta representa a saida do universo upstream.

Exemplo:

```yaml
output_port: result
```

### `output_kind`

Define o tipo da saida publicada.

Exemplo:

```yaml
output_kind: summary/text
output_kind: diagnostic/test
output_kind: plan/summary
```

Isso ajuda o runtime a rotular a comunicacao.

### `merge_policy`

Define como varios universos upstream entram no universo downstream.

Exemplos:

```yaml
merge_policy: append
merge_policy: latest
```

Leitura pratica:

- `append`: junta contribuicoes de varios upstreams
- `latest`: usa so a contribuicao mais recente disponivel

### `handoff_max_chars`

Controla quantos caracteres de cada saida entram no handoff.

Exemplo:

```yaml
handoff_max_chars: 220
```

Isso existe para:

- reduzir ruido
- economizar tokens
- evitar replay exagerado de texto bruto

## Quando os universos nao se comunicam

Os universos nao se comunicam quando:

- nao existe `depends_on`
- voce roda `--multiverse N` sem um crew com universos conectados

Exemplo:

```bash
deeph run --multiverse 3 guide "task"
```

Nesse caso, o runtime cria clones de universo.
Eles servem como branches paralelos, mas sem handoffs entre eles por default.

Ou seja:

- varios universos podem existir
- nem todos conversam entre si
- a comunicacao depende do DAG de `depends_on`

## Como inspecionar isso na CLI

### Ver o crew

```bash
deeph crew show reviewpack
```

Isso mostra:

- `spec`
- universos
- `depends_on`
- `input_port`
- `output_port`
- `output_kind`
- `merge_policy`
- `handoff_max_chars`

### Ver a orquestracao antes de rodar

```bash
deeph trace --multiverse 0 @reviewpack "task"
```

Aqui voce consegue ver:

- branches
- scheduler
- `universe_handoffs`

### Rodar o multiverse

```bash
deeph run --multiverse 0 @reviewpack "task"
```

Quando ha comunicacao entre universos, o runtime usa scheduler estilo DAG/channels.
Quando nao ha dependencia entre eles, o scheduler fica paralelo.

## Quando usar workflow simples, crew ou multiverse

### Workflow simples

Use quando:

- a tarefa e curta
- voce esta explorando
- nao precisa salvar o fluxo

### Crew

Use quando:

- quer nomear o workflow
- quer reaproveitar o fluxo
- quer compartilhar com o time

### Multiverse

Use quando:

- quer varias abordagens
- quer baseline vs strict
- quer backend vs frontend vs synth
- quer reconciliacao entre branches

Regra pratica:

- workflow simples para experimentar
- crew para padronizar
- universos para variantes coordenadas

## Erros comuns

### Achar que todo multiverse tem comunicacao

Nao tem.

Sem `depends_on`, os universos podem rodar separados.

### Criar ciclo entre universos

Exemplo ruim:

```yaml
- name: a
  depends_on: [b]

- name: b
  depends_on: [a]
```

Isso cria ciclo e o runtime falha.

### Esquecer `output_kind`

O runtime consegue funcionar com defaults, mas a comunicacao fica menos explicita.

### Tentar usar universo fora de crew

Universo e conceito de `crew` com `universes:`.
Se nao houver crew, nao ha esse nivel de configuracao.

## Resumo rapido

- workflow = forma de organizar a execucao
- crew = workflow salvo em arquivo
- universo = uma variante/branch dentro de um crew
- universos se comunicam via `depends_on`
- a comunicacao usa `input_port`, `output_port`, `output_kind`, `merge_policy` e `handoff_max_chars`
- sem `depends_on`, normalmente nao ha handoff entre universos

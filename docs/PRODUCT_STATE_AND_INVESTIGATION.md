# Estado do Produto e Investigacao do deepH

Este documento resume:

- o que mudou no `deepH` nas ultimas rodadas
- como isso melhora o produto para usuarios reais
- onde o `deepH` esta forte hoje
- quais limites ainda existem
- para onde a investigacao tecnica esta apontando

Nao e um texto de marketing.
O foco aqui e explicar o estado real do produto.

## Resumo curto

Nas rodadas mais recentes, o `deepH` evoluiu em 3 eixos:

1. `chat`
2. `review`
3. `studio`

O resultado e um produto mais coerente:

- o chat esta mais controlado e previsivel
- o review deixou de ser generico e ficou diff-aware
- o studio deixou de ser so um menu e ficou mais proximo de um cockpit

## O que mudou

## 1. Chat

O chat recebeu mudancas de arquitetura e UX.

Principais pontos:

- roteamento local antes de chamar LLM
- barramento `deeph-only` para `/exec`
- actor por sessao
- estado operacional mais claro
- recibo do ultimo comando
- terminal mais legivel

Arquivos principais:

- [cmd/deeph/chat_router.go](../cmd/deeph/chat_router.go)
- [cmd/deeph/chat_command_bus.go](../cmd/deeph/chat_command_bus.go)
- [cmd/deeph/chat_session_actor.go](../cmd/deeph/chat_session_actor.go)
- [cmd/deeph/chat_session.go](../cmd/deeph/chat_session.go)
- [cmd/deeph/term_ui.go](../cmd/deeph/term_ui.go)

Impacto pratico:

- menos ambiguidade no fluxo do chat
- menos risco de execucao fora do `deeph`
- melhor retomada de sessao
- menos cara de transcript cru e mais cara de produto

## 2. Review

O `deepH` ganhou uma capacidade propria de review.

O review agora:

- parte do `git diff`
- monta um `working set` Go-aware
- expande contexto por package, testes, imports e reverse imports
- usa uma crew oficial de review
- sintetiza melhor os achados entre universos

Arquivos principais:

- [cmd/deeph/review.go](../cmd/deeph/review.go)
- [internal/reviewscope/reviewscope.go](../internal/reviewscope/reviewscope.go)
- [crews/reviewflow.yaml](../crews/reviewflow.yaml)
- [agents/reviewer.yaml](../agents/reviewer.yaml)
- [agents/review_synth.yaml](../agents/review_synth.yaml)

Depois disso, o review recebeu mais duas camadas:

### Findings estruturados nos handoffs

Os universos de review agora conseguem passar `parsed_findings` para o synth, e nao apenas texto cru.

Arquivos:

- [internal/reviewfindings/reviewfindings.go](../internal/reviewfindings/reviewfindings.go)
- [cmd/deeph/multiverse.go](../cmd/deeph/multiverse.go)

Impacto:

- menos perda de sinal no synth
- melhor deduplicacao
- melhor separacao entre finding real e comentario solto

### Expansao semantica leve em Go

O `reviewscope` passou a usar AST leve para capturar simbolos tocados pelos hunks.

Hoje ele tenta priorizar:

- testes que realmente referenciam o simbolo alterado
- arquivos importados que declaram o simbolo usado no diff
- consumidores reversos que chamam simbolos exportados alterados

Isso fica em:

- [internal/reviewscope/reviewscope.go](../internal/reviewscope/reviewscope.go)

Impacto:

- menos contexto generico por package
- melhor custo por review
- menos chance de poluir o prompt com arquivos irrelevantes

## 3. Studio

O `studio` mudou de um menu plano para um fluxo mais guiado.

Hoje ele esta organizado em:

- `Quick Resume`
- `Start`
- `Build`
- `Review`
- `Operate`
- `Advanced`

Arquivos:

- [cmd/deeph/studio.go](../cmd/deeph/studio.go)
- [docs/STUDIO_MANUAL.md](./STUDIO_MANUAL.md)

Melhorias concretas:

- `Quick Resume` no topo
- acao recomendada baseada no estado do workspace
- `Review current changes` como fluxo proprio
- preview do comando `deeph ...` antes da execucao

Impacto:

- menos carga cognitiva
- onboarding mais facil
- operacao diaria mais direta
- usuario aprende a CLI ao usar o studio

## Como isso melhora o produto

## Para usuarios

- menos confusao no chat
- review mais util sem configuracao pesada
- studio mais facil para retomar trabalho
- menor dependencia de contexto bruto
- melhor custo em tarefas de review

## Para o produto

- menos acoplamento em `cmd/`
- mais reutilizacao de capacidades internas
- review virou capacidade de plataforma, nao so comando
- universos passaram a fazer mais sentido em cima de um escopo melhor

## Onde o deepH esta forte hoje

Hoje o `deepH` ja esta razoavelmente forte em:

- chat operacional com execucao `deeph-only`
- review diff-aware em Go
- handoffs tipados no runtime
- budget de contexto e observabilidade
- UX de terminal melhor que um CLI cru

Para repos pequenos e medios, isso ja entrega valor real.

Para PRs localizados em repos maiores, o caminho tambem esta bom.

## Limites atuais

Alguns limites continuam importantes:

1. O chat ainda nao tem compaction de elite.
   Melhorou, mas ainda nao e o nivel maximo de sessao longa.

2. O review ainda usa heuristica AST leve.
   Isso e bom e barato, mas nao substitui analise semantica profunda.

3. Ainda faltam benchmarks fortes em repos grandes.
   Hoje temos boa direcao tecnica, mas pouca prova quantitativa de escala.

4. Findings estruturados ainda dependem de parser tolerante.
   Isso ajuda bastante, mas nao e o mesmo que ter contrato rigido ponta a ponta.

5. O studio melhorou muito, mas ainda pode virar um painel operacional melhor.

## Avaliacao critica

O `deepH` melhorou bastante, mas ainda nao esta resolvido para codebase enorme.

O que ja esta certo:

- a direcao de arquitetura
- o foco em selecao de contexto
- a aposta em handoffs tipados
- o review diff-first

O que ainda falta para chamar de forte em projeto muito grande:

- compaction melhor de memoria
- indexacao semantica mais forte
- benchmark real de qualidade/custo/latencia
- mais prova em provider real

## Como esta a investigacao hoje

A investigacao atual aponta para 3 linhas principais.

## 1. Contexto e memoria

Esse e o proximo topico natural.

Perguntas centrais:

- como evitar transcript cru em sessoes longas
- como compactar memoria sem perder estado util
- como manter um `working set` claro por turno

Direcao tecnica:

- memoria tipada
- compaction incremental
- reaproveitamento de blocos de contexto
- separacao entre memoria operacional e conversa crua

## 2. Review e semantica

Aqui a pergunta e:

- como sair de heuristica leve para impacto mais semantico sem matar a leveza

Direcao tecnica:

- `go/packages` ou indice persistente
- expansao por simbolo e consumidor
- possivel call graph simplificado
- melhor correlacao entre diff, testes e runtime errors

## 3. UX operacional e metricas

Aqui o foco e:

- reduzir friccao
- tornar o produto mais ensinavel
- medir qualidade de selecao de contexto

Direcao tecnica:

- mais fluxo util no studio
- renderizacao melhor de findings
- metricas de budget, drop rate e review yield

## Comandos uteis para experimentar o estado atual

Chat:

```bash
deeph chat guide
```

Review:

```bash
deeph review
deeph review --trace
deeph review --json
```

Studio:

```bash
deeph
deeph studio
```

Atualizacao:

```bash
deeph update
```

## Conclusao

Hoje o `deepH` ja nao parece so um scaffold de agents.

Ele comecou a virar um runtime com opiniao clara sobre:

- contexto
- review
- fluxo operacional

Ainda ha lacunas reais para projeto grande.
Mas a base tecnica ja esta bem melhor do que uma abordagem centrada em prompt e transcript.

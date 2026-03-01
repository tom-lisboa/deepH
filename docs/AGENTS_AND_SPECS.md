# Agents e Agent Spec no deepH

Guia pratico para entender:

- o que e um agent
- como funciona o arquivo `agents/*.yaml`
- o que significa um agent spec
- quando usar `+`
- quando usar `>`
- quando usar `@crew`
- como conversar com um agent simples ou com varios agents

Os links abaixo sao clicaveis no GitHub.

## Navegacao rapida

- [O que e um agent](#o-que-e-um-agent)
- [Anatomia do arquivo do agent](#anatomia-do-arquivo-do-agent)
- [O que e um agent spec](#o-que-e-um-agent-spec)
- [Como funciona o operador plus (+)](#como-funciona-o-operador-plus-)
- [Como funciona o operador maior-que (>) ](#como-funciona-o-operador-maior-que-)
- [Combinar plus (+) e maior-que (>)](#combinar-plus--e-maior-que-)
- [Quando usar cada formato](#quando-usar-cada-formato)
- [Conversa com agents no chat](#conversa-com-agents-no-chat)
- [Quando usar `@crew` ou `crew:nome`](#quando-usar-crew-ou-crewnome)
- [Erros comuns](#erros-comuns)

## O que e um agent

No `deepH`, um agent e uma unidade de trabalho definida por voce em YAML.

Ele normalmente tem:

- nome
- descricao
- provider/model
- `system_prompt`
- skills opcionais
- regras de entrada e saida

Exemplo simples:

```yaml
name: guide
description: Agente inicial do workspace
provider: deepseek
model: deepseek-chat
system_prompt: |
  Voce e um agente guia.
  Explique o que vai fazer e seja claro nas suposicoes.
skills: []
```

## Anatomia do arquivo do agent

Campos mais importantes:

- `name`: nome unico do agent
- `description`: resumo curto do papel dele
- `provider`: qual provider ele usa
- `model`: qual modelo ele usa
- `system_prompt`: comportamento base do agent
- `skills`: ferramentas locais que ele pode usar

Campos avancados:

- `depends_on`: dependencia explicita em outro agent
- `depends_on_ports`: roteamento mais fino por agent/porta
- `io.inputs` e `io.outputs`: contratos tipados de entrada e saida
- `startup_calls`: skills que rodam antes da geracao
- `metadata`: limites e ajustes de contexto/tool budget

Exemplo com mais estrutura:

```yaml
name: reviewer
description: Revisa a resposta final
provider: deepseek
model: deepseek-chat
system_prompt: |
  Revise a saida anterior e aponte riscos.
skills:
  - file_read_range
depends_on: [planner]
depends_on_ports:
  brief: [planner.summary]
io:
  inputs:
    - name: brief
      accepts: [summary/text, text/plain]
      merge_policy: latest
  outputs:
    - name: answer
      produces: [text/markdown]
```

## O que e um agent spec

Agent spec e a forma curta de dizer para o `deepH` quais agents vao rodar e em que ordem.

Exemplos:

```text
guide
planner+reader
planner>coder
planner+reader>coder>reviewer
@reviewpack
crew:reviewpack
```

Esses specs sao usados em:

- `deeph run`
- `deeph trace`
- `deeph chat`

## Como funciona o operador plus (+)

`+` significa: agents na mesma etapa.

Exemplo:

```text
planner+reader
```

Isso quer dizer:

- `planner` e `reader` fazem parte da mesma stage
- eles tentam rodar em paralelo
- um nao depende do outro por causa do spec

Use `+` quando voce quer:

- duas perspectivas independentes
- duas leituras paralelas
- comparar respostas
- produzir material para um agente posterior sintetizar

Exemplo:

```bash
deeph run "planner+reader" "analise este repositorio"
```

No chat:

```bash
deeph chat "planner+reader"
```

Quando evitar:

- quando o segundo agent precisa da saida do primeiro

Nesse caso, use `>`.

## Como funciona o operador maior-que (>)

`>` significa: proxima etapa.

Exemplo:

```text
planner>coder
```

Isso quer dizer:

- `planner` roda antes
- `coder` roda depois
- por padrao, a etapa anterior alimenta a etapa seguinte

Na implementacao atual, a etapa anterior alimenta a proxima por default.
Ou seja, em `a>b`, `b` recebe handoffs de `a`.

Use `>` quando voce quer:

- pipeline
- passagem de contexto
- um agent preparando terreno para outro
- planejar antes de implementar
- gerar antes de revisar

Exemplo:

```bash
deeph run "planner>coder" "implemente autenticacao"
```

Outro exemplo:

```bash
deeph run "writer>reviewer" "escreva e revise este texto"
```

## Combinar plus (+) e maior-que (>)

Voce pode combinar os dois operadores.

Exemplo:

```text
planner+reader>coder>reviewer
```

Leitura correta:

- `planner` e `reader` rodam na mesma etapa
- `coder` roda depois
- `reviewer` roda por ultimo

Esse e o caso classico de:

1. abrir frentes paralelas
2. sintetizar ou implementar depois
3. revisar no final

Outro exemplo importante:

```text
planner+critic>synth
```

Leitura:

- `planner` e `critic` trabalham em paralelo
- `synth` junta e reconcilia as duas saidas

## Quando usar cada formato

### `guide`

Use para:

- conversa normal
- onboarding
- perguntas simples
- fluxo mais barato e previsivel

### `a+b`

Use para:

- duas visoes independentes
- exploracao paralela
- comparar ideias
- preparar material para um proximo agent

### `a>b`

Use para:

- fluxo sequencial
- passagem de contexto
- trabalho em etapas

### `a+b>c`

Use para:

- duas analises paralelas seguidas de sintese
- dois especialistas alimentando um terceiro
- consolidacao final

### `a+b>c>d`

Use para:

- pipelines mais estruturados
- planejar, executar, revisar, sintetizar

## Conversa com agents no chat

O `chat` aceita agent spec tambem.

Exemplos:

```bash
deeph chat guide
deeph chat "planner>coder"
deeph chat "planner+reader>coder>reviewer"
```

Quando conversar com um agent simples:

- quase sempre
- onboarding
- uso diario
- perguntas rapidas
- custo e latencia menores

Quando conversar com varios agents:

- quando a tarefa pede etapas claras
- quando voce quer separacao de papeis
- quando faz sentido ter sintese ou revisao

Exemplos bons de conversa multi-agent:

- `planner>coder`: planejar e depois implementar
- `reader+critic>synth`: leitura paralela seguida de consolidacao
- `writer>reviewer`: escrever e revisar

Quando evitar multi-agent no chat:

- perguntas simples
- prompts curtos demais
- quando voce nao precisa de especializacao

Regra pratica:

- se um agent resolve bem, prefira um agent
- se voce precisa de papeis diferentes, use multi-agent

## Quando usar `@crew` ou `crew:nome`

`@crew` e `crew:nome` sao aliases para um preset salvo em `crews/`.

Exemplos:

```bash
deeph run @reviewpack "task"
deeph trace crew:reviewpack "task"
```

No Windows PowerShell, prefira aspas simples com `crew:nome`:

```powershell
deeph run 'crew:reviewpack' 'task'
deeph trace 'crew:reviewpack' 'task'
```

Use crew quando voce quer:

- reaproveitar um workflow pronto
- dar um nome para um spec longo
- configurar universos/multiverso

Use agent spec direto quando voce quer:

- testar rapido
- iterar sem criar arquivo de crew

## Erros comuns

### Repetir o mesmo agent no spec

Isso falha:

```text
planner+planner
```

No parser atual, nomes duplicados no mesmo spec nao sao permitidos.

### Tentar usar `+` quando precisava de ordem

Isso:

```text
planner+coder
```

nao garante que `coder` espere `planner`.

Se precisa esperar, use:

```text
planner>coder
```

### Colocar um agent dependente em etapa anterior

Se um agent depende de outro, ele nao pode aparecer antes no spec.

Errado:

```text
coder>planner
```

se `coder` depende de `planner`.

### Escrever spec invalido

Exemplos invalidos:

```text
a++b
a>>b
>a
a>
```

## Resumo rapido

- `guide`: um agent
- `a+b`: mesma etapa, paralelismo
- `a>b`: proxima etapa, handoff
- `a+b>c`: paralelos antes, sintese depois
- `@crew`: workflow nomeado salvo em `crews/`

Se estiver em duvida:

- comece com um agent
- use `>` quando houver dependencia clara
- use `+` quando houver trabalho independente
- use `a+b>c` quando quiser sintese

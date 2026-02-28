# Manual do Studio, Workspace e Chat no deepH

Guia pratico para entender:

- o que e o `studio`
- o que e um `workspace`
- como o `deepH` escolhe onde trabalhar
- como criar, abrir e trocar workspace
- como abrir um chat pelo `studio`
- como abrir um chat direto pela CLI

Os links abaixo sao clicaveis no GitHub.

## Navegacao rapida

- [O que e o studio](#o-que-e-o-studio)
- [O que e um workspace](#o-que-e-um-workspace)
- [Criando a pasta do workspace](#criando-a-pasta-do-workspace)
- [Como o deepH escolhe o workspace](#como-o-deeph-escolhe-o-workspace)
- [Como abrir o studio](#como-abrir-o-studio)
- [O launcher inicial](#o-launcher-inicial)
- [Criar workspace com DeepSeek](#criar-workspace-com-deepseek)
- [Criar workspace com local-mock](#criar-workspace-com-local-mock)
- [Abrir um workspace existente](#abrir-um-workspace-existente)
- [Trocar de workspace no studio](#trocar-de-workspace-no-studio)
- [Como abrir um chat no studio](#como-abrir-um-chat-no-studio)
- [Como abrir um chat direto pela CLI](#como-abrir-um-chat-direto-pela-cli)
- [Como as sessoes funcionam](#como-as-sessoes-funcionam)
- [Fluxo recomendado com IDE](#fluxo-recomendado-com-ide)
- [Erros comuns](#erros-comuns)

## O que e o studio

O `studio` e o modo interativo do `deepH`.

Ele existe para facilitar o fluxo mais comum:

- criar workspace
- configurar provider
- criar agent
- validar o projeto
- rodar um agent uma vez
- abrir chat
- trocar de workspace

Se voce rodar `deeph` num terminal interativo, ele abre o `studio` por padrao.

Tambem da para abrir explicitamente:

```bash
deeph studio
```

Ou apontando para uma pasta especifica:

```bash
deeph studio --workspace ./meu-workspace
```

## O que e um workspace

Workspace e a pasta onde o `deepH` trabalha.

Em termos praticos, e a raiz do projeto do `deepH`. E nela que ficam:

- `deeph.yaml`
- `agents/`
- `skills/`
- `sessions/`
- `crews/` quando existir

Exemplo:

```text
meu-workspace/
├── deeph.yaml
├── agents/
├── skills/
└── sessions/
```

O arquivo que marca a raiz do workspace e `deeph.yaml`.

Sem esse arquivo, o `deepH` entende que a pasta ainda nao foi inicializada.

## Criando a pasta do workspace

O `studio` consegue criar e inicializar um workspace para voce, mas ajuda entender que primeiro existe uma pasta normal do sistema operacional.

Exemplo em macOS/Linux:

```bash
mkdir -p ~/deeph-workspace
```

Exemplo em Windows PowerShell:

```powershell
New-Item -ItemType Directory -Force -Path "$env:USERPROFILE\deeph-workspace" | Out-Null
```

Depois disso, o `deepH` vai usar essa pasta como base para criar `deeph.yaml`, `agents/`, `skills/` e `sessions/`.

## Como o deepH escolhe o workspace

O `studio` segue esta ordem:

1. se voce passou `--workspace`, ele usa esse caminho
2. se a pasta atual ja tem `deeph.yaml`, ele usa a pasta atual
3. se houver um ultimo workspace salvo pelo `studio`, ele tenta usar esse
4. se ainda nao houver workspace valido, ele abre o launcher inicial

Na pratica, isso significa:

- `deeph studio --workspace ./app` vence tudo
- se voce estiver dentro de um workspace valido, `deeph` ja abre nele
- se voce abriu o `studio` ontem num workspace valido, ele pode lembrar esse caminho hoje

## Como abrir o studio

Jeito mais simples:

```bash
deeph
```

Jeito explicito:

```bash
deeph studio
```

Jeito mais seguro quando voce quer garantir a pasta certa:

```bash
deeph studio --workspace /caminho/do/workspace
```

No Windows PowerShell:

```powershell
deeph studio --workspace "$env:USERPROFILE\deeph-workspace"
```

## O launcher inicial

Quando o `studio` abre fora de um workspace valido, ele nao tenta conversar direto. Primeiro ele mostra um launcher inicial.

Hoje esse launcher oferece:

1. `Create workspace (DeepSeek)`
2. `Create workspace (local mock)`
3. `Open existing workspace`
4. `Calculator workspace`
0. `Exit`

Esse launcher existe para evitar o erro classico de tentar usar `chat` numa pasta sem `deeph.yaml`.

## Criar workspace com DeepSeek

Essa e a opcao certa quando voce quer um workspace real para conversar com modelo.

Pelo `studio`:

1. abra `deeph`
2. escolha `Create workspace (DeepSeek)` no launcher inicial
3. confirme o caminho do workspace
4. escolha o nome do agent inicial, normalmente `guide`

Por baixo, isso faz um `quickstart` com DeepSeek e cria a base do projeto.

Tambem da para fazer direto pela CLI:

```bash
deeph quickstart --workspace ./meu-workspace --deepseek
```

Depois, se quiser reforcar a configuracao do provider:

```bash
deeph provider add --workspace ./meu-workspace --name deepseek --model deepseek-chat --set-default --force deepseek
```

## Criar workspace com local-mock

`local mock` e um provider falso, local, usado para validar fluxo sem custo e sem API key.

Use essa opcao quando voce quer:

- testar o `studio`
- validar se o workspace foi criado
- conferir se agents e skills estao carregando
- fazer onboarding sem depender de chave

Nao use essa opcao esperando uma conversa inteligente de verdade.

Pelo `studio`, escolha:

1. `Create workspace (local mock)`

Pela CLI:

```bash
deeph quickstart --workspace ./meu-workspace --provider local_mock --model mock-small
```

## Abrir um workspace existente

Se o workspace ja existe e ja tem `deeph.yaml`, use:

1. `Open existing workspace`

O `studio` vai pedir o caminho.

Se o caminho apontar para uma pasta sem `deeph.yaml`, ele vai recusar e avisar que o workspace nao foi inicializado.

Tambem da para abrir direto sem passar pelo launcher:

```bash
deeph studio --workspace ./meu-workspace
```

## Trocar de workspace no studio

Depois que o `studio` ja abriu, voce pode trocar de pasta sem sair dele.

Use a opcao:

`13) Switch workspace`

Isso e util quando:

- voce tem mais de um projeto
- quer mudar de um workspace de teste para um real
- quer sair de um workspace de calculadora e voltar para outro projeto

## Como abrir um chat no studio

No menu principal, use:

`7) Chat`

O fluxo atual e:

1. escolher o workspace
2. escolher o `agent spec`
3. escolher ou confirmar o `session id`
4. conversar

Quando houver agents no workspace, o `studio` lista os nomes e deixa voce selecionar por numero ou por nome.

Exemplo de `agent spec`:

- `guide`
- `planner+reader`
- `planner>coder`
- `planner+reader>coder>reviewer`

No starter padrao, o `guide` foi pensado para responder com comandos exatos do `deeph` e usar uma consulta curta ao dicionario de comandos so quando a sintaxe precisa ser confirmada.

Quando houver sessoes recentes, o `studio` tambem lista as ultimas para facilitar resume.

Dentro do chat, os comandos principais sao:

- `/help`
- `/history`
- `/trace`
- `/exit`

## Como abrir um chat direto pela CLI

Se voce nao quiser passar pelo menu, rode direto:

```bash
deeph chat --workspace ./meu-workspace guide
```

Exemplo com multi-agent:

```bash
deeph chat --workspace ./meu-workspace "planner+reader>coder>reviewer"
```

Exemplo retomando ou criando uma sessao nomeada:

```bash
deeph chat --workspace ./meu-workspace --session feat-login guide
```

No Windows PowerShell:

```powershell
deeph chat --workspace "$env:USERPROFILE\deeph-workspace" guide
```

## Como as sessoes funcionam

Cada conversa fica salva dentro do workspace, na pasta `sessions/`.

Arquivos usados:

- `sessions/<id>.meta.json`
- `sessions/<id>.jsonl`

Regras praticas:

- se voce passar `--session meu-id`, o `deepH` retoma essa sessao se ela existir
- se voce passar `--session meu-id` e ela nao existir, o `deepH` cria essa sessao
- se voce nao passar `--session`, o `deepH` gera um id automaticamente
- no `studio`, o campo de sessao pode vir preenchido com a sessao recente mostrada entre colchetes

Importante:

- uma sessao pertence a um `agent spec`
- se voce tentar reusar o mesmo `session id` com outro `agent spec`, o `deepH` vai recusar

## Fluxo recomendado com IDE

O fluxo mais saudavel hoje e este:

1. criar o workspace com `deeph`
2. abrir a pasta na sua IDE favorita
3. editar `agents/*.yaml`, `skills/*.yaml` e `crews/*.yaml`
4. voltar para `deeph validate`, `deeph run`, `deeph trace` e `deeph chat`

Ou seja:

- o `deepH` cria, valida e executa
- a IDE edita melhor do que um terminal

Exemplo:

```bash
deeph agent create --workspace ./meu-workspace reviewer
deeph validate --workspace ./meu-workspace
deeph chat --workspace ./meu-workspace reviewer
```

## Erros comuns

### O studio abriu, mas o chat falhou com `deeph.yaml` nao encontrado

Voce esta fora de um workspace valido.

Resolva com um destes caminhos:

```bash
deeph studio --workspace ./meu-workspace
```

ou:

```bash
deeph quickstart --workspace ./meu-workspace --deepseek
```

### O studio abriu na pasta errada

Passe `--workspace` explicitamente:

```bash
deeph studio --workspace ./meu-workspace
```

Ou use `13) Switch workspace`.

### O chat abre, mas o provider real nao responde

Confira:

- se o workspace usa `deepseek` como `default_provider`
- se a variavel `DEEPSEEK_API_KEY` esta presente na sessao atual
- se `deeph validate --workspace ./meu-workspace` esta limpo

### Quero so testar o fluxo sem gastar token

Crie um workspace com `local mock`.

### Quero um exemplo pronto para construir algo

Use:

- `12) Calculator workspace` no `studio`
- ou o guia [Calculadora estilo iPhone com deepH](IPHONE_CALCULATOR.md)

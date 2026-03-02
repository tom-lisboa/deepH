# CRUD com deepH

O comando `deeph crud` existe para transformar uma intencao simples do usuario em um fluxo previsivel de geracao e operacao.

O objetivo e este:

- o usuario informa a entidade e os campos
- o `deepH` escolhe a stack opinativa
- o `deepH` gera o projeto CRUD
- o usuario sobe e valida o ambiente com poucos comandos

## Stack opinativa

Hoje o fluxo oficial de CRUD parte destes defaults:

- backend: `Go`
- frontend: `Next.js`
- banco relacional: `Postgres`
- banco nao relacional: `MongoDB`

Se o usuario escolher `backend-only`, o fluxo nao gera frontend.

## Fluxo recomendado

### 1. Inicializar o workspace CRUD

```bash
deeph crud init --workspace ./futebol
```

Em terminal interativo, o `deepH` abre um wizard e pergunta:

- `backend-only` ou `fullstack`
- banco `relational` ou `document`
- engine do banco, como `postgres` ou `mongodb`
- entidade principal, como `players`
- campos, como `nome:text,posicao:text,time_id:int`
- se quer containers locais

As respostas sao salvas em:

- `.deeph/crud.json`

Exemplo de perfil salvo:

```json
{
  "version": 1,
  "entity": "players",
  "fields": [
    { "name": "nome", "type": "text" },
    { "name": "posicao", "type": "text" },
    { "name": "time_id", "type": "int" }
  ],
  "db_kind": "relational",
  "db": "postgres",
  "backend": "go",
  "backend_only": true,
  "containers": true
}
```

Se quiser pular o wizard, passe tudo por flag:

```bash
deeph crud init \
  --workspace ./futebol \
  --mode backend \
  --db-kind relational \
  --db postgres \
  --entity players \
  --fields nome:text,posicao:text,time_id:int \
  --containers=true \
  --no-prompt
```

### 2. Inspecionar o plano antes de gerar

```bash
deeph crud trace --workspace ./futebol
```

Esse comando monta o prompt final do CRUD e escolhe o crew certo para o perfil salvo.

Hoje os crews sao:

- `crud_backend_relational`
- `crud_fullstack_relational`
- `crud_backend_document`
- `crud_fullstack_document`

### 3. Gerar o CRUD

```bash
deeph crud run --workspace ./futebol
```

O `deepH` parte do perfil salvo, escolhe o crew correto e executa universos especializados para:

- contrato CRUD
- schema/tabela ou colecao
- rotas HTTP
- backend em `Go`
- infraestrutura local
- smoke tests
- frontend em `Next.js`, quando o modo for `fullstack`
- sintese final

### 4. Subir o ambiente local

```bash
deeph crud up --workspace ./futebol
```

Esse comando:

- procura `docker-compose.yml`, `docker-compose.yaml`, `compose.yml` ou `compose.yaml`
- usa `docker compose` quando disponivel
- cai para `docker-compose` quando necessario
- sobe os containers
- tenta esperar a API ficar acessivel

### 5. Validar o CRUD

```bash
deeph crud smoke --workspace ./futebol
```

O `deepH` tenta primeiro rodar:

- `scripts/smoke.sh`
- `scripts/smoke.ps1`

Se o projeto nao tiver script gerado, ele cai para um smoke test HTTP embutido:

- `POST`
- `GET` lista
- `GET` por id
- `PUT` ou `PATCH`
- `DELETE`

### 6. Derrubar o ambiente

```bash
deeph crud down --workspace ./futebol
```

Se quiser derrubar tambem os volumes do banco:

```bash
deeph crud down --workspace ./futebol --volumes
```

## Exemplo completo

### Backend-only relacional

```bash
deeph crud init --workspace ./futebol
deeph crud trace --workspace ./futebol
deeph crud run --workspace ./futebol
deeph crud up --workspace ./futebol
deeph crud smoke --workspace ./futebol
deeph crud down --workspace ./futebol
```

Respostas do wizard:

- modo: `backend-only`
- banco: `relational`
- engine: `postgres`
- entidade: `players`
- campos: `nome:text,posicao:text,time_id:int`
- containers: `yes`

### Fullstack relacional com flags

```bash
deeph crud init \
  --workspace ./crm \
  --mode fullstack \
  --db-kind relational \
  --db postgres \
  --entity customers \
  --fields nome:text,cidade:text,email:text \
  --containers=true \
  --no-prompt

deeph crud run --workspace ./crm
deeph crud up --workspace ./crm
deeph crud smoke --workspace ./crm
```

### Documento com MongoDB

```bash
deeph crud init \
  --workspace ./catalogo \
  --mode backend \
  --db-kind document \
  --db mongodb \
  --entity products \
  --fields nome:text,sku:text,estoque:int \
  --containers=true \
  --no-prompt

deeph crud run --workspace ./catalogo
```

## O que o deepH pede para os agentes entregarem

O prompt gerado pelo `deeph crud` orienta os agentes a entregar:

- rotas CRUD completas
- tabela final de rotas HTTP com metodo, path e payload
- endpoint `GET /health`
- persistencia no banco
- migration SQL ou equivalente
- `Dockerfile`
- `docker-compose`
- `scripts/smoke.sh`
- `scripts/smoke.ps1`
- `README` com comandos exatos

Para banco relacional, a modelagem deve ser explicita.
Para banco nao relacional, o foco e colecao, ids e indices previsiveis.

## O que acontece por baixo

O fluxo passa por estes agentes:

- `crud_contract`
- `crud_schema`
- `crud_routes`
- `crud_backend`
- `crud_infra`
- `crud_tester`
- `crud_frontend`
- `crud_synth`

O agente `crud_routes` e o especialista de rotas.
Ele existe para forcar uma saida clara de:

- metodo HTTP
- path
- payload
- status code

## Como o usuario pode ajustar o fluxo

O usuario nao precisa apagar o perfil salvo para mudar de direcao.

Exemplos:

Trocar para fullstack sem editar arquivo:

```bash
deeph crud run --workspace ./futebol --mode fullstack
```

Trocar a entidade e os campos:

```bash
deeph crud run \
  --workspace ./futebol \
  --entity teams \
  --fields nome:text,cidade:text,pais:text
```

Rodar smoke apontando para uma URL especifica:

```bash
deeph crud smoke --workspace ./futebol --base-url http://127.0.0.1:8080
```

Rodar sem script gerado e forcar o smoke embutido:

```bash
deeph crud smoke --workspace ./futebol --no-script
```

## Windows PowerShell

Os comandos `deeph crud ...` sao os mesmos no PowerShell:

```powershell
deeph crud init --workspace .\futebol
deeph crud run --workspace .\futebol
deeph crud up --workspace .\futebol
deeph crud smoke --workspace .\futebol
deeph crud down --workspace .\futebol
```

O `deepH` tenta usar:

- `docker compose`
- `docker-compose`

Se o projeto gerado tiver `scripts/smoke.ps1`, o `deeph crud smoke` executa esse script primeiro no Windows.

## Troubleshooting rapido

### `deeph crud up` nao encontrou compose

Rode primeiro:

```bash
deeph crud run --workspace ./futebol
```

Ou indique o arquivo manualmente:

```bash
deeph crud up --workspace ./futebol --compose-file ./deploy/docker-compose.yml
```

### `deeph crud smoke` nao detectou a URL da API

Passe a URL manualmente:

```bash
deeph crud smoke --workspace ./futebol --base-url http://127.0.0.1:8080
```

### Quero ver o prompt antes de gerar

```bash
deeph crud prompt --workspace ./futebol
```

### Quero revisar o crew escolhido

```bash
deeph crew show --workspace ./futebol crud_backend_relational
```

## Resumo do caminho ideal

Para a maioria dos usuarios, o caminho ideal hoje e:

```bash
deeph crud init --workspace ./meu-projeto
deeph crud run --workspace ./meu-projeto
deeph crud up --workspace ./meu-projeto
deeph crud smoke --workspace ./meu-projeto
```

Esse e o fluxo oficial para criar, subir e validar CRUDs com o `deepH`.

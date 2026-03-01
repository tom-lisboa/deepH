# deepH no Windows PowerShell

Guia rapido e reproduzivel para instalar o `deepH` no Windows usando PowerShell, configurar a API key do DeepSeek e abrir um workspace funcional.

Os links abaixo sao clicaveis no GitHub.

## Navegacao rapida

- [Passo 1: instalar o deeph](#passo-1-instalar-o-deeph)
- [Passo 2: confirmar o binario](#passo-2-confirmar-o-binario)
- [Passo 3: salvar e importar a API key](#passo-3-salvar-e-importar-a-api-key)
- [Passo 4: criar o workspace](#passo-4-criar-o-workspace)
- [Passo 5: configurar o provider DeepSeek](#passo-5-configurar-o-provider-deepseek)
- [Passo 6: abrir o studio e conversar](#passo-6-abrir-o-studio-e-conversar)
- [Passo 7: troubleshooting](#passo-7-troubleshooting)

## Passo 1: instalar o deeph

Abra o Windows PowerShell e rode:

```powershell
$tmp = Join-Path $env:TEMP "deeph-install.ps1"
iwr https://raw.githubusercontent.com/tom-lisboa/deepH/main/scripts/install.ps1 -UseBasicParsing -OutFile $tmp
Set-ExecutionPolicy -Scope Process Bypass -Force
& $tmp
```

Se o comando `deeph` ainda nao existir na mesma janela, feche o PowerShell e abra outro.

## Passo 2: confirmar o binario

Teste:

```powershell
deeph
```

Se preferir validar o executavel direto:

```powershell
& "$env:LOCALAPPDATA\Programs\deeph\deeph.exe"
```

## Passo 3: salvar e importar a API key

Troque `sk-SUA_CHAVE_REAL` pela chave real do DeepSeek.

Esse bloco faz duas coisas:

- salva a chave no perfil do usuario do Windows
- carrega a mesma chave na sessao atual do PowerShell

```powershell
[Environment]::SetEnvironmentVariable("DEEPSEEK_API_KEY", "sk-SUA_CHAVE_REAL", "User")
$env:DEEPSEEK_API_KEY = [Environment]::GetEnvironmentVariable("DEEPSEEK_API_KEY", "User")
```

Confirme:

```powershell
if ($env:DEEPSEEK_API_KEY) { "DEEPSEEK_API_KEY OK" } else { "DEEPSEEK_API_KEY vazio" }
```

Observacao importante:

- outra janela nova do PowerShell normalmente pega a chave
- outra aba ja aberta do Windows Terminal pode nao pegar
- terminal integrado do VS Code costuma precisar ser reiniciado

## Passo 4: criar o workspace

```powershell
$workspace = Join-Path $env:USERPROFILE "deeph-workspace"
New-Item -ItemType Directory -Force -Path $workspace | Out-Null
Set-Location $workspace

deeph quickstart --workspace $workspace --deepseek
deeph validate --workspace $workspace
```

Isso cria:

- `deeph.yaml`
- pasta `agents/`
- pasta `skills/`
- agent inicial `guide`

## Passo 5: configurar o provider DeepSeek

O timeout padrao pode ficar curto em alguns fluxos. Esse comando sobe para `120000ms`.

```powershell
deeph provider add --workspace $workspace --name deepseek --model deepseek-chat --timeout-ms 120000 --set-default --force deepseek
deeph validate --workspace $workspace
```

Se quiser checar os providers:

```powershell
deeph provider list --workspace $workspace
```

## Passo 6: abrir o studio e conversar

Opcao 1, abrir o studio no workspace:

```powershell
deeph studio --workspace $workspace
```

Opcao 2, entrar na pasta e abrir:

```powershell
Set-Location $workspace
deeph
```

Opcao 3, conversar direto sem menu:

```powershell
deeph chat --workspace $workspace guide
```

Teste simples:

```text
hello
```

## Passo 7: troubleshooting

### `deeph` nao e reconhecido

Feche a janela atual e abra um novo PowerShell.

Se ainda falhar:

```powershell
& "$env:LOCALAPPDATA\Programs\deeph\deeph.exe" studio --workspace $workspace
```

### A API key funciona em uma janela e em outra nao

Reimporte na sessao atual:

```powershell
$env:DEEPSEEK_API_KEY = [Environment]::GetEnvironmentVariable("DEEPSEEK_API_KEY", "User")
```

### O studio abriu na pasta errada

Abra explicitamente com workspace:

```powershell
deeph studio --workspace $workspace
```

### Quero um workspace de calculadora

Use o guia clicavel:

- [Calculadora estilo iPhone com deepH](IPHONE_CALCULATOR.md)

Ou rode direto:

```powershell
$workspace = Join-Path $env:USERPROFILE "deeph-calculadora"
New-Item -ItemType Directory -Force -Path $workspace | Out-Null

deeph quickstart --workspace $workspace --deepseek
deeph provider add --workspace $workspace --name deepseek --model deepseek-chat --timeout-ms 120000 --set-default --force deepseek
deeph kit add --workspace $workspace crud-next-multiverse --provider-name deepseek --model deepseek-chat --set-default-provider
deeph crew show --workspace $workspace crud_fullstack_multiverse
deeph run --workspace $workspace --multiverse 0 'crew:crud_fullstack_multiverse' "Crie uma calculadora fullstack com frontend Next.js, rotas API, controller/service, operacoes soma/subtracao/multiplicacao/divisao e testes basicos"
```

Observacoes:

- no PowerShell, prefira `'crew:nome'` em vez de `@nome` para evitar conflito de parsing do shell
- `deepH` carrega crews ativas de `crews/`; arquivos em `examples/crews/` servem como referencia e nao sao executados diretamente

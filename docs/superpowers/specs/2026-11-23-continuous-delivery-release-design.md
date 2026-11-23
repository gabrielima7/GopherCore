# Design Doc: Continuous Delivery and Automated Release Pipeline

**Data:** 2026-11-23  
**Status:** Proposto  
**Escopo:** Pequeno (Workflow e automação CI/CD)

---

## 1. Contexto e Motivação
O projeto GopherCore é uma biblioteca Go pura (`github.com/fsvxavier/gophercore`) que necessita de um pipeline contínuo de publicação de artefatos e automação de releases. O objetivo é eliminar o processo manual de lançamento, garantindo entrega contínua sempre que houver commit na branch `main` ou tag `v*`, com controle rigoroso de concorrência, versionamento semântico automatizado com rollover em `.10`, e proteção na etapa de deploy.

---

## 2. Objetivos e Anti-Objetivos
- **Objetivos:**
  - Disparar pipeline sob `push` na branch `main` e criação de tags `v*`.
  - Versionamento dinâmico automático iniciando em `v0.6.0`.
  - Regra de rollover de versão: `PATCH` sobe até `10`. Ao atingir `.10`, a próxima versão sobe o `MINOR` e zera o `PATCH` (`0.5.10` -> `0.6.0`).
  - Concorrência sem cancelamento de pipelines em andamento (`cancel-in-progress: false`).
  - Construir e publicar artefatos de produção (tarball/zip de distribuição Go, SBOM SPDX, checksums SHA256) em GitHub Releases.
  - Etapa de deploy dependente da publicação (`needs: release/publish`) executando em ambiente protegido (`environment: production`).
  - Geração automática de Release com changelog categorizado: Novas funcionalidades (`feat`), Alterações (`chore`, `refactor`, `perf`, `build`, `ci`), Correções (`fix`), e Remoções (`revert`, `deprecate`).
  - Listagem dos pacotes e arquivos gerados no corpo da Release.

- **Anti-Objetivos:**
  - Não criar imagens Docker desnecessárias (projeto é biblioteca Go pura).
  - Não cancelar builds/publicações anteriores em execução.

- **Anti-Goals (O que tornaria isso uma falha):**
  - Quebrar compatibilidade de importação do módulo Go.
  - Cancelar pipelines de publicação concorrentes que estejam enviando artefatos.
  - Pular ou permitir deploy sem validação de artefatos prévios.

---

## 3. Restrições e Premissas
- Plataforma: GitHub Actions nativo.
- Permissões mínimas no workflow: `contents: write`, `id-token: write` (para SLSA/provenance).
- Ponto de partida do versionamento: `v0.6.0`.
- Ambiente protegido de deploy: `production` (configurável para aprovações manuais no repositório).

---

## 4. Abordagens Consideradas
1. **Abordagem 1: Pipeline Nativo com Configuração GitHub (Escolhida)**
   - Utiliza `.github/release.yml` para categorização declarativa de release notes.
   - Script shell inline mínimo no workflow para cálculo aritmético determinístico da versão.
   - Utiliza `softprops/action-gh-release@v3` para publicação e geração automática de notas.
   - Job `deploy` consome ambiente protegido e valida integridade.
   - *Vantagens:* Sem dependências pesadas externas, código mínimo (Ponytail), alta confiabilidade.

2. **Abordagem 2: Script Customizado Externo (Git Log / Conventional Changelog CLI)**
   - Utiliza pacote Node/Go/CLI externo (`git-cliff` ou script bash complexo) para parsear commits.
   - *Desvantagens:* Mais dependências no runner, frágil contra mensagens fora de padrão, maior manutenção.

---

## 5. Arquitetura e Detalhes da Implementação

### 5.1 Gatilhos e Concorrência
```yaml
on:
  push:
    branches: [main]
    tags: ["v*"]

concurrency:
  group: ${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: false
```

### 5.2 Lógica de Versionamento (Step `determine-version`)
- Se `github.ref_type == 'tag'`: utiliza a tag atual (ex: `v0.6.0`).
- Se `github.ref_name == 'main'`:
  - Obtém a última tag semântica do git.
  - Se a última tag for anterior a `v0.6.0` (ex: `v0.5.1`), o ponto de partida é forçado para `v0.6.0`.
  - Para tags `vX.Y.Z >= v0.6.0`:
    - Se `Z < 10`: próxima versão = `vX.Y.(Z+1)`.
    - Se `Z >= 10`: próxima versão = `vX.(Y+1).0`.

### 5.3 Categorização do Changelog (`.github/release.yml`)
Categorias configuradas no repositório:
- 🚀 **Novas Funcionalidades** (`feat*`, `feature*`)
- 🛠 **Alterações** (`chore*`, `refactor*`, `perf*`, `build*`, `ci*`)
- 🐛 **Correções** (`fix*`, `bug*`)
- 🗑️ **Remoções** (`revert*`, `deprecat*`, `remov*`)

### 5.4 Publicação dos Artefatos
Gera e anexa à Release:
- Arquivo comprimido do código fonte / distribuição (`gophercore-${VERSION}.tar.gz`).
- SBOM SPDX (`gophercore-sbom.spdx.json`).
- Checksums SHA256 (`checksums.txt`).
- SLSA Provenance attestation.

### 5.5 Etapa de Deploy Protegida
```yaml
deploy:
  name: Deploy to Production
  runs-on: ubuntu-latest
  needs: [release]
  environment: production
  steps:
    - name: Verify Release Assets & Smoke Test
      run: |
        echo "Validating published package for version ${{ needs.release.outputs.version }}..."
```

---

## 6. O que Fica a Cargo do Builder
- Ajustar `.github/workflows/release.yml` para contemplar os novos gatilhos, concorrência, cálculo de versão e job de deploy.
- Criar `.github/release.yml` com as regras de categorização de changelog.
- Validar a execução local ou sintaxe dos arquivos YAML.

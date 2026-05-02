# Phone Validator - Documentacao de Telas

**Data:** 2026-02-06
**Status:** Planning

---

## Apps e Interfaces

Este sistema possui **duas interfaces** distintas:

### 1. App Mobile Validador (`/screens/mobile/`)
App instalado no celular do usuario final. Recebe push notifications e confirma validacoes.

| Tela | Arquivo | Prioridade |
|------|---------|------------|
| Splash / Welcome | `01-splash.md` | MVP |
| Registro de Telefone | `02-register-phone.md` | MVP |
| Verificacao Inicial (OTP) | `03-verify-phone.md` | MVP |
| Home / Dashboard | `04-home.md` | MVP |
| Validacao Recebida (Push) | `05-validation-request.md` | MVP |
| Historico de Validacoes | `06-history.md` | MVP |
| Configuracoes | `07-settings.md` | MVP |

### 2. Dashboard Web - Painel do Cliente (`/screens/dashboard/`)
Interface web para empresas (clientes SaaS) gerenciarem API keys, verem metricas e configuracoes.

| Tela | Arquivo | Prioridade |
|------|---------|------------|
| Login / Cadastro | `01-auth.md` | MVP |
| Dashboard Principal | `02-dashboard.md` | MVP |
| API Keys | `03-api-keys.md` | MVP |
| Logs de Validacoes | `04-validation-logs.md` | MVP |
| Configuracoes da Conta | `05-account-settings.md` | MVP |
| Documentacao da API | `06-api-docs.md` | Pos-MVP |
| Billing / Faturamento | `07-billing.md` | Pos-MVP |

---

## Fluxo Geral

```
[Cliente SaaS]                    [Usuario Final]
     │                                  │
     │  Dashboard Web                   │  App Mobile Validador
     │  ┌──────────────┐               │  ┌──────────────────┐
     │  │ Cria conta   │               │  │ Instala app      │
     │  │ Gera API Key │               │  │ Registra celular │
     │  └──────┬───────┘               │  └──────┬───────────┘
     │         │                        │         │
     │         ▼                        │         ▼
     │  Backend do cliente              │  Recebe push
     │  chama POST /validations         │  Confirma validacao
     │         │                        │         │
     │         ▼                        │         ▼
     │  Consulta status                 │  Ve historico
     │  GET /validations/:id            │
     └─────────────────────────────────────────────┘
```

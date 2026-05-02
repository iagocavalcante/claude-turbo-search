# Tela: Documentacao da API

**App:** Dashboard Web (Cliente SaaS)
**Prioridade:** Pos-MVP
**Tecnologia:** Phoenix + SwaggerUI

---

## Objetivo

Documentacao interativa da API para desenvolvedores do cliente, com exemplos e possibilidade de testar endpoints.

---

## Layout

```
┌──────────────────────────────────────────────────────────────────────┐
│  [Logo] Phone Validator          [Docs ←]  [Suporte]  [User ▼]     │
│──────────────────────────────────────────────────────────────────────│
│  │                                                                  │
│  │ Introducao         │  API de Validacao de Telefone               │
│  │ Autenticacao       │  ─────────────────────────────              │
│  │ Endpoints          │                                             │
│  │  POST /validations │  Base URL                                   │
│  │  GET /validations  │  https://api.phonevalidator.io/v1           │
│  │  GET /health       │                                             │
│  │ Webhooks           │  Autenticacao                               │
│  │ Erros              │  Todas as requisicoes devem incluir o       │
│  │ SDKs               │  header X-API-Key com sua chave de API.     │
│  │ Changelog          │                                             │
│  │                    │  ┌─────────────────────────────────────┐    │
│  │                    │  │ curl -X POST .../v1/validations \   │    │
│  │                    │  │   -H "X-API-Key: pk_live_xxx" \     │    │
│  │                    │  │   -H "Content-Type: application/... │    │
│  │                    │  │   -d '{"phone_number": "+5511..."}'│    │
│  │                    │  └─────────────────────────────────────┘    │
│  │                    │  [cURL] [JavaScript] [Python] [Elixir]      │
│  │                    │                                             │
│  │                    │  ─────────────────────────────────          │
│  │                    │                                             │
│  │                    │  POST /validations                          │
│  │                    │  Iniciar validacao de numero                │
│  │                    │                                             │
│  │                    │  Request Body                               │
│  │                    │  ┌─────────────────────────────────────┐    │
│  │                    │  │ {                                   │    │
│  │                    │  │   "phone_number": "+5511999999999", │    │
│  │                    │  │   "callback_url": "https://...",    │    │
│  │                    │  │   "metadata": {}                    │    │
│  │                    │  │ }                                   │    │
│  │                    │  └─────────────────────────────────────┘    │
│  │                    │                                             │
│  │                    │  Response 201                               │
│  │                    │  ┌─────────────────────────────────────┐    │
│  │                    │  │ {                                   │    │
│  │                    │  │   "id": "550e8400-...",              │    │
│  │                    │  │   "status": "sent",                 │    │
│  │                    │  │   "expires_at": "2026-02-..."       │    │
│  │                    │  │ }                                   │    │
│  │                    │  └─────────────────────────────────────┘    │
│  │                    │                                             │
│  │                    │  ┌──────────────────┐                      │
│  │                    │  │  Testar Endpoint  │                      │
│  │                    │  └──────────────────┘                      │
│  │                    │                                             │
└──┴────────────────────┴─────────────────────────────────────────────┘
```

---

## Secoes

### Introducao
- Visao geral do servico
- Como funciona o fluxo de validacao
- Diagrama do fluxo

### Autenticacao
- Como obter API Key
- Header X-API-Key
- Exemplos em multiplas linguagens

### Endpoints
- POST /validations - Iniciar validacao
- GET /validations/:id - Consultar status
- GET /health - Health check

### Webhooks
- Como configurar
- Formato do payload
- Verificacao de assinatura (HMAC)
- Eventos disponiveis

### Erros
- Tabela de codigos de erro
- Formato padrao de resposta de erro

### SDKs (pos-MVP)
- Links para SDKs oficiais

### Changelog
- Historico de mudancas da API

---

## Exemplos por Linguagem

```javascript
// JavaScript / Node.js
const response = await fetch('https://api.phonevalidator.io/v1/validations', {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
    'X-API-Key': 'pk_live_your_api_key'
  },
  body: JSON.stringify({
    phone_number: '+5511999999999'
  })
});
```

```python
# Python
import requests

response = requests.post(
    'https://api.phonevalidator.io/v1/validations',
    headers={'X-API-Key': 'pk_live_your_api_key'},
    json={'phone_number': '+5511999999999'}
)
```

```elixir
# Elixir
HTTPoison.post(
  "https://api.phonevalidator.io/v1/validations",
  Jason.encode!(%{phone_number: "+5511999999999"}),
  [{"X-API-Key", "pk_live_your_api_key"}, {"Content-Type", "application/json"}]
)
```

---

## Notas Tecnicas

- Swagger UI integrado via OpenApiSpex (ja configurado no router)
- Documentacao customizada como pagina estatica ou LiveView
- Botao "Testar Endpoint" pre-preenche com API key do usuario logado
- Syntax highlighting para code blocks
- Versionamento da API documentado

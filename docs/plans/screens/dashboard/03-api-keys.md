# Tela: API Keys

**App:** Dashboard Web (Cliente SaaS)
**Prioridade:** MVP
**Tecnologia:** Phoenix LiveView

---

## Objetivo

Gerenciar API keys usadas para autenticar chamadas a API de validacao.

---

## Layout: Lista

```
┌──────────────────────────────────────────────────────────────────────┐
│  [Logo] Phone Validator          [Docs]  [Suporte]  [User ▼]       │
│──────────────────────────────────────────────────────────────────────│
│  │                                                                  │
│  │ Dashboard          │  API Keys                                   │
│  │ API Keys  ←        │                                             │
│  │ Logs               │  Gerencie suas chaves de API para           │
│  │ Configuracoes      │  integrar com o servico de validacao.       │
│  │                    │                                             │
│  │                    │  ┌───────────────────────────────────────┐  │
│  │                    │  │  + Criar Nova API Key                 │  │
│  │                    │  └───────────────────────────────────────┘  │
│  │                    │                                             │
│  │                    │  ┌───────────────────────────────────────┐  │
│  │                    │  │  Production                           │  │
│  │                    │  │  pk_live_a1b2c3d4...                  │  │
│  │                    │  │                                       │  │
│  │                    │  │  Criada: 01/02/2026                   │  │
│  │                    │  │  Ultimo uso: Hoje, 14:32              │  │
│  │                    │  │  Status: ● Ativa                      │  │
│  │                    │  │                                       │  │
│  │                    │  │  [Copiar]  [Revogar]                  │  │
│  │                    │  └───────────────────────────────────────┘  │
│  │                    │                                             │
│  │                    │  ┌───────────────────────────────────────┐  │
│  │                    │  │  Staging                              │  │
│  │                    │  │  pk_test_e5f6g7h8...                  │  │
│  │                    │  │                                       │  │
│  │                    │  │  Criada: 01/02/2026                   │  │
│  │                    │  │  Ultimo uso: 05/02/2026               │  │
│  │                    │  │  Status: ● Ativa                      │  │
│  │                    │  │                                       │  │
│  │                    │  │  [Copiar]  [Revogar]                  │  │
│  │                    │  └───────────────────────────────────────┘  │
│  │                    │                                             │
│  │                    │  ┌───────────────────────────────────────┐  │
│  │                    │  │  Old Key                              │  │
│  │                    │  │  pk_live_z9y8x7w6...                  │  │
│  │                    │  │                                       │  │
│  │                    │  │  Criada: 15/01/2026                   │  │
│  │                    │  │  Revogada: 01/02/2026                 │  │
│  │                    │  │  Status: ○ Revogada                   │  │
│  │                    │  └───────────────────────────────────────┘  │
│  │                    │                                             │
└──┴────────────────────┴─────────────────────────────────────────────┘
```

---

## Modal: Criar Nova API Key

```
┌───────────────────────────────┐
│  Criar Nova API Key       [X] │
│                               │
│  Nome                         │
│  ┌─────────────────────────┐  │
│  │ Ex: Production          │  │
│  └─────────────────────────┘  │
│                               │
│  Expiracao (opcional)         │
│  ┌─────────────────────────┐  │
│  │ Nunca                 ▼ │  │
│  └─────────────────────────┘  │
│  Opcoes: Nunca, 30 dias,     │
│  90 dias, 1 ano              │
│                               │
│  [Cancelar]    [Criar Key]    │
└───────────────────────────────┘
```

---

## Modal: Key Criada (exibida UMA vez)

```
┌───────────────────────────────┐
│  API Key Criada!          [X] │
│                               │
│  ⚠️ Copie sua key agora.     │
│  Ela nao sera exibida         │
│  novamente.                   │
│                               │
│  ┌─────────────────────────┐  │
│  │ pk_live_a1b2c3d4e5f6g7  │  │
│  │ h8i9j0k1l2m3n4o5p6q7r8 │  │
│  │                [Copiar] │  │
│  └─────────────────────────┘  │
│                               │
│  [Entendi, ja copiei]         │
└───────────────────────────────┘
```

---

## Modal: Revogar Key

```
┌───────────────────────────────┐
│  Revogar API Key?         [X] │
│                               │
│  ⚠️ Esta acao e irreversivel. │
│  Todas as integracoes usando  │
│  esta key deixarao de         │
│  funcionar imediatamente.     │
│                               │
│  Key: pk_live_a1b2c3d4...     │
│  Nome: Production             │
│                               │
│  Digite "REVOGAR" para        │
│  confirmar:                   │
│  ┌─────────────────────────┐  │
│  │                         │  │
│  └─────────────────────────┘  │
│                               │
│  [Cancelar]    [Revogar Key]  │
└───────────────────────────────┘
```

---

## Card de API Key

| Campo | Descricao |
|-------|-----------|
| Nome | Nome dado pelo usuario (Production, Staging, etc) |
| Prefixo | Primeiros 16 chars da key + "..." |
| Data criacao | Data de criacao |
| Ultimo uso | Timestamp do ultimo uso (ou "Nunca usada") |
| Status | Ativa (verde), Revogada (cinza), Expirada (amarelo) |

---

## Comportamento

| Acao | Resultado |
|------|-----------|
| Tap "Criar Nova API Key" | Abre modal de criacao |
| Tap "Criar Key" (modal) | Gera key → Modal com key completa |
| Tap "Copiar" | Copia key/prefixo para clipboard |
| Tap "Revogar" | Modal de confirmacao |
| Digita "REVOGAR" + confirma | Key revogada imediatamente |
| Tap "Entendi, ja copiei" | Fecha modal, exibe card da nova key |

---

## Regras de Negocio

- Maximo de 5 API keys ativas por cliente (MVP)
- Key completa so e exibida uma vez (no momento da criacao)
- Revogacao e imediata e irreversivel
- Keys expiradas sao automaticamente invalidadas
- Keys revogadas permanecem visiveis na lista (para auditoria)

---

## API / Dados

```elixir
# Lista de keys
%{
  api_keys: [
    %{
      id: "uuid",
      name: "Production",
      key_prefix: "pk_live_a1b2c3d4",
      status: "active",
      created_at: ~U[2026-02-01 10:00:00Z],
      last_used_at: ~U[2026-02-06 14:32:00Z],
      expires_at: nil
    }
  ]
}
```

---

## Notas Tecnicas

- Key gerada com :crypto.strong_rand_bytes(32) + Base.url_encode64
- Formato: `pk_live_` (producao) ou `pk_test_` (teste) + 32 chars
- Armazenar apenas SHA-256 hash da key no banco
- Manter key_prefix (primeiros 8 chars) para identificacao
- Key completa disponivel apenas no momento da criacao (nunca mais)

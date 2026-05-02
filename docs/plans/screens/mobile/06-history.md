# Tela: Historico de Validacoes

**App:** Mobile Validador
**Prioridade:** MVP
**Plataformas:** iOS, Android

---

## Objetivo

Listar todas as validacoes recebidas pelo dispositivo, com filtros e detalhes.

---

## Layout

```
┌─────────────────────────────┐
│  Historico                  │
│─────────────────────────────│
│                             │
│  [Todos] [Validados] [Outros]│
│                             │
│  Hoje                       │
│  ─────────────────────────  │
│  ┌───────────────────────┐  │
│  │ ✓  Empresa XYZ        │  │
│  │    14:32 - Validado   │  │
│  └───────────────────────┘  │
│  ┌───────────────────────┐  │
│  │ ✓  App ABC            │  │
│  │    11:15 - Validado   │  │
│  └───────────────────────┘  │
│                             │
│  Ontem                      │
│  ─────────────────────────  │
│  ┌───────────────────────┐  │
│  │ ✗  Servico DEF        │  │
│  │    22:40 - Expirado   │  │
│  └───────────────────────┘  │
│  ┌───────────────────────┐  │
│  │ ✓  Empresa GHI        │  │
│  │    18:20 - Validado   │  │
│  └───────────────────────┘  │
│                             │
│  06 Fev 2026                │
│  ─────────────────────────  │
│  ┌───────────────────────┐  │
│  │ ⊘  Loja JKL           │  │
│  │    09:45 - Recusado   │  │
│  └───────────────────────┘  │
│                             │
│─────────────────────────────│
│  [Home]    [Historico]  [⚙️] │
└─────────────────────────────┘
```

---

## Detalhe da Validacao (ao tap no item)

```
┌─────────────────────────────┐
│ ←  Detalhe                  │
│─────────────────────────────│
│                             │
│     ┌─────────────────┐     │
│     │    ✓ Grande     │     │
│     └─────────────────┘     │
│                             │
│  Status: Validado           │
│                             │
│  ─────────────────────────  │
│  Solicitante                │
│  Empresa XYZ                │
│                             │
│  Data/Hora                  │
│  06/02/2026 14:32:15        │
│                             │
│  Tempo de Resposta          │
│  12 segundos                │
│                             │
│  ID da Validacao            │
│  550e8400-e29b-41d4-...     │
│  ─────────────────────────  │
│                             │
└─────────────────────────────┘
```

---

## Filtros

| Filtro | Descricao |
|--------|-----------|
| Todos | Todas as validacoes |
| Validados | Apenas status "validated" |
| Outros | Expirados, recusados, falhas |

---

## Item da Lista

| Campo | Descricao |
|-------|-----------|
| Icone status | ✓ verde, ✗ vermelho, ⏰ amarelo, ⊘ cinza |
| Nome solicitante | Nome do cliente SaaS |
| Hora | HH:MM se hoje, DD/MM se outro dia |
| Status texto | "Validado", "Expirado", "Recusado", "Falhou" |

---

## Comportamento

| Acao | Resultado |
|------|-----------|
| Scroll | Infinite scroll com paginacao |
| Tap filtro | Filtra lista, scroll para topo |
| Tap item | Abre detalhe da validacao |
| Pull-to-refresh | Recarrega lista |
| Tap "←" (detalhe) | Volta para lista |

---

## API Call

```
GET /internal/validations?page=1&per_page=20&status=all
Headers:
  X-Device-Token: device_token_here

Response 200:
{
  "validations": [
    {
      "id": "uuid",
      "status": "validated",
      "client_name": "Empresa XYZ",
      "created_at": "2026-02-06T14:32:00Z",
      "validated_at": "2026-02-06T14:32:15Z",
      "response_time_seconds": 12
    }
  ],
  "pagination": {
    "page": 1,
    "per_page": 20,
    "total": 45,
    "total_pages": 3
  }
}
```

---

## Estados Vazios

### Sem validacoes (geral)
```
Nenhuma validacao ainda.
Quando alguem solicitar validacao
do seu numero, aparecera aqui.
```

### Sem validacoes (filtro ativo)
```
Nenhuma validacao com este filtro.
```

---

## Notas Tecnicas

- Paginacao: 20 itens por pagina, infinite scroll
- Agrupar por data (Hoje, Ontem, data completa)
- Cache local para funcionamento offline
- Dados retidos por 90 dias (configuravel)

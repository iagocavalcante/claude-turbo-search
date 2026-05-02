# Tela: Logs de Validacoes

**App:** Dashboard Web (Cliente SaaS)
**Prioridade:** MVP
**Tecnologia:** Phoenix LiveView

---

## Objetivo

Visualizar e filtrar todas as validacoes realizadas pelo cliente, com detalhes de cada operacao.

---

## Layout

```
┌──────────────────────────────────────────────────────────────────────┐
│  [Logo] Phone Validator          [Docs]  [Suporte]  [User ▼]       │
│──────────────────────────────────────────────────────────────────────│
│  │                                                                  │
│  │ Dashboard          │  Logs de Validacoes                         │
│  │ API Keys           │                                             │
│  │ Logs  ←            │  ┌────────────────────────────────────────┐ │
│  │ Configuracoes      │  │ Buscar: [+5511...          ]          │ │
│  │                    │  │                                        │ │
│  │                    │  │ Status:  [Todos ▼]                     │ │
│  │                    │  │ Periodo: [Ultimos 7 dias ▼]            │ │
│  │                    │  │ API Key: [Todas ▼]                     │ │
│  │                    │  └────────────────────────────────────────┘ │
│  │                    │                                             │
│  │                    │  ┌────────────────────────────────────────┐ │
│  │                    │  │ ID        Numero       Status   Key    │ │
│  │                    │  │           Data/Hora    Resp.    Tempo  │ │
│  │                    │  │──────────────────────────────────────  │ │
│  │                    │  │ 550e...  +5511999...  ✓ Valid  Prod.  │ │
│  │                    │  │          06/02 14:32           8s     │ │
│  │                    │  │──────────────────────────────────────  │ │
│  │                    │  │ 661f...  +5521888...  ⏰ Expir  Prod.  │ │
│  │                    │  │          06/02 14:28           -      │ │
│  │                    │  │──────────────────────────────────────  │ │
│  │                    │  │ 772g...  +5531777...  ✓ Valid  Stag.  │ │
│  │                    │  │          06/02 14:15           12s    │ │
│  │                    │  │──────────────────────────────────────  │ │
│  │                    │  │ 883h...  +5541666...  ✗ Falha  Prod.  │ │
│  │                    │  │          06/02 14:02           -      │ │
│  │                    │  │──────────────────────────────────────  │ │
│  │                    │  │ 994i...  +5511555...  ✓ Valid  Prod.  │ │
│  │                    │  │          06/02 13:55           5s     │ │
│  │                    │  └────────────────────────────────────────┘ │
│  │                    │                                             │
│  │                    │  ← 1  2  3  4  5 ... 12 →                  │
│  │                    │                                             │
│  │                    │  Exportar: [CSV]  [JSON]                    │
│  │                    │                                             │
└──┴────────────────────┴─────────────────────────────────────────────┘
```

---

## Detalhe da Validacao (modal ou pagina)

```
┌───────────────────────────────────────┐
│  Detalhe da Validacao             [X] │
│                                       │
│  ID: 550e8400-e29b-41d4-a716-446...   │
│  Status: ✓ Validado                   │
│                                       │
│  ─────────────────────────────────    │
│                                       │
│  Numero:        +5511999999999        │
│  Solicitado em: 06/02/2026 14:32:00   │
│  Push enviado:  06/02/2026 14:32:01   │
│  Confirmado em: 06/02/2026 14:32:08   │
│  Expirava em:   06/02/2026 14:37:00   │
│                                       │
│  Tempo de resposta: 8 segundos        │
│                                       │
│  ─────────────────────────────────    │
│                                       │
│  API Key: Production (pk_live_a1b2..) │
│  IP de origem: 189.100.xxx.xxx        │
│  Callback URL: https://api.client...  │
│  Callback enviado: Sim (200 OK)       │
│                                       │
│  ─────────────────────────────────    │
│  Timeline                             │
│                                       │
│  14:32:00  Solicitacao recebida       │
│  14:32:01  Push notification enviada  │
│  14:32:08  Confirmacao recebida       │
│  14:32:08  Callback enviado (200 OK)  │
│                                       │
└───────────────────────────────────────┘
```

---

## Filtros

| Filtro | Opcoes | Padrao |
|--------|--------|--------|
| Busca | Texto livre (numero, ID) | Vazio |
| Status | Todos, Validado, Pendente, Expirado, Falha | Todos |
| Periodo | Hoje, 7 dias, 30 dias, 90 dias, Personalizado | 7 dias |
| API Key | Todas, lista de keys ativas | Todas |

---

## Colunas da Tabela

| Coluna | Descricao | Ordenavel |
|--------|-----------|-----------|
| ID | UUID truncado | Nao |
| Numero | Numero E.164 (parcialmente mascarado) | Nao |
| Status | Icone + texto | Sim |
| API Key | Nome da key usada | Sim |
| Data/Hora | Timestamp da solicitacao | Sim (padrao: desc) |
| Tempo Resp. | Segundos ate confirmacao (ou "-") | Sim |

---

## Comportamento

| Acao | Resultado |
|------|-----------|
| Digitar na busca | Filtra em tempo real (debounce 300ms) |
| Alterar filtro | Atualiza tabela sem reload (LiveView) |
| Tap em linha | Abre detalhe da validacao |
| Tap "CSV" / "JSON" | Exporta dados filtrados |
| Tap header de coluna | Ordena por coluna |
| Tap paginacao | Navega entre paginas |
| Nova validacao chega | Insere no topo da lista (LiveView real-time) |

---

## Exportacao

### CSV
```csv
id,phone_number,status,api_key,created_at,validated_at,response_time_seconds
550e8400-...,+5511999999999,validated,Production,2026-02-06T14:32:00Z,2026-02-06T14:32:08Z,8
```

### JSON
```json
{
  "exported_at": "2026-02-06T15:00:00Z",
  "filters": { "status": "all", "period": "7d" },
  "total": 245,
  "validations": [ ... ]
}
```

---

## Paginacao

- 25 itens por pagina
- Total de registros exibido
- Navegacao: primeira, anterior, numeros, proxima, ultima

---

## Notas Tecnicas

- LiveView para atualizacao em tempo real de novos logs
- Busca com ILIKE no banco (ou full-text search para escala)
- Indices necessarios: `(client_id, created_at DESC)`, `(client_id, status)`
- Numeros mascarados na listagem: `+5511999...999`
- Numero completo visivel apenas no detalhe
- Exportacao limitada a 10.000 registros por vez
- Logs retidos por 90 dias (MVP), configuravel por plano

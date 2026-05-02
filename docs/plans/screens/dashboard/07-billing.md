# Tela: Billing / Faturamento

**App:** Dashboard Web (Cliente SaaS)
**Prioridade:** Pos-MVP
**Tecnologia:** Phoenix LiveView + Stripe

---

## Objetivo

Gerenciar plano, metodo de pagamento e visualizar faturas.

---

## Layout

```
┌──────────────────────────────────────────────────────────────────────┐
│  [Logo] Phone Validator          [Docs]  [Suporte]  [User ▼]       │
│──────────────────────────────────────────────────────────────────────│
│  │                                                                  │
│  │ Dashboard          │  Faturamento                                │
│  │ API Keys           │                                             │
│  │ Logs               │  [Plano] [Pagamento] [Faturas] [Uso]       │
│  │ Configuracoes      │                                             │
│  │ Faturamento ←      │                                             │
│  │                    │                                             │
└──┴────────────────────┴─────────────────────────────────────────────┘
```

---

## Tab: Plano

```
│  SEU PLANO ATUAL                                           │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  ★ Plano Startup                                     │  │
│  │  R$ 49/mes                                           │  │
│  │                                                      │  │
│  │  ✓ 5.000 validacoes/mes                              │  │
│  │  ✓ 3 API Keys                                        │  │
│  │  ✓ Webhooks                                          │  │
│  │  ✓ Suporte por email                                 │  │
│  │                                                      │  │
│  │  Uso atual: 2.847 de 5.000 (56.9%)                   │  │
│  │  █████████████░░░░░░░░░░░                             │  │
│  │                                                      │  │
│  │  Renova em: 01/03/2026                               │  │
│  └──────────────────────────────────────────────────────┘  │
│                                                             │
│  PLANOS DISPONIVEIS                                        │
│  ┌────────────┐ ┌────────────┐ ┌────────────┐              │
│  │   Free     │ │  Startup   │ │  Business  │              │
│  │            │ │  (atual)   │ │            │              │
│  │   Gratis   │ │  R$ 49/mes │ │ R$ 199/mes │              │
│  │            │ │            │ │            │              │
│  │  500/mes   │ │  5.000/mes │ │ 50.000/mes │              │
│  │  1 API Key │ │  3 Keys    │ │  10 Keys   │              │
│  │  Sem webhook│ │  Webhooks  │ │  Webhooks  │              │
│  │            │ │  Email     │ │  Prioritario│             │
│  │            │ │            │ │  IP Whitelist│             │
│  │            │ │            │ │  SLA 99.9% │              │
│  │            │ │            │ │            │              │
│  │ [Plano     │ │ [Atual]    │ │ [Upgrade]  │              │
│  │  Atual]    │ │            │ │            │              │
│  └────────────┘ └────────────┘ └────────────┘              │
│                                                             │
│  Precisa de mais? Entre em contato para plano Enterprise.  │
```

---

## Tab: Pagamento

```
│  METODO DE PAGAMENTO                                       │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  💳 Visa terminando em 4242                          │  │
│  │  Expira: 12/2028                                     │  │
│  │                                                      │  │
│  │  [Alterar Cartao]  [Remover]                         │  │
│  └──────────────────────────────────────────────────────┘  │
│                                                             │
│  DADOS DE FATURAMENTO                                      │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  Razao Social                                        │  │
│  │  ┌──────────────────────────────────────────┐        │  │
│  │  │ Empresa XYZ Ltda                         │        │  │
│  │  └──────────────────────────────────────────┘        │  │
│  │                                                      │  │
│  │  CNPJ                                                │  │
│  │  ┌──────────────────────────────────────────┐        │  │
│  │  │ 12.345.678/0001-90                       │        │  │
│  │  └──────────────────────────────────────────┘        │  │
│  │                                                      │  │
│  │  Endereco                                            │  │
│  │  ┌──────────────────────────────────────────┐        │  │
│  │  │ Rua Exemplo, 123 - Sao Paulo/SP          │        │  │
│  │  └──────────────────────────────────────────┘        │  │
│  │                                                      │  │
│  │  ┌────────────┐                                      │  │
│  │  │  Salvar    │                                      │  │
│  │  └────────────┘                                      │  │
│  └──────────────────────────────────────────────────────┘  │
```

---

## Tab: Faturas

```
│  HISTORICO DE FATURAS                                      │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  Fev/2026   R$ 49,00   Pago ✓      [PDF] [NF-e]    │  │
│  │  Jan/2026   R$ 49,00   Pago ✓      [PDF] [NF-e]    │  │
│  │  Dez/2025   R$ 0,00    Free        -               │  │
│  └──────────────────────────────────────────────────────┘  │
```

---

## Tab: Uso

```
│  USO DO MES ATUAL (Fevereiro 2026)                        │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  Validacoes: 2.847 de 5.000                          │  │
│  │  █████████████░░░░░░░░░░░  56.9%                     │  │
│  │                                                      │  │
│  │  ┌──────────────────────────────────────────┐        │  │
│  │  │  Grafico de uso diario                   │        │  │
│  │  │  (barras por dia do mes)                 │        │  │
│  │  └──────────────────────────────────────────┘        │  │
│  │                                                      │  │
│  │  Detalhamento                                        │  │
│  │  Validacoes enviadas:     2.847                      │  │
│  │  Validacoes confirmadas:  2.691 (94.5%)              │  │
│  │  Validacoes expiradas:    128 (4.5%)                 │  │
│  │  Validacoes com falha:    28 (1.0%)                  │  │
│  │                                                      │  │
│  │  Projecao para o mes: ~4.700 (94% do limite)         │  │
│  └──────────────────────────────────────────────────────┘  │
│                                                             │
│  ⚠️ Ao atingir o limite, novas validacoes serao            │
│  recusadas ate o proximo ciclo. Considere fazer upgrade.   │
```

---

## Planos

| Recurso | Free | Startup | Business | Enterprise |
|---------|------|---------|----------|------------|
| Preco | R$ 0 | R$ 49/mes | R$ 199/mes | Sob consulta |
| Validacoes/mes | 500 | 5.000 | 50.000 | Ilimitado |
| API Keys | 1 | 3 | 10 | Ilimitado |
| Webhooks | Nao | Sim | Sim | Sim |
| IP Whitelist | Nao | Nao | Sim | Sim |
| Suporte | Docs | Email | Prioritario | Dedicado |
| SLA | - | - | 99.9% | 99.99% |
| Retencao logs | 7 dias | 30 dias | 90 dias | 1 ano |

---

## Comportamento

| Acao | Resultado |
|------|-----------|
| Tap "Upgrade" | Stripe Checkout para upgrade |
| Tap "Alterar Cartao" | Stripe Customer Portal |
| Tap "PDF" fatura | Download do PDF |
| Tap "NF-e" | Download da nota fiscal |
| Uso atinge 80% | Email de alerta enviado |
| Uso atinge 100% | API retorna 402, email enviado |

---

## Notas Tecnicas

- Integracao com Stripe para pagamentos
- Stripe Customer Portal para gerenciamento de cartao
- Webhook do Stripe para atualizar status do pagamento
- NF-e: integracao com servico de emissao (ex: Nota Carioca, NFe.io)
- Contagem de uso via query agregada com cache
- Alerta de uso via job Oban rodando diariamente

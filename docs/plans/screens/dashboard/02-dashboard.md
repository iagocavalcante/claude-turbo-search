# Tela: Dashboard Principal

**App:** Dashboard Web (Cliente SaaS)
**Prioridade:** MVP
**Tecnologia:** Phoenix LiveView

---

## Objetivo

Visao geral do uso da API, metricas e status do servico para o cliente SaaS.

---

## Layout

```
┌──────────────────────────────────────────────────────────────────────┐
│  [Logo] Phone Validator          [Docs]  [Suporte]  [User ▼]       │
│──────────────────────────────────────────────────────────────────────│
│  │                                                                  │
│  │ Dashboard          │  Bem-vindo, Empresa XYZ                     │
│  │ API Keys           │                                             │
│  │ Logs               │  ┌──────────┐ ┌──────────┐ ┌──────────┐    │
│  │ Configuracoes      │  │ Hoje     │ │ Taxa de  │ │ Tempo    │    │
│  │                    │  │   127    │ │ Sucesso  │ │ Medio    │    │
│  │                    │  │ validacoes│ │  94.5%   │ │  8.2s    │    │
│  │                    │  └──────────┘ └──────────┘ └──────────┘    │
│  │                    │                                             │
│  │                    │  ┌──────────────────────────────────────┐   │
│  │                    │  │  Validacoes - Ultimos 7 dias         │   │
│  │                    │  │                                      │   │
│  │                    │  │  150│    ╭─╮                         │   │
│  │                    │  │  100│╭─╮ │ │╭─╮                     │   │
│  │                    │  │   50││ │ │ ││ │╭─╮╭─╮╭─╮╭─╮        │   │
│  │                    │  │    0│Seg Ter Qua Qui Sex Sab Dom    │   │
│  │                    │  │                                      │   │
│  │                    │  │  ■ Validados  ■ Expirados ■ Falhas  │   │
│  │                    │  └──────────────────────────────────────┘   │
│  │                    │                                             │
│  │                    │  ┌──────────────────────────────────────┐   │
│  │                    │  │  Atividade Recente                   │   │
│  │                    │  │                                      │   │
│  │                    │  │  14:32  +5511999... Validado  8s     │   │
│  │                    │  │  14:28  +5521888... Expirado  -      │   │
│  │                    │  │  14:15  +5531777... Validado  12s    │   │
│  │                    │  │  14:02  +5541666... Validado  5s     │   │
│  │                    │  │  13:55  +5511555... Falhou    -      │   │
│  │                    │  │                                      │   │
│  │                    │  │  Ver todos os logs →                 │   │
│  │                    │  └──────────────────────────────────────┘   │
│  │                    │                                             │
│  │                    │  ┌─────────────────┐ ┌─────────────────┐   │
│  │                    │  │ Quick Start     │ │ Status API      │   │
│  │                    │  │                 │ │                 │   │
│  │                    │  │ Sua API Key:    │ │ ● Operacional   │   │
│  │                    │  │ pk_live_xxxx... │ │ Uptime: 99.9%   │   │
│  │                    │  │ [Copiar]        │ │ Latencia: 45ms  │   │
│  │                    │  └─────────────────┘ └─────────────────┘   │
│  │                    │                                             │
└──┴────────────────────┴─────────────────────────────────────────────┘
```

---

## Componentes

### Cards de Metricas (topo)

| Card | Dados | Descricao |
|------|-------|-----------|
| Validacoes Hoje | Numero inteiro | Total de validacoes iniciadas hoje |
| Taxa de Sucesso | Percentual | Validacoes confirmadas / total |
| Tempo Medio | Segundos | Media de tempo entre envio e confirmacao |

### Grafico de Validacoes
- Tipo: Barras empilhadas
- Periodo: Ultimos 7 dias (seletor: 7d, 30d, 90d)
- Series: Validados (verde), Expirados (amarelo), Falhas (vermelho)

### Atividade Recente
- Lista das ultimas 10 validacoes
- Colunas: Hora, Numero (mascarado), Status, Tempo de resposta
- Link "Ver todos os logs" → Tela de Logs

### Quick Start
- Exibe API key principal (mascarada)
- Botao copiar (copia key completa)

### Status API
- Indicador de status: Operacional (verde), Degradado (amarelo), Fora (vermelho)
- Uptime e latencia media

---

## Sidebar (Navegacao)

| Item | Destino |
|------|---------|
| Dashboard | Tela atual |
| API Keys | Gerenciamento de API Keys |
| Logs | Logs de validacoes |
| Configuracoes | Configuracoes da conta |

---

## Header

| Item | Acao |
|------|------|
| Docs | Abre documentacao da API |
| Suporte | Abre canal de suporte |
| User dropdown | Perfil, Configuracoes, Sair |

---

## Comportamento

| Acao | Resultado |
|------|-----------|
| Selecionar periodo do grafico | Atualiza grafico (LiveView, sem reload) |
| Tap em card de metrica | Navega para Logs com filtro |
| Tap "Copiar" API Key | Copia para clipboard, toast "Copiado!" |
| Tap item atividade | Navega para detalhe do log |
| Nova validacao chega | Atualiza metricas e lista em tempo real (LiveView) |

---

## API / Dados

```elixir
# Dados carregados no mount do LiveView
%{
  today_count: 127,
  success_rate: 94.5,
  avg_response_time: 8.2,
  chart_data: [
    %{date: "2026-02-03", validated: 95, expired: 8, failed: 2},
    %{date: "2026-02-04", validated: 142, expired: 12, failed: 3},
    # ...
  ],
  recent_validations: [
    %{time: "14:32", phone: "+5511999...", status: "validated", response_time: 8},
    # ...
  ],
  primary_api_key: "pk_live_xxxx...xxxx",
  api_status: :operational,
  uptime: 99.9,
  latency: 45
}
```

---

## Notas Tecnicas

- LiveView para atualizacoes em tempo real
- PubSub: subscribe no topico do client_id para novas validacoes
- Grafico: usar Chart.js ou similar via hook
- Metricas agregadas via query otimizada (considerar materialized view para escala)
- Cache de metricas com TTL de 30 segundos

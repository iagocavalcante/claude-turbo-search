# Tela: Home / Dashboard

**App:** Mobile Validador
**Prioridade:** MVP
**Plataformas:** iOS, Android

---

## Objetivo

Tela principal do app. Mostra status do dispositivo, validacoes recentes e informacoes uteis.

---

## Layout

```
┌─────────────────────────────┐
│  Phone Validator     [⚙️]   │
│─────────────────────────────│
│                             │
│  ┌───────────────────────┐  │
│  │  ✓ Ativo              │  │
│  │  +55 11 99999-9999    │  │
│  │  Pronto para validar  │  │
│  └───────────────────────┘  │
│                             │
│  Validacoes Recentes        │
│  ─────────────────────────  │
│                             │
│  ┌───────────────────────┐  │
│  │ ✓ Validado            │  │
│  │ Hoje, 14:32           │  │
│  │ Empresa XYZ           │  │
│  └───────────────────────┘  │
│                             │
│  ┌───────────────────────┐  │
│  │ ✓ Validado            │  │
│  │ Hoje, 11:15           │  │
│  │ App ABC               │  │
│  └───────────────────────┘  │
│                             │
│  ┌───────────────────────┐  │
│  │ ✗ Expirado            │  │
│  │ Ontem, 22:40          │  │
│  │ Servico DEF           │  │
│  └───────────────────────┘  │
│                             │
│  Ver todo historico →       │
│                             │
│─────────────────────────────│
│  [Home]    [Historico]  [⚙️] │
└─────────────────────────────┘
```

---

## Estados

### 1. Ativo (padrao)
- Card verde: "Ativo - Pronto para validar"
- Lista de validacoes recentes (ultimas 5)

### 2. Sem Validacoes
```
┌───────────────────────┐
│  ✓ Ativo              │
│  +55 11 99999-9999    │
│  Pronto para validar  │
└───────────────────────┘

  Nenhuma validacao ainda.
  Quando alguem solicitar
  validacao do seu numero,
  voce vera aqui.
```

### 3. Push Notifications Desabilitadas
```
┌───────────────────────┐
│  ⚠️ Atencao           │
│  Notificacoes estao   │
│  desabilitadas.       │
│                       │
│  [Ativar Notificacoes]│
└───────────────────────┘
```

### 4. Sem Conexao
- Banner no topo: "Sem conexao com a internet"
- Dados locais em cache exibidos

---

## Componentes

### Card de Status
| Campo | Descricao |
|-------|-----------|
| Icone | Checkmark verde (ativo) ou Warning amarelo (problema) |
| Status | "Ativo", "Notificacoes desabilitadas" |
| Numero | Numero registrado (parcialmente mascarado) |
| Mensagem | Texto de status curto |

### Item de Validacao Recente
| Campo | Descricao |
|-------|-----------|
| Status | Icone + texto (Validado, Expirado, Recusado) |
| Data/Hora | Timestamp formatado (relativo se < 24h) |
| Solicitante | Nome do cliente SaaS que solicitou |

### Tab Bar (Navegacao)
| Tab | Destino |
|-----|---------|
| Home | Tela atual |
| Historico | Tela de Historico |
| Configuracoes | Tela de Configuracoes |

---

## Comportamento

| Acao | Resultado |
|------|-----------|
| Pull-to-refresh | Atualiza status e lista de validacoes |
| Tap item de validacao | Navega para detalhe da validacao |
| Tap "Ver todo historico" | Navega para Historico |
| Tap icone engrenagem | Navega para Configuracoes |
| Tap "Ativar Notificacoes" | Abre settings do OS para o app |
| Recebe push notification | Exibe tela de Validacao Recebida |

---

## API Call

```
GET /internal/validations/recent
Headers:
  X-Device-Token: device_token_here

Response 200:
{
  "device_status": "active",
  "phone_number": "+5511999999999",
  "recent_validations": [
    {
      "id": "uuid",
      "status": "validated",
      "client_name": "Empresa XYZ",
      "created_at": "2026-02-06T14:32:00Z",
      "validated_at": "2026-02-06T14:32:15Z"
    }
  ]
}
```

---

## Notas Tecnicas

- Atualizar token FCM silenciosamente ao abrir o app
- Verificar permissao de push notifications
- Cache local das validacoes recentes (offline-first)
- Badge no icone do app para validacoes pendentes
- Atualizar dados via pull-to-refresh e ao voltar ao foreground
